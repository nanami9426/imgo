#!/usr/bin/env bash
set -euo pipefail

# 这个脚本用于验证 /v1/chat/completions 的双维度限流：
# 1) 请求级（dimension=request）
# 2) token 级（dimension=token）
#
# 默认按下面配置校验（写在 config/app.yaml，并重启服务）：
# rate_limit:
#   request_per_min: 2
#   token_per_min: 3
#   token_k: 100
#   default_max_tokens: 256
#   window_seconds: 60
#   redis_prefix: rl:chat
#
# 可通过环境变量覆盖（见下方默认值）。

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:5000}"
JWT="${JWT:-}"
MODEL="${MODEL:-Qwen/Qwen2.5-1.5B-Instruct}"

# 测试模式：request | token | all
MODE="${MODE:-all}"

# 固定窗口秒数（需要与你的 YAML 一致，用于跨窗口等待）
WINDOW_SECONDS="${WINDOW_SECONDS:-60}"
# 是否在每个用例前等待到新窗口，1=等待，0=不等待
WAIT_NEXT_WINDOW="${WAIT_NEXT_WINDOW:-1}"

# 请求级限流参数（与 YAML 对应）
REQUEST_LIMIT="${REQUEST_LIMIT:-2}"

# token 级限流参数（与 YAML 对应）
TOKEN_LIMIT="${TOKEN_LIMIT:-3}"
TOKEN_K="${TOKEN_K:-100}"
TOKEN_TEST_MAX_TOKENS="${TOKEN_TEST_MAX_TOKENS:-256}"
TOKEN_TEST_CONTENT="${TOKEN_TEST_CONTENT:-}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing dependency: $1"
    exit 1
  }
}

fail() {
  echo "[FAIL] $1"
  exit 1
}

pass() {
  echo "[PASS] $1"
}

wait_for_next_window() {
  if [[ "$WAIT_NEXT_WINDOW" != "1" ]]; then
    return
  fi
  if ! [[ "$WINDOW_SECONDS" =~ ^[0-9]+$ ]] || [[ "$WINDOW_SECONDS" -le 0 ]]; then
    fail "WINDOW_SECONDS must be positive integer, got: $WINDOW_SECONDS"
  fi
  local now
  now="$(date +%s)"
  local remain=$((WINDOW_SECONDS - (now % WINDOW_SECONDS) + 1))
  echo "Waiting ${remain}s for next rate-limit window..."
  sleep "$remain"
}

send_chat() {
  local payload="$1"
  local body_file
  body_file="$(mktemp /tmp/rl_resp.XXXXXX)"
  local code
  code="$(curl -sS -o "$body_file" -w "%{http_code}" \
    "$GATEWAY_URL/v1/chat/completions" \
    -H "Authorization: Bearer $JWT" \
    -H "Content-Type: application/json" \
    -d "$payload" || echo "000")"
  echo "${code}|${body_file}"
}

extract_dimension() {
  local body_file="$1"
  local d
  d="$(grep -o 'dimension=[^"[:space:]]*' "$body_file" | head -n1 || true)"
  if [[ -z "$d" ]]; then
    echo ""
    return
  fi
  echo "${d#dimension=}"
}

cleanup_file() {
  local file="$1"
  [[ -f "$file" ]] && rm -f "$file"
}

run_request_case() {
  if ! [[ "$REQUEST_LIMIT" =~ ^[0-9]+$ ]]; then
    fail "REQUEST_LIMIT must be non-negative integer, got: $REQUEST_LIMIT"
  fi
  if [[ "$REQUEST_LIMIT" -le 0 ]]; then
    fail "REQUEST_LIMIT must be > 0 to test request-level limiting"
  fi

  wait_for_next_window

  echo "== Request-level limit test =="
  echo "Expect first ${REQUEST_LIMIT} requests pass, then request $((REQUEST_LIMIT + 1)) returns 429 (dimension=request)."

  local i
  for ((i = 1; i <= REQUEST_LIMIT + 1; i++)); do
    local payload
    payload="$(cat <<JSON
{
  "model":"$MODEL",
  "messages":[{"role":"user","content":"ping"}],
  "stream":false,
  "temperature":0.0,
  "max_tokens":1
}
JSON
)"
    local result
    result="$(send_chat "$payload")"
    local code="${result%%|*}"
    local body_file="${result#*|}"
    local dim
    dim="$(extract_dimension "$body_file")"

    if [[ "$i" -le "$REQUEST_LIMIT" ]]; then
      if [[ "$code" == "429" ]]; then
        cleanup_file "$body_file"
        fail "request #$i unexpectedly got 429 (dimension=${dim:-unknown})"
      fi
      if [[ ! "$code" =~ ^2 ]]; then
        echo "Response body:"
        cat "$body_file" || true
        cleanup_file "$body_file"
        fail "request #$i expected 2xx, got $code"
      fi
    else
      if [[ "$code" != "429" ]]; then
        echo "Response body:"
        cat "$body_file" || true
        cleanup_file "$body_file"
        fail "request #$i expected 429, got $code"
      fi
      if [[ "$dim" != "request" ]]; then
        echo "Response body:"
        cat "$body_file" || true
        cleanup_file "$body_file"
        fail "request #$i expected dimension=request, got dimension=${dim:-unknown}"
      fi
    fi
    cleanup_file "$body_file"
  done

  pass "Request-level limiting works"
}

