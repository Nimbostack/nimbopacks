# Showcase: custom CA certificates

This sample walks through the **`tls`** section of `nimpack.yaml` —
injecting custom CA certificates into every layer of the build and into
the final image's trust store. The Go code is intentionally trivial.

## When you need this

Reach for `tls.ca_cert_paths` (or `tls.ca_dir_path`) when any of:

- You're behind a **corporate TLS-intercepting proxy** (Zscaler, Netskope,
  Palo Alto, BlueCoat, …) and APK/package fetches fail with
  `x509: certificate signed by unknown authority`.
- You pull packages from a **private Wolfi mirror** that uses a self-signed
  or internal-CA-signed certificate.
- Your application makes **outbound HTTPS calls to internal services** at
  runtime and needs to trust the same internal CAs.
- You push images to a **private registry** with a custom CA.

If `curl https://packages.wolfi.dev/os/...` works on your laptop but
`nimbopacks build` fails on TLS, this is almost certainly what's missing.

## The config

```yaml
tls:
  ca_cert_paths:
    - certs/example-ca.pem
```

That's the minimum. With that one field set, the listed CA(s) are:

1. Added to melange's environment when fetching APK packages.
2. Added to apko's environment when assembling the image.
3. **Baked into the final image's `/etc/ssl/certs/`** so the running
   application trusts them (unless `inject_into_image: false`).
4. Used by `nimbopacks build` itself when pushing to a registry.

## All TLS options

```yaml
tls:
  # 1. Explicit PEM file list. Paths are absolute or relative to the
  #    project root. Multiple files are concatenated in the order listed.
  ca_cert_paths:
    - certs/corporate-ca.pem
    - /etc/ssl/internal/proxy-ca.pem

  # 2. Or: a directory of PEM files. All .pem and .crt files in the
  #    directory are loaded. Use this when you mirror the host's
  #    /usr/local/share/ca-certificates layout.
  ca_dir_path: certs/

  # 3. Whether to inject the CAs into the final image's trust store.
  #    Default: true. Set false if you only need the CAs during the build
  #    (e.g. to fetch deps) and don't want them in the published image.
  inject_into_image: true

  # 4. Disable TLS verification entirely.
  #    DEBUG ONLY. Never in production. Useful as a one-off diagnostic
  #    when you suspect a CA issue and want to rule TLS out as the cause.
  insecure: false
```

You can combine `ca_cert_paths` and `ca_dir_path` — both are loaded.

## 1. Build the image

```bash
nimbopacks build
```

The included `certs/example-ca.pem` is a throwaway self-signed CA
generated for this sample. See [`certs/README.md`](certs/README.md) — in
your own project, replace it with your actual corporate CA and update the
path in `nimpack.yaml`.

## 2. Verify the CA was baked into the image

```bash
docker run --rm --entrypoint sh custom-ca-certs-sample -c \
  'grep -l "Nimbopacks Example CA" /etc/ssl/certs/* 2>/dev/null | head -3'
```

You should see the CA bundle (and/or hash-named symlinks) listing the
example CA. With `inject_into_image: false`, the same command returns
nothing — the CA was used during build but not shipped in the image.

## 3. Verify the running app trusts it

Mount a service that uses your CA and watch the call succeed:

```bash
# Inside the image, an outbound HTTPS call to a service signed by your
# internal CA should now succeed without -k / InsecureSkipVerify.
```

## Common pitfalls

- **Symptom**: build still fails with `x509: certificate signed by unknown
  authority` after setting `ca_cert_paths`.
  **Cause**: the PEM file isn't valid — wrong format, wrong encoding,
  empty, or it's a leaf cert rather than the CA cert.
  **Check**: `openssl x509 -in certs/your-ca.pem -noout -subject -issuer`
  should print the CA's own subject as both subject *and* issuer.

- **Symptom**: `nimbopacks build` works but the running app gets TLS
  errors when calling internal services.
  **Cause**: `inject_into_image: false` is set (or you set it once and
  forgot). Flip it to `true` (or remove it — `true` is the default).

- **Symptom**: works on Linux, fails inside the build runner on macOS/WSL.
  **Cause**: the docker runner doesn't see your host trust store. Custom
  CAs **must** be declared in `nimpack.yaml`; we don't read host trust
  stores implicitly (that would be both surprising and non-reproducible).

## Reference

- Schema: `internal/types/types.go` (`TLSConfig`)
- Implementation: `internal/utils/tlsutil.go`
