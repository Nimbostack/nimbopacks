# Node.js NestJS sample

Minimal NestJS HTTP API (on the Express platform). The build runs
`npm install && npm run build`, which compiles the TypeScript sources to
`dist/` with `tsc` and installs all dependencies. The compiled entrypoint
`dist/main.js` is run by node.

There's no `package-lock.json` checked in, intentionally — the build will
generate one on first install. To pin lockfile-style, commit
`package-lock.json` and switch the build command to `npm ci && npm run build`.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 node-nestjs-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/healthz
```