run_token_case() {
  if ! [[ "$TOKEN_LIMIT" =~ ^[0-9]+$ ]] || ! [[ "$TOKEN_K" =~ ^[0-9]+$ ]] || ! [[ "$TOKEN_TEST_MAX_TOKENS" =~ ^[0-9]+$ ]]; then
    fail "TOKEN_LIMIT/TOKEN_K/TOKEN_TEST_MAX_TOKENS must be non-negative integers"
  fi
  if [[ "$TOKEN_LIMIT" -le 0 ]]; then
    fail "TOKEN_LIMIT must be > 0 to test token-level limiting"
  fi
  if [[ "$TOKEN_K" -le 0 ]]; then
    fail "TOKEN_K must be > 0"
  fi

  wait_for_next_window

  # 与后端估算逻辑对齐：role + content 的 UTF-8 bytes，再 ceil(bytes/4)。
  local prompt_bytes
  prompt_bytes="$(printf '%s%s' "user" "$TOKEN_TEST_CONTENT" | wc -c | tr -d ' ')"
  local prompt_est=$(((prompt_bytes + 3) / 4))
  local token_cost=$(((prompt_est + TOKEN_TEST_MAX_TOKENS + TOKEN_K - 1) / TOKEN_K))
  if [[ "$token_cost" -lt 1 ]]; then
    token_cost=1
  fi
  local attempts=$((TOKEN_LIMIT / token_cost + 1))

  echo "== Token-level limit test =="
  echo "prompt_bytes=$prompt_bytes prompt_est=$prompt_est token_cost=$token_cost token_limit=$TOKEN_LIMIT attempts=$attempts"
  echo "Expect first $((attempts - 1)) requests pass, request #$attempts returns 429 (dimension=token)."
  if [[ "$REQUEST_LIMIT" -gt 0 && "$attempts" -gt "$REQUEST_LIMIT" ]]; then
    echo "[WARN] REQUEST_LIMIT=$REQUEST_LIMIT may trigger request-level 429 before token-level."
    echo "       Set request_per_min=0 or >=$attempts for a clean token-level test."
  fi

  local i
  for ((i = 1; i <= attempts; i++)); do
    local payload
    payload="$(cat <<JSON
{
  "model":"$MODEL",
  "messages":[{"role":"user","content":"$TOKEN_TEST_CONTENT"}],
  "stream":false,
  "temperature":0.0,
  "max_tokens":$TOKEN_TEST_MAX_TOKENS
}
JSON
)"
    local result
    result="$(send_chat "$payload")"
    local code="${result%%|*}"
    local body_file="${result#*|}"
    local dim
    dim="$(extract_dimension "$body_file")"

    if [[ "$i" -lt "$attempts" ]]; then
      if [[ "$code" == "429" ]]; then
        echo "Response body:"
        cat "$body_file" || true
        cleanup_file "$body_file"
        fail "request #$i unexpectedly got 429 (dimension=${dim:-unknown})"
      fi
      if [[ ! "$code" =~ ^2 ]]; then
        echo "Response body:"
        cat "$body_file" || true
        cleanup_file "$body_file"
        fail "request #$i expected 2xx, got $code"
      fi
    else
      if [[ "$code" != "429" ]]; then
        echo "Response body:"
        cat "$body_file" || true
        cleanup_file "$body_file"
        fail "request #$i expected 429, got $code"
      fi
      if [[ "$dim" != "token" ]]; then
        echo "Response body:"
        cat "$body_file" || true
        cleanup_file "$body_file"
        fail "request #$i expected dimension=token, got dimension=${dim:-unknown}"
      fi
    fi
    cleanup_file "$body_file"
  done

  pass "Token-level limiting works"
}

main() {
  need curl
  if [[ -z "$JWT" ]]; then
    fail "JWT is empty. Example: export JWT='xxx.yyy.zzz'"
  fi

  echo "Gateway: $GATEWAY_URL"
  echo "Mode: $MODE"
  echo "Model: $MODEL"

  case "$MODE" in
    request)
      run_request_case
      ;;
    token)
      run_token_case
      ;;
    all)
      run_request_case
      run_token_case
      ;;
    *)
      fail "Unsupported MODE=$MODE. Use request|token|all"
      ;;
  esac

  echo "== Rate-limit tests completed =="
}

main "$@"
