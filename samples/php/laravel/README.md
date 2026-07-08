# PHP Laravel sample

A trimmed-down Laravel app — a single route rendering a static view, no
database, no frontend build step. The build runs `composer install --no-dev
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
docker run --rm -p 8080:8080 laravel-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/up
```
