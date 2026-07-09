// Package phppack is the nimbopacks pack for PHP/Composer projects.
package phppack

import (
	"context"
	"maps"
	"strings"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

func init() { registry.Register(&Pack{}) }

type Pack struct{}

func (p *Pack) Name() string { return "php" }

// runtimeExtensions are the Wolfi php-8.4-* extension packages a typical
// Composer/Laravel/CakePHP app needs at runtime.
var runtimeExtensions = []string{
	"php-8.4-openssl",
	"php-8.4-mbstring",
	"php-8.4-xml",
	"php-8.4-dom",
	"php-8.4-curl",
	"php-8.4-ctype",
	"php-8.4-fileinfo",
	"php-8.4-intl",
	"php-8.4-zip",
	"php-8.4-bcmath",
	"php-8.4-gd",
	"php-8.4-pdo",
	"php-8.4-pdo_mysql",
	"php-8.4-mysqlnd",
	"php-8.4-pdo_sqlite",
	"php-8.4-pdo_pgsql",
}

// documentRoot and env mirror what each framework's own template
// (pkg/templates/templates/php/php-*.yaml) sets, so a config generated via
// Detect() behaves the same as one scaffolded from the matching template.
func documentRoot(tmpl string) string {
	switch tmpl {
	case "php-cakephp":
		return "/app/webroot"
	case "php-composer":
		return "/app"
	default: // php-laravel, php-codeigniter
		return "/app/public"
	}
}

func frameworkEnv(tmpl string) map[string]string {
	switch tmpl {
	case "php-laravel":
		return map[string]string{
			"APP_ENV":          "production",
			"LOG_CHANNEL":      "stderr",
			"SESSION_DRIVER":   "file",
			"CACHE_STORE":      "file",
			"QUEUE_CONNECTION": "sync",
		}
	case "php-cakephp":
		return map[string]string{"DEBUG": "false"}
	case "php-codeigniter":
		return map[string]string{"CI_ENVIRONMENT": "production"}
	default:
		return map[string]string{}
	}
}

func (p *Pack) GenerateConfig(_ context.Context, _ string, _ *types.DetectResult, tmpl string) (*types.NimpackConfig, error) {
	cfg := pack.BaseConfig("app", "0.0.1", p.Name(), tmpl)
	cfg.Build.Command = "composer install --no-dev --optimize-autoloader --ignore-platform-reqs"
	cfg.Build.Dependencies = append([]string{"php-8.4", "composer", "build-base"}, runtimeExtensions...)
	cfg.Build.Env = map[string]string{"COMPOSER_CACHE_DIR": "/tmp/composer-cache"}
	cfg.Image.Packages = append([]string{"php-8.4", "ca-certificates-bundle"}, runtimeExtensions...)
	cfg.Image.Entrypoint = "php"
	cfg.Image.Cmd = []string{"-S", "0.0.0.0:8080", "-t", documentRoot(tmpl)}
	cfg.Image.Env = frameworkEnv(tmpl)
	return &cfg, nil
}

func (p *Pack) Plan(_ context.Context, _ string, cfg *types.NimpackConfig) (*types.BuildPlan, error) {
	melange := pack.NewMelangeConfig(cfg.Project.Name, cfg.Project.Version, cfg.Build.Dependencies)
	melange.Package.Dependencies = types.MelangeDependencies{
		Runtime: cfg.Image.Packages,
	}
	melange.Pipeline = []types.MelangePipelineStep{
		{Runs: cfg.Build.Command},
		// Stage the whole source tree (including vendor/) under /app, then
		// install it into the package root (${{targets.destdir}} — the only
		// thing melange actually packages). The find excludes the staging dir
		// itself; a plain `cp -r .` would recurse into it and busybox cp exits
		// non-zero.
		{Runs: "mkdir -p /home/build/output/app\nfind . -mindepth 1 -maxdepth 1 ! -name output -exec cp -r {} /home/build/output/app/ \\;"},
		// Laravel's APP_KEY and CakePHP's SECURITY_SALT are secrets the
		// framework needs to boot (encryption, signed cookies) but that must
		// never be committed to the repo. Generate them fresh on every build
		// and write them where each framework actually reads them from —
		// never touching git — without clobbering anything the project
		// already ships.
		//
		// .env covers Laravel (its dotenv loader is a core, not dev-only,
		// dependency, so it's present even with --no-dev). CakePHP's dotenv
		// loader is dev-only and therefore absent from a --no-dev install, so
		// its SECURITY_SALT wouldn't be read from .env at all; CakePHP's own
		// bootstrap.php already conditionally loads config/app_local.php if
		// present, so writing the salt there needs no dotenv and no
		// CakePHP-specific detection here — it's simply unused, harmless
		// clutter for frameworks that don't look for that file.
		{Runs: `php -r '
$key = base64_encode(random_bytes(32));
$salt = bin2hex(random_bytes(32));

$envPath = "/home/build/output/app/.env";
if (!file_exists($envPath)) {
    file_put_contents($envPath, "APP_KEY=base64:{$key}\nSECURITY_SALT={$salt}\n");
}

$appLocalPath = "/home/build/output/app/config/app_local.php";
if (is_dir(dirname($appLocalPath)) && !file_exists($appLocalPath)) {
    file_put_contents($appLocalPath, "<?php\nreturn [\n    \x27Security\x27 => [\x27salt\x27 => \x27{$salt}\x27],\n];\n");
}
'`},
		// The app runs as a non-root UID (image.run_as) but PHP frameworks write
		// to a few known directories at runtime — Laravel needs the specific
		// nested layout under storage/ (a flat mkdir isn't enough: Blade's
		// compiled-view path, sessions, and cache all live several levels
		// deep), plus bootstrap/cache/; CakePHP needs tmp/ and logs/;
		// CodeIgniter needs writable/. None of these ship the empty dirs in
		// git (git doesn't track empty directories), so create them
		// unconditionally. melange packages files as root and chown to
		// another uid does not survive, so make the directories (not the
		// files already staged inside them — that trips melange's
		// worldwrite linter) world-writable instead.
		{Runs: `for d in \
  storage/framework/cache/data \
  storage/framework/sessions \
  storage/framework/testing \
  storage/framework/views \
  storage/app/public \
  storage/app/private \
  storage/logs \
  bootstrap/cache \
  tmp/cache \
  tmp/sessions \
  logs \
  writable/cache \
  writable/logs \
  writable/session \
  writable/uploads \
  writable/debugbar; do
  mkdir -p "/home/build/output/app/$d"
done
find /home/build/output/app/storage /home/build/output/app/bootstrap/cache /home/build/output/app/tmp /home/build/output/app/logs /home/build/output/app/writable -type d -exec chmod 0777 {} + 2>/dev/null
true`},
		pack.InstallOutputStep(),
	}

	apko := pack.NewApkoConfig(cfg.Project.Name, cfg.Image.Entrypoint, cfg.Image.Packages)
	if len(cfg.Image.Cmd) > 0 {
		apko.Cmd = strings.Join(cfg.Image.Cmd, " ")
	}

	maps.Copy(apko.Environment, cfg.Image.Env)
	plan := &types.BuildPlan{Melange: melange, Apko: apko}
	pack.ApplyConfig(plan, cfg)
	return plan, nil
}

func (p *Pack) Detect(_ context.Context, srcDir string) (*types.DetectResult, error) {
	if !pack.FileExists(srcDir, "composer.json") {
		return nil, nil
	}
	content, _ := pack.ReadFile(srcDir, "composer.json")
	fw := detectFW(content)
	suggested := "php-composer"
	confidence := 0.7
	switch fw {
	case "laravel":
		suggested = "php-laravel"
		confidence = 0.9
	case "cakephp":
		suggested = "php-cakephp"
		confidence = 0.9
	case "codeigniter":
		suggested = "php-codeigniter"
		confidence = 0.9
	}
	summary := "PHP project (Composer)"
	if fw != "" {
		summary = "PHP/" + fw + " (Composer)"
	}
	return &types.DetectResult{
		PackName:          p.Name(),
		Confidence:        confidence,
		Summary:           summary,
		SuggestedTemplate: suggested,
	}, nil
}

func detectFW(composerJSON string) string {
	switch {
	case strings.Contains(composerJSON, "laravel/framework"):
		return "laravel"
	case strings.Contains(composerJSON, "cakephp/cakephp"):
		return "cakephp"
	case strings.Contains(composerJSON, "codeigniter4/framework"):
		return "codeigniter"
	default:
		return ""
	}
}
