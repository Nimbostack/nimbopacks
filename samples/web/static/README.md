# Web static sample

A single `index.html` served by nginx — no build step.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 web-static-sample
```

## View

```bash
curl http://localhost:8080
```

Or open http://localhost:8080 in a browser.
