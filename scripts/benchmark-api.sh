#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123}"
RUNS="${RUNS:-5}"
LABEL="${LABEL:-manual}"
OUT_FILE="${OUT_FILE:-}"

ENDPOINTS=(
  "/api/v1/stats/dashboard"
  "/api/v1/tasks?page=1&pageSize=20&sortBy=createdAt&sortOrder=desc"
  "/api/v1/projects?page=1&pageSize=20&sortBy=createdAt&sortOrder=desc"
)

login() {
  curl -sS -X POST "$BASE_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" \
    | sed -n 's/.*"token":"\([^"]*\)".*/\1/p'
}

calc_stats() {
  awk '
    BEGIN { min=1e9; max=-1e9; sum=0; count=0 }
    {
      value=$1 + 0
      if (value < min) min=value
      if (value > max) max=value
      sum += value
      count++
    }
    END {
      if (count == 0) {
        printf "0.000000|0.000000|0.000000"
      } else {
        printf "%.6f|%.6f|%.6f", sum/count, min, max
      }
    }
  '
}

measure_endpoint() {
  local endpoint="$1"
  local token="$2"
  local i
  local times=()
  local time_total

  echo "==> $endpoint" >&2
  for ((i=1; i<=RUNS; i++)); do
    time_total="$(curl -sS -o /dev/null -w "%{time_total}" \
      "$BASE_URL$endpoint" \
      -H "Authorization: Bearer $token" \
      -H "Content-Type: application/json")"
    times+=("$time_total")
    echo "  run $i: ${time_total}s" >&2
  done

  local stats
  stats="$(printf "%s\n" "${times[@]}" | calc_stats)"
  echo "$endpoint|$stats"
}

write_report() {
  local report_path="$1"
  shift
  local rows=("$@")

  mkdir -p "$(dirname "$report_path")"
  {
    echo "# API Benchmark ($LABEL)"
    echo
    echo "- base_url: \`$BASE_URL\`"
    echo "- runs: \`$RUNS\`"
    echo "- generated_at: \`$(date '+%Y-%m-%d %H:%M:%S %z')\`"
    echo
    echo "| Endpoint | avg (s) | min (s) | max (s) |"
    echo "|---|---:|---:|---:|"
    for row in "${rows[@]}"; do
      IFS='|' read -r endpoint avg min max <<< "$row"
      echo "| \`$endpoint\` | $avg | $min | $max |"
    done
  } > "$report_path"

  echo
  echo "Report written to: $report_path"
}

main() {
  local benchmark_rows=()
  token="$(login)"
  if [[ -z "${token}" ]]; then
    echo "login failed: empty token" >&2
    exit 1
  fi

  for endpoint in "${ENDPOINTS[@]}"; do
    benchmark_rows+=("$(measure_endpoint "$endpoint" "$token" | tail -n 1)")
  done

  if [[ -n "$OUT_FILE" ]]; then
    write_report "$OUT_FILE" "${benchmark_rows[@]}"
  fi
}

main "$@"
