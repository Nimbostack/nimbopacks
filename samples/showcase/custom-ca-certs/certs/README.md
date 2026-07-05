# Example CA certificate

`example-ca.pem` in this directory is a **self-signed CA generated for this
sample only**. Its subject is literally:

```
CN = Nimbopacks Example CA (DO NOT USE)
```

It exists so `nimbopacks build` in this sample directory works out of the
box and so the layout (`certs/your-ca.pem` referenced from `nimpack.yaml`)
is concrete. It does not sign anything real and you should not use it for
anything.

In your own project, replace `example-ca.pem` with your corporate/internal
CA, then point `tls.ca_cert_paths` in `nimpack.yaml` at the new filename.
