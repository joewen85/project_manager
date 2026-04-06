#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env"
PROVIDER="${1:-auto}"

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

timestamp() {
  date +"%Y-%m-%d %H:%M:%S"
}

log() {
  echo "[$(timestamp)] $*"
}

json_get() {
  local key="$1"
  python3 - "$key" <<'PY'
import json
import sys

path = sys.argv[1].split(".")
raw = sys.stdin.read().strip()
if not raw:
    print("")
    raise SystemExit(0)
try:
    data = json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)
value = data
for part in path:
    if isinstance(value, dict):
        value = value.get(part)
    else:
        value = None
        break
if value is None:
    print("")
elif isinstance(value, (dict, list)):
    print(json.dumps(value, ensure_ascii=False))
else:
    print(value)
PY
}

require_command() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing command: $cmd"
    exit 1
  fi
}

require_command curl
require_command python3

build_test_message() {
  local channel="$1"
  printf "任务通知自检消息\n渠道：%s\n时间：%s\n来源：scripts/test-task-notify.sh" "$channel" "$(timestamp)"
}

send_wecom() {
  local corp_id="${WECOM_CORP_ID:-}"
  local corp_secret="${WECOM_CORP_SECRET:-}"
  local agent_id="${WECOM_AGENT_ID:-}"
  local to_user="${WECOM_TO_USER:-@all}"
  if [[ -z "$corp_id" || -z "$corp_secret" || -z "$agent_id" ]]; then
    log "skip wecom: missing WECOM_CORP_ID/WECOM_CORP_SECRET/WECOM_AGENT_ID"
    return 1
  fi

  local token_resp
  token_resp="$(curl -sS "https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=${corp_id}&corpsecret=${corp_secret}")"
  local token_errcode
  token_errcode="$(printf '%s' "$token_resp" | json_get errcode)"
  if [[ "${token_errcode:-}" != "0" ]]; then
    log "wecom get token failed: $token_resp"
    return 1
  fi
  local access_token
  access_token="$(printf '%s' "$token_resp" | json_get access_token)"
  if [[ -z "${access_token:-}" ]]; then
    log "wecom get token failed: access_token empty"
    return 1
  fi

  local content payload send_resp send_errcode
  content="$(build_test_message "wecom")"
  payload="$(python3 - "$to_user" "$agent_id" "$content" <<'PY'
import json
import sys
print(json.dumps({
  "touser": sys.argv[1],
  "msgtype": "text",
  "agentid": int(sys.argv[2]),
  "text": {"content": sys.argv[3]},
  "safe": 0
}, ensure_ascii=False))
PY
)"
  send_resp="$(curl -sS -X POST "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=${access_token}" \
    -H "Content-Type: application/json" \
    -d "$payload")"
  send_errcode="$(printf '%s' "$send_resp" | json_get errcode)"
  if [[ "${send_errcode:-}" != "0" ]]; then
    log "wecom send failed: $send_resp"
    return 1
  fi
  log "wecom notify test success"
  return 0
}

send_dingtalk() {
  local webhook="${DINGTALK_WEBHOOK:-}"
  local secret="${DINGTALK_SECRET:-}"
  if [[ -z "$webhook" ]]; then
    log "skip dingtalk: missing DINGTALK_WEBHOOK"
    return 1
  fi

  local request_url="$webhook"
  if [[ -n "$secret" ]]; then
    local sign_result timestamp_ms sign
    sign_result="$(python3 - "$secret" <<'PY'
import base64
import hashlib
import hmac
import time
import urllib.parse
import sys
secret = sys.argv[1]
ts = str(int(time.time() * 1000))
string_to_sign = f"{ts}\n{secret}".encode("utf-8")
sign = base64.b64encode(hmac.new(secret.encode("utf-8"), string_to_sign, hashlib.sha256).digest()).decode("utf-8")
print(ts)
print(urllib.parse.quote_plus(sign))
PY
)"
    timestamp_ms="$(printf '%s' "$sign_result" | sed -n '1p')"
    sign="$(printf '%s' "$sign_result" | sed -n '2p')"
    if [[ "$request_url" == *"?"* ]]; then
      request_url="${request_url}&timestamp=${timestamp_ms}&sign=${sign}"
    else
      request_url="${request_url}?timestamp=${timestamp_ms}&sign=${sign}"
    fi
  fi

  local text payload resp errcode
  text="$(build_test_message "dingtalk")"
  payload="$(python3 - "$text" <<'PY'
