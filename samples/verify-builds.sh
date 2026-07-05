#!/usr/bin/env bash
#
# verify-builds.sh — build every sample, run the resulting image, and probe it.
#
# This proves the samples actually *run*, not just that `nimbopacks build`
# succeeds. For each sample it: builds the image, loads the OCI tarball into
# Docker, starts a container, and probes the running service (HTTP, gRPC h2c,
# or a log pattern for non-HTTP workers).
#
# Usage:
#   samples/verify-builds.sh                 # verify all samples
#   samples/verify-builds.sh go/rest node/express   # verify only these
#
# Env:
#   NIMBOPACKS_BIN   path to the nimbopacks binary (default: ./bin/nimbopacks,
#                    then `nimbopacks` on PATH)
#   HOST_PORT        host port to bind while probing (default: 8099)
#
# Exit code: 0 if all verified samples pass, 1 otherwise.

set -uo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
samples_dir="$repo_root/samples"
host_port="${HOST_PORT:-8099}"

# Resolve the nimbopacks binary.
if [[ -n "${NIMBOPACKS_BIN:-}" ]]; then
  nb="$NIMBOPACKS_BIN"
elif [[ -x "$repo_root/bin/nimbopacks" ]]; then
  nb="$repo_root/bin/nimbopacks"
elif command -v nimbopacks >/dev/null 2>&1; then
  nb="nimbopacks"
else
  echo "error: nimbopacks binary not found (build it with 'task build' or set NIMBOPACKS_BIN)" >&2
  exit 2
fi

# Sample manifest: "dir|container-port|probe-type|arg"
#   probe-type http : GET http://host:port<arg>, expect HTTP 200
#   probe-type h2c  : gRPC over HTTP/2 cleartext on <port>, expect an HTTP/2 reply
#   probe-type log  : no port; container logs must match the <arg> regex
SAMPLES=(
  "go/rest|8080|http|/"
  "go/grpc|50051|h2c|"
  "dotnet/minimal-api|8080|http|/"
  "dotnet/webapi|8080|http|/healthz"
  "dotnet/monorepo|8080|http|/"
  "dotnet/grpc|8080|h2c|"
  "dotnet/blazor|8080|http|/"
  "dotnet/worker|-|log|Heartbeat"
  "java/maven|8080|http|/healthz"
  "java/gradle|8080|http|/healthz"
  "java/micronaut|8080|http|/"
  "java/quarkus|8080|http|/healthz"
  "java/webflux|8080|http|/healthz"
  "node/express|8080|http|/healthz"
  "node/fastify|8080|http|/healthz"
  "node/hono|8080|http|/healthz"
  "node/nestjs|8080|http|/healthz"
  "node/nextjs|3000|http|/"
  "python/fastapi|8080|http|/"
  "python/django|8080|http|/healthz"
  "web/static|8080|http|/"
  "web/spa|8080|http|/"
  "web/hugo|8080|http|/"
  "web/custom-nginx|8080|http|/custom"
  "showcase/layering|8080|http|/"
  "showcase/custom-ca-certs|8080|http|/"
  "showcase/cve-patching|8080|http|/"
)

container="nimbopacks-verify"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT

