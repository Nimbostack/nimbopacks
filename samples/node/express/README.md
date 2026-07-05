# Node.js Express sample

Minimal Express HTTP API. The build runs `npm install --omit=dev` so only
production dependencies land in the image.

There's no `package-lock.json` checked in, intentionally — the build will
generate one on first install. To pin lockfile-style, commit
`package-lock.json` and switch the build command to `npm ci --omit=dev`.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 node-express-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/healthz
```
