#!/usr/bin/env bash
# Local dev: make sure the "RMA Form" agent exists and has an API key, saved to
# local/rma-form.env (git-ignored). The Google Form's Apps Script signs in with
# this key; dev/rma-intake-test.sh uses it to fake form submissions locally.
# On a hosted copy, do the same in the app: Admin -> Agents -> add "RMA Form",
# then "Generate API key" (see integrations/google-forms/README.md).
#
#   dev/rma-form-key.sh            reuse the saved key if it still works
#   dev/rma-form-key.sh --new      generate a fresh key (the old one stops working)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/local/secrets.env"
BASE="${KD_URL:-http://127.0.0.1:9000}"
OUT="$ROOT/local/rma-form.env"
EMAIL="rma-form@kd-helpdesk.local"

if [ "${1:-}" != "--new" ] && [ -f "$OUT" ]; then
  source "$OUT"
  if curl -fsS -o /dev/null -H "Authorization: token $RMA_FORM_API_KEY:$RMA_FORM_API_SECRET" "$BASE/api/v1/kd/rmas?page_size=1"; then
    echo "RMA Form API key in $OUT still works."
    exit 0
  fi
fi

jar="$(mktemp)"; trap 'rm -f "$jar"' EXIT
# Sign in as the System admin (password from local/secrets.env, sent in the body only)
jq -n --arg p "$SYSTEM_USER_PASSWORD" '{email: "System", password: $p}' |
  curl -fsS -o /dev/null -c "$jar" -b "$jar" -H 'Content-Type: application/json' --data @- "$BASE/api/v1/auth/login"
csrf="$(awk '$6 == "csrf_token" {print $7}' "$jar")"
api() { curl -fsS -b "$jar" -H "X-CSRFTOKEN: $csrf" -H 'Content-Type: application/json' "$@"; }

id="$(api "$BASE/api/v1/agents" | jq -r --arg e "$EMAIL" '.data[] | select(.email == $e) | .id' | head -1)"
if [ -z "$id" ]; then
  id="$(api --data '{"first_name":"RMA","last_name":"Form","email":"'"$EMAIL"'","roles":["Agent"],"enabled":true}' \
    "$BASE/api/v1/agents" | jq -r '.data.id')"
  echo "Created the RMA Form agent (id $id)."
fi

api -X POST "$BASE/api/v1/agents/$id/api-key" |
  jq -r '"RMA_FORM_API_KEY=\(.data.api_key)\nRMA_FORM_API_SECRET=\(.data.api_secret)"' > "$OUT"
chmod 600 "$OUT"
echo "Saved a new RMA Form API key to $OUT"
