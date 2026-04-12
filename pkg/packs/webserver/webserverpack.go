// Package webpack is the nimbopacks pack for static sites and web server builds.
//
// It detects projects that produce static output (HTML/CSS/JS) and serves them
// with nginx. Supports:
//
//   - Static HTML sites (index.html in root or build output)
//   - SPA frameworks: React (CRA, Vite), Vue, Angular, Svelte, Astro
//   - Static site generators: Hugo, Jekyll, Eleventy, Gatsby
//   - Any project with a dist/, build/, public/, or out/ directory
//
// nginx.conf and mime.types:
//   - If the project contains nginx.conf, it's used as-is
//   - If not, nimbopacks provides a production-ready default
//   - Same for mime.types
//
// This pack works in two modes:
//   - Pre-built: static files already exist (Hugo output, hand-written HTML)
//   - Build-then-serve: run a build command (npm run build) then serve the output
package webpack

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

func init() { registry.Register(&Pack{}) }

type Pack struct{}

func (p *Pack) Name() string { return "webserver" }

// ---------- Detect ----------

func (p *Pack) Detect(_ context.Context, srcDir string) (*types.DetectResult, error) {
	meta := make(map[string]string)
	var confidence float64
	var summary string
	suggested := "web-static"

	// Check for SPA/SSG framework configs first (highest signal).
	framework, buildCmd, outputDir := detectWebFramework(srcDir)
	if framework != "" {
		meta["framework"] = framework
		meta["build_cmd"] = buildCmd
		meta["output_dir"] = outputDir
		confidence = 0.9
		summary = fmt.Sprintf("Web project (%s)", framework)
		suggested = "web-spa"
	}

	// Check for pre-built static output.
	if confidence == 0 {
		for _, dir := range []string{"dist", "build", "public", "_site", "out", "output", "www"} {
			if pack.FileExists(srcDir, filepath.Join(dir, "index.html")) {
				meta["output_dir"] = dir
				confidence = 0.75
				summary = fmt.Sprintf("Static site (pre-built in %s/)", dir)
				break
			}
		}
	}

	// Check for index.html in root (simplest case).
	if confidence == 0 && pack.FileExists(srcDir, "index.html") {
		meta["output_dir"] = "."
		confidence = 0.7
		summary = "Static HTML site"
	}

	if confidence == 0 {
		return nil, nil
	}

	// Check for custom nginx.conf.
	if pack.FileExists(srcDir, "nginx.conf") {
		meta["has_nginx_conf"] = "true"
	}
	if pack.FileExists(srcDir, "mime.types") {
		meta["has_mime_types"] = "true"
	}

	// Check common nginx config locations.
	for _, loc := range []string{"config/nginx.conf", ".nginx/nginx.conf", "docker/nginx.conf"} {
		if pack.FileExists(srcDir, loc) {
			meta["has_nginx_conf"] = "true"
			meta["nginx_conf_path"] = loc
			break
		}
	}

	return &types.DetectResult{
		PackName:          p.Name(),
		Confidence:        confidence,
		Summary:           summary,
		SuggestedTemplate: suggested,
		Metadata:          meta,
	}, nil
}

// ---------- GenerateConfig ----------

func (p *Pack) GenerateConfig(_ context.Context, srcDir string, detected *types.DetectResult, templateName string) (*types.NimpackConfig, error) {
	meta := detected.Metadata

	absDir, _ := filepath.Abs(srcDir)
	name := filepath.Base(absDir)

	cfg := pack.BaseConfig(name, "0.0.1", p.Name(), templateName)

	outputDir := meta["output_dir"]
	if outputDir == "" {
		outputDir = "dist"
	}

	// Build deps — only if there's a build step.
	if buildCmd, ok := meta["build_cmd"]; ok && buildCmd != "" {
		cfg.Build.Command = buildCmd
		cfg.Build.Dependencies = []string{"nodejs-20", "npm", "build-base"}
	}

	cfg.Image.Packages = []string{"nginx", "ca-certificates-bundle"}
	cfg.Image.Entrypoint = "/usr/sbin/nginx"
	cfg.Image.Cmd = []string{"-c", "/etc/nginx/nginx.conf", "-g", "daemon off;"}
	cfg.Image.Ports = []string{"8080"}
	cfg.Image.RunAs = 65532

	cfg.Build.Env = map[string]string{
		"OUTPUT_DIR": outputDir,
	}

	if path, ok := meta["nginx_conf_path"]; ok && path != "" {
		cfg.Build.Env["NGINX_CONF_PATH"] = path
	}

	return &cfg, nil
}

