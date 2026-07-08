# PHP CodeIgniter sample

A trimmed-down CodeIgniter 4 app — a single controller action returning a
static page, no database. The build runs `composer install --no-dev
--optimize-autoloader --ignore-platform-reqs` and serves the app with PHP's
built-in web server.

There's no `composer.lock` checked in, intentionally — the build resolves
fresh on first install.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 codeigniter-sample
```

## View

```bash
curl http://localhost:8080
```
