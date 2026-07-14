#!/usr/bin/env bash
#
# affected-samples.sh — map a list of changed files to the subset of samples
# (discovered from samples/<lang>/<name>/nimpack.yaml on disk) that actually
# need to be rebuilt.
#
# Reads changed file paths, one per line, on stdin. Prints a JSON array of
# "lang/name" sample identifiers to stdout, suitable for a GitHub Actions
# matrix (`fromJSON(...)`).
#
# Rules:
#   - samples/<lang>/<name>/**            -> that one sample
#   - pkg/packs/<pack>/**                 -> every sample built by that pack
#   - pkg/templates/templates/<pack>/**   -> every sample built by that pack
#   - golang pack changes also affect samples/showcase/** (they all use
#     `pack: golang` to exercise apko-level features like layering/CA certs)
#   - anything else (pkg/backend/**, internal/**, cmd/**, go.mod, go.sum,
#     pkg/templates/*.go, this workflow file, verify-builds.sh, ...) is
#     shared/core code: it can affect every sample, so the full list runs.
#   - no input at all also means "run everything" (fail safe).
#
# Usage:
#   git diff --name-only "$BASE" "$HEAD" | samples/affected-samples.sh
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Discovered from disk (every samples/<lang>/<name>/nimpack.yaml) rather than
# hand-maintained, so this can't drift from the SAMPLES list in
# verify-builds.sh — adding/removing a sample directory is enough.
mapfile -t ALL_SAMPLES < <(
  find "$repo_root/samples" -mindepth 3 -maxdepth 3 -name nimpack.yaml \
    | sed -e "s#^$repo_root/samples/##" -e 's#/nimpack.yaml$##' \
    | sort
)

# samples/<dir> -> pkg/packs/<pack> and pkg/templates/templates/<pack> name,
# for the directories where they differ.
pack_for_lang_dir() {
  case "$1" in
    go) echo golang ;;
    web) echo webserver ;;
    *) echo "$1" ;;
  esac
}

# to_json_array <item...> — no jq dependency; sample names are always
# plain "lang/name" tokens, so no escaping is needed.
to_json_array() {
  local out="[" first=1
  for item in "$@"; do
    if [[ $first -eq 1 ]]; then first=0; else out+=","; fi
    out+="\"$item\""
  done
  out+="]"
  printf '%s\n' "$out"
}

print_full() {
  to_json_array "${ALL_SAMPLES[@]}"
  exit 0
}

files="$(cat)"
[[ -z "$files" ]] && print_full

selected=()
while IFS= read -r f; do
  [[ -z "$f" ]] && continue

  case "$f" in
    samples/*/*)
      rest="${f#samples/}"
      selected+=("$(cut -d/ -f1,2 <<<"$rest")")
      continue
      ;;
  esac

  matched_pack=""
  case "$f" in
    pkg/packs/*/*) matched_pack="$(cut -d/ -f3 <<<"$f")" ;;
    pkg/templates/templates/*/*) matched_pack="$(cut -d/ -f4 <<<"$f")" ;;
  esac
  if [[ -n "$matched_pack" ]]; then
    for s in "${ALL_SAMPLES[@]}"; do
      dir="${s%%/*}"
      if [[ "$(pack_for_lang_dir "$dir")" == "$matched_pack" ]]; then
        selected+=("$s")
      fi
    done
    if [[ "$matched_pack" == "golang" ]]; then
      for s in "${ALL_SAMPLES[@]}"; do
        [[ "$s" == showcase/* ]] && selected+=("$s")
      done
    fi
    continue
  fi

  # Anything else is treated as shared/core: force a full run.
  print_full
done <<<"$files"

[[ ${#selected[@]} -eq 0 ]] && print_full

mapfile -t deduped < <(printf '%s\n' "${selected[@]}" | sort -u)
to_json_array "${deduped[@]}"
