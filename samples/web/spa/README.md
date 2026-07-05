# Web SPA sample (Vite + React)

A small Vite + React 18 app built into static assets and served by nginx.
The `web-spa` template's default nginx config handles push-state routing
(`try_files … /index.html`), so deep links work without extra configuration.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 web-spa-sample
```

## View

Open http://localhost:8080 in a browser. Try refreshing on a deep path
(e.g. `localhost:8080/anything`) — nginx falls back to `index.html`.
