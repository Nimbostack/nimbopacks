# Next.js (standalone) sample

A minimal Next.js 14 app using the App Router. `next.config.js` sets
`output: "standalone"`, which makes Next emit a self-contained server tree
under `.next/standalone` that includes only the dependencies the app
actually loads. The image runs that `server.js` directly — full
`node_modules` is not shipped.

Listens on port `3000` (the Next.js standalone default).

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 3000:3000 node-nextjs-sample
```

## View

Open http://localhost:3000 in a browser, or:

```bash
curl http://localhost:3000
```