import json
import sys
msg = sys.argv[1]
print(json.dumps({
  "msgtype": "markdown",
  "markdown": {
    "title": "任务通知自检",
    "text": "### 任务通知自检\n\n" + msg.replace("\n", "\n\n")
  }
}, ensure_ascii=False))
PY
)"
  resp="$(curl -sS -X POST "$request_url" -H "Content-Type: application/json" -d "$payload")"
  errcode="$(printf '%s' "$resp" | json_get errcode)"
  if [[ "${errcode:-}" != "0" ]]; then
    log "dingtalk send failed: $resp"
    return 1
  fi
  log "dingtalk notify test success"
  return 0
}

send_feishu() {
  local app_id="${FEISHU_APP_ID:-}"
  local app_secret="${FEISHU_APP_SECRET:-}"
  local receive_type="${FEISHU_RECEIVE_ID_TYPE:-email}"
  local receive_id="${FEISHU_RECEIVE_ID:-}"
  if [[ -z "$app_id" || -z "$app_secret" ]]; then
    log "skip feishu: missing FEISHU_APP_ID/FEISHU_APP_SECRET"
    return 1
  fi
  if [[ -z "$receive_id" ]]; then
    log "skip feishu: missing FEISHU_RECEIVE_ID (self-test requires explicit receive id)"
    return 1
  fi

  local token_payload token_resp code access_token
  token_payload="$(python3 - "$app_id" "$app_secret" <<'PY'
import json
import sys
print(json.dumps({"app_id": sys.argv[1], "app_secret": sys.argv[2]}))
PY
)"
  token_resp="$(curl -sS -X POST "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal" \
    -H "Content-Type: application/json" \
    -d "$token_payload")"
  code="$(printf '%s' "$token_resp" | json_get code)"
  if [[ "${code:-}" != "0" ]]; then
    log "feishu get token failed: $token_resp"
    return 1
  fi
  access_token="$(printf '%s' "$token_resp" | json_get tenant_access_token)"
  if [[ -z "${access_token:-}" ]]; then
    log "feishu get token failed: tenant_access_token empty"
    return 1
  fi

  local text content payload resp send_code
  text="$(build_test_message "feishu")"
  content="$(python3 - "$text" <<'PY'
import json
import sys
print(json.dumps({"text": sys.argv[1]}, ensure_ascii=False))
PY
)"
  payload="$(python3 - "$receive_id" "$content" <<'PY'
import json
import sys
print(json.dumps({
  "receive_id": sys.argv[1],
  "msg_type": "text",
  "content": sys.argv[2]
}, ensure_ascii=False))
PY
)"
  resp="$(curl -sS -X POST "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=${receive_type}" \
    -H "Content-Type: application/json; charset=utf-8" \
    -H "Authorization: Bearer ${access_token}" \
    -d "$payload")"
  send_code="$(printf '%s' "$resp" | json_get code)"
  if [[ "${send_code:-}" != "0" ]]; then
    log "feishu send failed: $resp"
    return 1
  fi
  log "feishu notify test success"
  return 0
}