// ---------- Plan ----------

func (p *Pack) Plan(_ context.Context, _ string, cfg *types.NimpackConfig) (*types.BuildPlan, error) {
	name := cfg.Project.Name
	version := cfg.Project.Version

	buildDeps := cfg.Build.Dependencies
	if len(buildDeps) == 0 {
		buildDeps = []string{"build-base"}
	}

	melange := pack.NewMelangeConfig(name, version, buildDeps)
	melange.Package.Dependencies = types.MelangeDependencies{
		Runtime: []string{"nginx", "ca-certificates-bundle"},
	}

	outputDir := cfg.Build.Env["OUTPUT_DIR"]
	if outputDir == "" {
		outputDir = "dist"
	}

	var steps []types.MelangePipelineStep

	// Step 1: Build (if there's a build command).
	if cfg.Build.Command != "" {
		steps = append(steps, types.MelangePipelineStep{
			Runs: cfg.Build.Command,
		})
	}

	// Step 2: Copy static output to serve directory.
	steps = append(steps, types.MelangePipelineStep{
		Runs: fmt.Sprintf(`# Copy static files to nginx serve directory
mkdir -p /home/build/output/var/www/html
cp -r %s/* /home/build/output/var/www/html/
`, outputDir),
	})

	// Step 3: nginx.conf — use custom if exists, otherwise generate default.
	nginxConfPath := ""
	if path, ok := cfg.Build.Env["NGINX_CONF_PATH"]; ok && path != "" {
		nginxConfPath = path
	}

	if nginxConfPath != "" {
		// User provided a custom nginx.conf.
		steps = append(steps, types.MelangePipelineStep{
			Runs: fmt.Sprintf(`# Install custom nginx.conf
mkdir -p /home/build/output/etc/nginx
cp %s /home/build/output/etc/nginx/nginx.conf
`, nginxConfPath),
		})
	} else {
		// Generate default nginx.conf.
		steps = append(steps, types.MelangePipelineStep{
			Runs: fmt.Sprintf(`# Generate default nginx.conf
mkdir -p /home/build/output/etc/nginx
cat > /home/build/output/etc/nginx/nginx.conf << 'NGINXEOF'
%s
NGINXEOF
`, defaultNginxConf),
		})
	}

	// Step 4: mime.types — use custom if exists, otherwise generate default.
	hasMimeTypes := false
	if v, ok := cfg.Build.Env["HAS_MIME_TYPES"]; ok && v == "true" {
		hasMimeTypes = true
	}

	if !hasMimeTypes {
		steps = append(steps, types.MelangePipelineStep{
			Runs: fmt.Sprintf(`# Generate default mime.types
cat > /home/build/output/etc/nginx/mime.types << 'MIMEEOF'
%s
MIMEEOF
`, defaultMimeTypes),
		})
	} else {
		steps = append(steps, types.MelangePipelineStep{
			Runs: `# Install custom mime.types
cp mime.types /home/build/output/etc/nginx/mime.types
`,
		})
	}

	// Step 5: Create required nginx directories.
	steps = append(steps, types.MelangePipelineStep{
		Runs: `# Create nginx runtime directories
mkdir -p /home/build/output/var/log/nginx
mkdir -p /home/build/output/var/cache/nginx
mkdir -p /home/build/output/var/run
touch /home/build/output/var/run/nginx.pid
chown -R 65532:65532 /home/build/output/var/log/nginx
chown -R 65532:65532 /home/build/output/var/cache/nginx
chown -R 65532:65532 /home/build/output/var/run
`,
	})

	melange.Pipeline = steps

	apko := pack.NewApkoConfig(name, cfg.Image.Entrypoint, cfg.Image.Packages)
	if len(cfg.Image.Cmd) > 0 {
		apko.Cmd = strings.Join(cfg.Image.Cmd, " ")
	}
	maps.Copy(apko.Environment, cfg.Image.Env)

	plan := &types.BuildPlan{Melange: melange, Apko: apko}
	pack.ApplyConfig(plan, cfg)
	return plan, nil
}

// ---------- Framework detection ----------

