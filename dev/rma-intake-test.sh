#!/usr/bin/env bash
# Local dev: send fake RMA Google Form submissions to the helpdesk, exactly as
# the Apps Script does (Google can't reach localhost). Fictional data only.
# Needs the RMA Form API key: run dev/rma-form-key.sh first.
#
#   dev/rma-intake-test.sh               send the built-in samples
#   dev/rma-intake-test.sh payload.json  send one submission from a file
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/local/rma-form.env"
BASE="${KD_URL:-http://127.0.0.1:9000}"

send() {
  curl -sS -H "Authorization: token $RMA_FORM_API_KEY:$RMA_FORM_API_SECRET" \
    -H 'Content-Type: application/json' --data @- "$BASE/api/v1/kd/rmas/intake" |
    jq -c 'if .status == "success"
      then {rma: .data.rma.number, customer: .data.rma.customer_name, ticket: .data.rma.ticket_ref, duplicate: .data.duplicate}
      else {error: .message} end'
}

if [ $# -gt 0 ]; then send < "$1"; exit; fi

run="test-$(date +%s)"
echo "1. Known contact (matched to their customer):"
send <<JSON
{"response_id": "$run-1", "email": "sean.murphy@brightline-sec.example", "name": "Seán Murphy",
 "company": "Brightline Security", "invoice_number": "INV-56102", "purchase_date": "2026-09-02",
 "brand": "Dahua", "product": "IPC-HFW2441S-S 4MP bullet camera", "quantity": "2",
 "serial_numbers": "7G0ABC1234567, 7G0ABC1234568", "fault": "No video after power cycle. IR works, stream is black.",
 "other": [["Preferred outcome", "Replacement"]]}
JSON

echo "2. New person at a known company (matched by email domain):"
send <<JSON
{"response_id": "$run-2", "email": "dave.oneill@leesidealarms.example", "name": "Dave O'Neill",
 "company": "Leeside Alarms Ltd", "phone": "083 555 0102", "invoice_number": "INV-55990",
 "purchase_date": "14/10/2026", "brand": "Ajax", "product": "MotionProtect Plus", "quantity": "1",
 "serial_numbers": "00A1B2C3", "fault": "False alarms every night, already swapped battery."}
JSON

echo "3. Unknown sender with a free email address (no customer match):"
send <<JSON
{"response_id": "$run-3", "email": "installs.kevin@gmail.com", "name": "Kevin Barry",
 "company": "KB Installs", "invoice_number": "", "purchase_date": "last summer",
 "brand": "Alta Labs", "product": "AP6-Pro access point", "quantity": "1",
 "serial_numbers": "", "fault": "Won't power up on PoE."}
JSON

echo "4. Same submission as 1 sent again (must not create a second RMA):"
send <<JSON
{"response_id": "$run-1", "email": "sean.murphy@brightline-sec.example", "product": "IPC-HFW2441S-S", "fault": "again"}
JSON
