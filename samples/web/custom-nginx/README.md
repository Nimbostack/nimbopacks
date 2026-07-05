# Web custom-nginx sample

Static files served by nginx using a **project-supplied `nginx.conf`** instead
of the generated default.

Set `build.env.NGINX_CONF_PATH` to your config's path and nimbopacks copies it
to `/etc/nginx/nimbopacks.conf` (the nginx package already owns
`/etc/nginx/nginx.conf`). The image runs `nginx -c /etc/nginx/nimbopacks.conf`.

Because the image runs as a non-root user (`run_as: 65532`), your config must be
non-root-safe — see the comments in [`nginx.conf`](nginx.conf): unprivileged
port, no `user` directive, pid/temp paths under `/var/cache/nginx`, and logs to
stdout/stderr.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 web-custom-nginx-sample
```

## View

```bash
curl http://localhost:8080            # index.html
curl http://localhost:8080/custom     # endpoint only the custom config defines
curl http://localhost:8080/healthz    # ok
curl -sI http://localhost:8080 | grep -i x-served-by   # proves the custom config is active
```