func detectWebFramework(srcDir string) (framework, buildCmd, outputDir string) {
	// Vite (React, Vue, Svelte, etc.)
	if pack.FileExists(srcDir, "vite.config.ts") || pack.FileExists(srcDir, "vite.config.js") {
		fw := "vite"
		// Check for specific framework under Vite.
		if pack.FileExists(srcDir, "svelte.config.js") {
			fw = "sveltekit"
		} else if content, err := pack.ReadFile(srcDir, "package.json"); err == nil {
			if strings.Contains(content, "\"vue\"") {
				fw = "vue"
			} else if strings.Contains(content, "\"react\"") {
				fw = "react"
			}
		}
		return fw, "npm ci && npm run build", "dist"
	}

	// Create React App.
	if pack.FileExists(srcDir, "package.json") {
		if content, err := pack.ReadFile(srcDir, "package.json"); err == nil {
			if strings.Contains(content, "react-scripts") {
				return "create-react-app", "npm ci && npm run build", "build"
			}
		}
	}

	// Angular.
	if pack.FileExists(srcDir, "angular.json") {
		return "angular", "npm ci && npx ng build --configuration production", "dist"
	}

	// Astro.
	if pack.FileExists(srcDir, "astro.config.mjs") || pack.FileExists(srcDir, "astro.config.ts") {
		return "astro", "npm ci && npm run build", "dist"
	}

	// Hugo.
	if pack.FileExists(srcDir, "hugo.toml") || pack.FileExists(srcDir, "hugo.yaml") || pack.FileExists(srcDir, "config.toml") {
		if pack.FileExists(srcDir, "content") || pack.FileExists(srcDir, "layouts") {
			return "hugo", "hugo --minify", "public"
		}
	}

	// Jekyll.
	if pack.FileExists(srcDir, "_config.yml") && pack.FileExists(srcDir, "_layouts") {
		return "jekyll", "bundle exec jekyll build", "_site"
	}

	// Eleventy.
	if pack.FileExists(srcDir, ".eleventy.js") || pack.FileExists(srcDir, "eleventy.config.js") {
		return "eleventy", "npm ci && npx @11ty/eleventy", "_site"
	}

	// Gatsby.
	if pack.FileExists(srcDir, "gatsby-config.js") || pack.FileExists(srcDir, "gatsby-config.ts") {
		return "gatsby", "npm ci && npx gatsby build", "public"
	}

	return "", "", ""
}

// ---------- Default configs ----------

const defaultNginxConf = `worker_processes auto;
pid /var/run/nginx.pid;
error_log /var/log/nginx/error.log warn;

events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent"';

    access_log /var/log/nginx/access.log main;

    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types
        text/plain
        text/css
        text/xml
        text/javascript
        application/json
        application/javascript
        application/xml
        application/rss+xml
        image/svg+xml;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    server {
        listen 8080;
        server_name _;
        root /var/www/html;
        index index.html;

        # SPA fallback — serve index.html for all routes
        location / {
            try_files $uri $uri/ /index.html;
        }

        # Cache static assets aggressively
        location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
            expires 1y;
            add_header Cache-Control "public, immutable";
        }

        # Don't log favicon/robots requests
        location = /favicon.ico { access_log off; log_not_found off; }
        location = /robots.txt  { access_log off; log_not_found off; }

        # Deny hidden files
        location ~ /\. {
            deny all;
            access_log off;
            log_not_found off;
        }

        # Health check endpoint
        location = /healthz {
            access_log off;
            return 200 "ok\n";
            add_header Content-Type text/plain;
        }
    }
}`

const defaultMimeTypes = `types {
    text/html                                        html htm shtml;
    text/css                                         css;
    text/xml                                         xml;
    text/plain                                       txt;
    text/javascript                                  js mjs;

    application/json                                 json;
    application/javascript                           js;
    application/xml                                  xml;
    application/rss+xml                              rss;
    application/atom+xml                             atom;
    application/pdf                                  pdf;
    application/zip                                  zip;
    application/gzip                                 gz;
    application/wasm                                 wasm;
    application/manifest+json                        webmanifest;

    image/gif                                        gif;
    image/jpeg                                       jpeg jpg;
    image/png                                        png;
    image/svg+xml                                    svg svgz;
    image/webp                                       webp;
    image/avif                                       avif;
    image/x-icon                                     ico;

    font/woff                                        woff;
    font/woff2                                       woff2;
    font/ttf                                         ttf;
    font/otf                                         otf;
    application/vnd.ms-fontobject                    eot;

    audio/mpeg                                       mp3;
    audio/ogg                                        ogg;
    audio/wav                                        wav;
    audio/webm                                       weba;

    video/mp4                                        mp4;
    video/webm                                       webm;
    video/ogg                                        ogv;
}`
