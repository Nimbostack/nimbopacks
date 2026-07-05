# Showcase: multi-layer images with apko layering

This sample exists to walk through **`image.layering`** end-to-end — apko's
built-in multi-layer image assembly. The Go code is intentionally trivial;
the interesting part is `nimpack.yaml` and the commands below.

## Why layering?

By default apko produces a **single-layer** OCI image. That keeps things
simple, but it means every package update invalidates the whole layer for
every consumer pulling the image.

With layering enabled, apko splits the image into multiple layers grouped
by package **origin** — packages built from the same upstream source share
a layer. When `openssl` ships a patch, only the openssl layer is rebuilt;
consumers pulling the new image re-download just that layer.

Chainguard found `budget: 10` eliminated ~70% of unique layer data across
their catalog.

## The config

```yaml
image:
  packages:
    - ca-certificates-bundle
    - openssl
    - glibc-locale-en
    - tzdata
    - curl
    - jq
    - bash
    - coreutils
  layering:
    strategy: origin
    budget: 8
```

Two fields, that's it:

| Field | Meaning |
|---|---|
| `strategy` | The layering algorithm. Currently only `origin` is supported. |
| `budget` | Number of additional layers apko will create. Total layer count in the resulting image is `budget + 1` (the +1 is the "top" layer for OS metadata — installed db, apk world, etc.). |

apko takes the top N origin-groups by installed size, gives each its own
layer, and overflows anything else into one shared layer.

## 1. Build the image

```bash
nimbopacks build
```

## 2. Inspect the layers

```bash
docker inspect layering-sample | jq '.[0].RootFS.Layers'
```

You should see roughly `budget + 1` entries — one per origin-group plus
the top metadata layer. Compare against the same build with `layering:`
removed (single layer).

You can also dive deeper with [dive](https://github.com/wagoodman/dive):

```bash
dive layering-sample
```

## 3. Tune the budget

Try different budgets and watch how the layers split:

```yaml
layering:
  strategy: origin
  budget: 2   # only the two biggest origin-groups get their own layer
```

```yaml
layering:
  strategy: origin
  budget: 20  # finer-grained — more layers, smaller diffs on update
```

Rebuild after each change and re-inspect:

```bash
nimbopacks build
docker inspect layering-sample | jq '.[0].RootFS.Layers | length'
```

## Picking a budget

| Image type | Suggested budget |
|---|---|
| Application image (leaf, won't be extended) | `8-16` |
| Base image (consumers will add layers on top) | `2-4` |
| Tiny image (≤4 packages, layering has little to split) | omit `layering:` entirely |

Container runtimes cap at 127 layers total, so always leave headroom.

## Reference

- apko layering docs: https://github.com/chainguard-dev/apko/blob/main/docs/layering.md
- Available since apko v0.27.0
