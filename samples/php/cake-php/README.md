# PHP CakePHP sample

A trimmed-down CakePHP app — a single page controller rendering a static
view, no database. The build runs `composer install --no-dev
--optimize-autoloader --ignore-platform-reqs` and serves the app with PHP's
built-in web server.

CakePHP's `SECURITY_SALT` is generated fresh on every build (see the php
pack's `Plan()`) and written to `config/app_local.php`, never committed.

There's no `composer.lock` checked in, intentionally — the build resolves
fresh on first install.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 cake-php-sample
```

## View

```bash
curl http://localhost:8080
```