send_email() {
  local host="${SMTP_HOST:-}"
  local port="${SMTP_PORT:-25}"
  local username="${SMTP_USERNAME:-}"
  local password="${SMTP_PASSWORD:-}"
  local from_addr="${SMTP_FROM:-}"
  local to_addr="${SMTP_TEST_TO:-${SMTP_FROM:-}}"
  if [[ -z "$host" || -z "$port" || -z "$from_addr" ]]; then
    log "skip email: missing SMTP_HOST/SMTP_PORT/SMTP_FROM"
    return 1
  fi
  if [[ -z "$to_addr" ]]; then
    log "skip email: missing SMTP_TEST_TO and SMTP_FROM"
    return 1
  fi

  local text
  text="$(build_test_message "email")"
  SMTP_HOST="$host" SMTP_PORT="$port" SMTP_USERNAME="$username" SMTP_PASSWORD="$password" SMTP_FROM="$from_addr" SMTP_TO="$to_addr" SMTP_TEXT="$text" python3 - <<'PY'
import os
import smtplib
from email.mime.text import MIMEText
from email.header import Header

host = os.environ["SMTP_HOST"]
port = int(os.environ["SMTP_PORT"])
username = os.environ.get("SMTP_USERNAME", "")
password = os.environ.get("SMTP_PASSWORD", "")
from_addr = os.environ["SMTP_FROM"]
to_addr = os.environ["SMTP_TO"]
text = os.environ.get("SMTP_TEXT", "")

msg = MIMEText(text, "plain", "utf-8")
msg["From"] = from_addr
msg["To"] = to_addr
msg["Subject"] = Header("[任务通知] 自检消息", "utf-8")

with smtplib.SMTP(host, port, timeout=8) as client:
    if username and password:
        client.login(username, password)
    client.sendmail(from_addr, [to_addr], msg.as_string())
PY
  log "email notify test success -> ${to_addr}"
  return 0
}

resolve_auto_non_email_provider() {
  local selected
  selected="$(echo "${TASK_NOTIFY_PROVIDER:-}" | tr '[:upper:]' '[:lower:]' | xargs)"
  local configured=()
  [[ -n "${WECOM_CORP_ID:-}" && -n "${WECOM_CORP_SECRET:-}" && -n "${WECOM_AGENT_ID:-}" ]] && configured+=("wecom")
  [[ -n "${DINGTALK_WEBHOOK:-}" ]] && configured+=("dingtalk")
  [[ -n "${FEISHU_APP_ID:-}" && -n "${FEISHU_APP_SECRET:-}" ]] && configured+=("feishu")

  if [[ -z "$selected" || "$selected" == "auto" ]]; then
    if [[ ${#configured[@]} -eq 1 ]]; then
      echo "${configured[0]}"
      return 0
    fi
    if [[ ${#configured[@]} -gt 1 ]]; then
      log "non-email provider conflict: ${configured[*]} (set TASK_NOTIFY_PROVIDER)"
      return 1
    fi
    echo ""
    return 0
  fi
  if [[ "$selected" == "none" ]]; then
    echo ""
    return 0
  fi
  case "$selected" in
    wecom|dingtalk|feishu)
      echo "$selected"
      return 0
      ;;
    *)
      log "invalid TASK_NOTIFY_PROVIDER: ${selected}"
      return 1
      ;;
  esac
}

main() {
  local ok=0
  local fail=0

  case "$PROVIDER" in
    email)
      send_email && ((ok+=1)) || ((fail+=1))
      ;;
    wecom)
      send_wecom && ((ok+=1)) || ((fail+=1))
      ;;
    dingtalk)
      send_dingtalk && ((ok+=1)) || ((fail+=1))
      ;;
    feishu)
      send_feishu && ((ok+=1)) || ((fail+=1))
      ;;
    auto)
      if [[ -n "${SMTP_HOST:-}" && -n "${SMTP_PORT:-}" && -n "${SMTP_FROM:-}" ]]; then
        send_email && ((ok+=1)) || ((fail+=1))
      else
        log "skip email: smtp not configured"
      fi

      local non_email_provider
      if ! non_email_provider="$(resolve_auto_non_email_provider)"; then
        ((fail+=1))
      elif [[ -z "$non_email_provider" ]]; then
        log "skip non-email: no provider configured"
      else
        case "$non_email_provider" in
          wecom) send_wecom && ((ok+=1)) || ((fail+=1)) ;;
          dingtalk) send_dingtalk && ((ok+=1)) || ((fail+=1)) ;;
          feishu) send_feishu && ((ok+=1)) || ((fail+=1)) ;;
        esac
      fi
      ;;
    *)
      echo "usage: bash scripts/test-task-notify.sh [auto|email|wecom|dingtalk|feishu]"
      exit 1
      ;;
  esac

  log "notify self-test done: success=${ok}, failed=${fail}"
  if [[ "$fail" -gt 0 ]]; then
    exit 1
  fi
}

main
