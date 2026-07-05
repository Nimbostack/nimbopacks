# Go REST sample

A minimal HTTP service using the Go standard library — no third-party
dependencies, so the build is fast and the image is tiny.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 go-rest-sample
```

## View

```bash
curl http://localhost:8080
curl -i http://localhost:8080/healthz
```
