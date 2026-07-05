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

// Name is "web" to match the build.pack value in the web-* templates and
// samples (and the resulting detection output).
func (p *Pack) Name() string { return "web" }

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

	// Check for a custom nginx.conf. Record its path (not just a boolean) so
	// GenerateConfig can wire NGINX_CONF_PATH — including the root location.
	for _, loc := range []string{"nginx.conf", "config/nginx.conf", ".nginx/nginx.conf", "docker/nginx.conf"} {
		if pack.FileExists(srcDir, loc) {
			meta["has_nginx_conf"] = "true"
			meta["nginx_conf_path"] = loc
			break
		}
	}
	if pack.FileExists(srcDir, "mime.types") {
		meta["has_mime_types"] = "true"
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
	// Our config lives at nimbopacks.conf (the nginx package already owns
	// /etc/nginx/nginx.conf). -e sets the early error log to stderr; the
	// `daemon off;` directive is in the config itself (apko splits cmd on
	// spaces, which would mangle a `-g "daemon off;"` argument).
	cfg.Image.Cmd = []string{"-e", "/dev/stderr", "-c", "/etc/nginx/nimbopacks.conf"}
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
	// find (rather than `cp -r <dir>/*`) skips the staging/build dirs — with
	// OUTPUT_DIR="." a plain glob would recurse into /home/build/output and
	// melange-out, and busybox cp exits non-zero on that.
	steps = append(steps, types.MelangePipelineStep{
		Runs: fmt.Sprintf(`# Copy static files to nginx serve directory
mkdir -p /home/build/output/var/www/html
find %s -mindepth 1 -maxdepth 1 ! -name output ! -name melange-out -exec cp -r {} /home/build/output/var/www/html/ \;
`, outputDir),
	})

	// Step 3: nginx.conf — use custom if exists, otherwise generate default.
	nginxConfPath := ""
	if path, ok := cfg.Build.Env["NGINX_CONF_PATH"]; ok && path != "" {
		nginxConfPath = path
	}

	// We write to nimbopacks.conf (not nginx.conf): the nginx package already
	// ships /etc/nginx/nginx.conf and /etc/nginx/mime.types, and apko forbids
	// two packages owning the same file. The image cmd points nginx at this
	// file via `-c /etc/nginx/nimbopacks.conf`, and the config reuses nginx's
	// own /etc/nginx/mime.types.
	if nginxConfPath != "" {
		// User provided a custom nginx.conf.
		steps = append(steps, types.MelangePipelineStep{
			Runs: fmt.Sprintf(`# Install custom nginx config
mkdir -p /home/build/output/etc/nginx
cp %s /home/build/output/etc/nginx/nimbopacks.conf
`, nginxConfPath),
		})
	} else {
		// Generate default nginx config.
		steps = append(steps, types.MelangePipelineStep{
			Runs: fmt.Sprintf(`# Generate default nginx config
mkdir -p /home/build/output/etc/nginx
cat > /home/build/output/etc/nginx/nimbopacks.conf << 'NGINXEOF'
%s
NGINXEOF
`, defaultNginxConf),
		})
	}

	// (mime.types intentionally not packaged — the config reuses the one shipped
	// by the nginx package at /etc/nginx/mime.types.)

	// Step 4: Create required nginx directories.
	steps = append(steps, types.MelangePipelineStep{
		Runs: `# Create nginx's writable runtime dir. melange packages files as root and
# chown to another uid does not survive, but permission bits do — so make the
# cache dir world-writable. nginx (running as the non-root runtime user) writes
# its pid and temp subdirs (client_body, proxy, ...) here. Logs go to
# stdout/stderr, so no log dir is needed. /var/run is intentionally avoided —
# packaging it trips melange's tempdir linter.
mkdir -p /home/build/output/var/cache/nginx
chmod 0777 /home/build/output/var/cache/nginx
`,
	})

	// Install everything staged under /home/build/output (var/www/html, the
	// nginx config, and the runtime dirs) into the package root.
	steps = append(steps, pack.InstallOutputStep())

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
# Run in the foreground. This lives here rather than as a -g "daemon off;"
# command-line arg because apko splits cmd on spaces and would mangle the
# quoted argument.
daemon off;
# Pid lives under the (writable, non-temp) cache dir; /var/run trips melange's
# tempdir linter and isn't writable by the non-root runtime user.
pid /var/cache/nginx/nginx.pid;
# Log to the container's stdout/stderr (12-factor) so no writable log dir or
# root-owned default path is needed.
error_log /dev/stderr warn;

events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent"';

    access_log /dev/stdout main;

    # nginx's compiled-in temp paths live under root-owned /var/lib/nginx;
    # point them at the writable cache dir so the non-root user can create them.
    client_body_temp_path /var/cache/nginx/client_body;
    proxy_temp_path /var/cache/nginx/proxy;
    fastcgi_temp_path /var/cache/nginx/fastcgi;
    uwsgi_temp_path /var/cache/nginx/uwsgi;
    scgi_temp_path /var/cache/nginx/scgi;

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
