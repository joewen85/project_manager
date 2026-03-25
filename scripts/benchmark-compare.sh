#!/usr/bin/env bash
set -euo pipefail

BEFORE_FILE="${BEFORE_FILE:-docs/benchmark/before.md}"
AFTER_FILE="${AFTER_FILE:-docs/benchmark/after.md}"
OUT_FILE="${OUT_FILE:-docs/benchmark/compare.md}"

extract_rows() {
  local file="$1"
  awk -F'|' '
    /^\| `\/api\/v1\// {
      endpoint=$2
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", endpoint)
      gsub(/`/, "", endpoint)

      avg=$3
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", avg)
      min=$4
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", min)
      max=$5
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", max)

      print endpoint "|" avg "|" min "|" max
    }
  ' "$file"
}

main() {
  if [[ ! -f "$BEFORE_FILE" ]]; then
    echo "before report not found: $BEFORE_FILE" >&2
    exit 1
  fi
  if [[ ! -f "$AFTER_FILE" ]]; then
    echo "after report not found: $AFTER_FILE" >&2
    exit 1
  fi

  mkdir -p "$(dirname "$OUT_FILE")"

  tmp_before="$(mktemp)"
  tmp_after="$(mktemp)"
  trap 'rm -f "$tmp_before" "$tmp_after"' EXIT

  extract_rows "$BEFORE_FILE" > "$tmp_before"
  extract_rows "$AFTER_FILE" > "$tmp_after"

  {
    echo "# API Benchmark Compare"
    echo
    echo "- before: \`$BEFORE_FILE\`"
    echo "- after: \`$AFTER_FILE\`"
    echo "- generated_at: \`$(date '+%Y-%m-%d %H:%M:%S %z')\`"
    echo
    echo "| Endpoint | Before avg (s) | Before min/max (s) | After avg (s) | After min/max (s) | Diff avg |"
    echo "|---|---:|---:|---:|---:|---:|"

    awk -F'|' '
      FNR==NR {
        bavg[$1]=$2
        bmin[$1]=$3
        bmax[$1]=$4
        next
      }
      {
        endpoint=$1
        aavg=$2
        amin=$3
        amax=$4

        beforeAvg=(endpoint in bavg ? bavg[endpoint] : "N/A")
        beforeMin=(endpoint in bmin ? bmin[endpoint] : "N/A")
        beforeMax=(endpoint in bmax ? bmax[endpoint] : "N/A")

        numericBefore=(beforeAvg ~ /^[0-9]+(\.[0-9]+)?$/)
        numericAfter=(aavg ~ /^[0-9]+(\.[0-9]+)?$/)

        if (numericBefore && numericAfter) {
          diff=sprintf("%.6f", aavg - beforeAvg)
        } else {
          diff="N/A"
        }

        printf "| `%s` | %s | %s / %s | %s | %s / %s | %s |\n",
          endpoint, beforeAvg, beforeMin, beforeMax, aavg, amin, amax, diff
      }
    ' "$tmp_before" "$tmp_after"
  } > "$OUT_FILE"

  echo "Compare report written to: $OUT_FILE"
}

main "$@"
