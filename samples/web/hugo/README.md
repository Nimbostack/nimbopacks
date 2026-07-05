# Web Hugo sample

A minimal Hugo site — one Markdown page (`content/_index.md`) and one HTML
layout (`layouts/index.html`), no theme. The build runs `hugo --minify`
which writes the rendered site to `public/`, and nginx serves it.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 web-hugo-sample
```

## View

```bash
curl http://localhost:8080
```