# Warm-up seconds before probing — JVM and Blazor need longer to boot.
warmup_for() {
  case "$1" in
    java/*|dotnet/blazor) echo 15 ;;
    dotnet/*)             echo 8  ;;
    node/nextjs)          echo 6  ;;
    *)                    echo 4  ;;
  esac
}

verify_one() {
  local dir="$1" cport="$2" ptype="$3" arg="$4"
  local path="$samples_dir/$dir"
  [[ -f "$path/nimpack.yaml" ]] || { echo "FAIL  $dir (no nimpack.yaml)"; return 1; }

  # Build.
  local log; log="$(mktemp)"
  if ! ( cd "$path" && "$nb" build ) >"$log" 2>&1; then
    echo "FAIL  $dir (build) — see ${log}"
    tail -3 "$log" | sed 's/^/      /'
    return 1
  fi

  # Load the freshly-built OCI tarball into Docker.
  local tar; tar="$(ls -t "$HOME"/.nimbopacks/cache/builds/*/output/image.tar 2>/dev/null | head -1)"
  [[ -n "$tar" ]] || { echo "FAIL  $dir (no image.tar produced)"; return 1; }
  local image; image="$(docker load -i "$tar" 2>/dev/null | grep -oE 'Loaded image: .*' | sed 's/Loaded image: //' | tail -1)"
  [[ -n "$image" ]] || { echo "FAIL  $dir (docker load produced no tag)"; return 1; }

  cleanup
  sleep "$(warmup_for "$dir")"

  # Worker (no HTTP): start, then assert the log pattern appears.
  if [[ "$ptype" == "log" ]]; then
    docker run -d --rm --name "$container" "$image" >/dev/null 2>&1
    sleep "$(warmup_for "$dir")"
    if docker logs "$container" 2>&1 | grep -qE "$arg"; then
      echo "PASS  $dir (worker log matched /$arg/)"; cleanup; return 0
    fi
    echo "FAIL  $dir (worker log did not match /$arg/)"
    docker logs "$container" 2>&1 | tail -3 | sed 's/^/      /'; cleanup; return 1
  fi

  docker run -d --rm -p "$host_port:$cport" --name "$container" "$image" >/dev/null 2>&1
  sleep "$(warmup_for "$dir")"

  # gRPC over HTTP/2 cleartext: a live gRPC server replies over HTTP/2 (it
  # rejects the non-gRPC content-type, but the HTTP/2 handshake itself is proof).
  if [[ "$ptype" == "h2c" ]]; then
    read -r ver code < <(curl -s --http2-prior-knowledge --max-time 8 \
      "http://localhost:$host_port/" -o /dev/null -w '%{http_version} %{http_code}' 2>/dev/null)
    if [[ "$ver" == "2" && "$code" != "000" ]]; then
      echo "PASS  $dir (gRPC h2c — HTTP/2, status $code)"; cleanup; return 0
    fi
    echo "FAIL  $dir (gRPC h2c — http_version=$ver code=$code)"
    docker logs "$container" 2>&1 | tail -3 | sed 's/^/      /'; cleanup; return 1
  fi

  # HTTP: expect a 200.
  local code body
  code="$(curl -s --max-time 8 "http://localhost:$host_port$arg" -o /dev/null -w '%{http_code}' 2>/dev/null)"
  body="$(curl -s --max-time 8 "http://localhost:$host_port$arg" 2>/dev/null | tr -d '\n' | head -c 50)"
  if [[ "$code" == "200" ]]; then
    echo "PASS  $dir (HTTP 200 $arg → ${body})"; cleanup; return 0
  fi
  echo "FAIL  $dir (HTTP $code $arg)"
  docker logs "$container" 2>&1 | tail -3 | sed 's/^/      /'; cleanup; return 1
}

# Build the work list: all samples, or just the dirs passed as args.
declare -a work=()
if [[ $# -gt 0 ]]; then
  for want in "$@"; do
    match=""
    for entry in "${SAMPLES[@]}"; do
      [[ "${entry%%|*}" == "$want" ]] && match="$entry" && break
    done
    [[ -n "$match" ]] || { echo "error: unknown sample '$want'" >&2; exit 2; }
    work+=("$match")
  done
else
  work=("${SAMPLES[@]}")
fi

echo "Verifying ${#work[@]} sample(s) with $nb"
echo

pass=0 fail=0
for entry in "${work[@]}"; do
  IFS='|' read -r dir cport ptype arg <<<"$entry"
  if verify_one "$dir" "$cport" "$ptype" "$arg"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
  fi
done

echo
echo "=== $pass passed, $fail failed ==="
[[ "$fail" -eq 0 ]]
