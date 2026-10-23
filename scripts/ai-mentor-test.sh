#!/usr/bin/env bash
# Uji fitur mentor AI ("MinSmart") pada ulasan hasil tryout.
#
# Alur: daftar peserta → kerjakan tryout (sebagian salah) → submit →
#       analisis AI → jelaskan soal → tanya-jawab → streaming → riwayat →
#       kuota → pagar pengaman → reset.
#
# Pemakaian: scripts/ai-mentor-test.sh [base-url]
set -euo pipefail

API="${1:-http://127.0.0.1:8080/api/v1}"
PASS=0
FAIL=0

say()  { printf '\n\033[36m== %s\033[0m\n' "$1"; }
ok()   { PASS=$((PASS+1)); printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  \033[31m✗\033[0m %s\n' "$1"; }
check(){ if [ "$2" = "$3" ]; then ok "$1 ($2)"; else bad "$1 — harap [$3], dapat [$2]"; fi; }

req() { curl -s -m 30 -H 'Content-Type: application/json' "$@"; }
jq1() { python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]'"$1"')'; }

say "Status mentor AI"
STATUS=$(req "$API/ai/status")
check "mentor AI aktif" "$(echo "$STATUS" | jq1 "['enabled']")" "True"
ok "provider: $(echo "$STATUS" | jq1 "['provider']") / model: $(echo "$STATUS" | jq1 "['model']")"

say "Siapkan peserta + sesi tryout"
STAMP=$(date +%s)
TOKEN=$(req -X POST "$API/auth/register" \
  -d "{\"name\":\"Mentor Test $STAMP\",\"email\":\"mentor-$STAMP@bumnacademy.id\",\"password\":\"rahasia123\"}" \
  | jq1 "['accessToken']")
[ -n "$TOKEN" ] && ok "peserta terdaftar" || bad "pendaftaran gagal"
AUTH="Authorization: Bearer $TOKEN"
echo "$TOKEN" > /tmp/ba-mentor-token.txt

SESSION=$(req -X POST "$API/tryouts/to-demo-nasional/sessions" -H "$AUTH")
SESSION_ID=$(echo "$SESSION" | jq1 "['id']")

# Jawab sengaja: soal ganjil benar, sebagian salah, satu dibiarkan kosong
# supaya mentor punya bahan pembahasan yang nyata.
BODY=$(echo "$SESSION" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
answers = []
for st in d["subtests"]:
    for i, q in enumerate(st["questions"]):
        if i == 1:
            answers.append({"questionId": q["id"], "selectedIndex": None, "flagged": False})
        elif i % 2 == 0:
            answers.append({"questionId": q["id"], "selectedIndex": q["options"] and 0, "flagged": False})
        else:
            answers.append({"questionId": q["id"], "selectedIndex": 1, "flagged": True})
print(json.dumps({"subtestIndex": len(d["subtests"]) - 1, "answers": answers}))')

req -X PATCH "$API/tryout-sessions/$SESSION_ID/answers" -H "$AUTH" -d "$BODY" > /dev/null
RESULT=$(req -X POST "$API/tryout-sessions/$SESSION_ID/submit" -H "$AUTH")
ATTEMPT_ID=$(echo "$RESULT" | jq1 "['id']")
SCORE=$(echo "$RESULT" | jq1 "['score']")
[ -n "$ATTEMPT_ID" ] && ok "hasil tryout tersimpan, skor $SCORE" || bad "submit gagal"
echo "$ATTEMPT_ID" > /tmp/ba-mentor-attempt.txt

say "Analisis hasil tryout"
ANALYSIS=$(req "$API/attempts/$ATTEMPT_ID/ai/analysis" -H "$AUTH")
echo "$ANALYSIS" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
print("  headline :", d["headline"])
print("  kesiapan :", d["readiness"], d["readinessScore"], "(ambang", d["passingThreshold"], ")")
print("  ringkas  :", d["summary"][:170])
print("  kekuatan :", [s["title"] for s in d["strengths"]])
print("  kelemahan:", [s["title"] for s in d["weaknesses"]])
print("  fokus    :", [s["title"] for s in d["focusAreas"]])
print("  catatan  :", len(d["questionNotes"]), "butir")
'
for FIELD in headline summary strengths focusAreas studyPlan questionNotes readinessScore; do
  VALUE=$(echo "$ANALYSIS" | FIELD="$FIELD" python3 -c 'import os, sys, json
d = json.load(sys.stdin)["data"]
v = d[os.environ["FIELD"]]
print("ok" if v not in (None, "", []) else "kosong")')
  check "analisis.$FIELD terisi" "$VALUE" "ok"
done
READINESS=$(echo "$ANALYSIS" | jq1 "['readiness']")
case "$READINESS" in
  siap|"hampir siap"|"perlu perbaikan") ok "label kesiapan valid: $READINESS" ;;
  *) bad "label kesiapan tidak dikenal: $READINESS" ;;
esac

say "Analisis dipakai ulang (cache)"
check "panggilan kedua memakai cache" "$(req "$API/attempts/$ATTEMPT_ID/ai/analysis" -H "$AUTH" | jq1 "['cached']")" "True"

say "Mentor menjelaskan satu soal"
QUESTION_ID=$(echo "$ANALYSIS" | python3 -c '
import sys, json
notes = [n for n in json.load(sys.stdin)["data"]["questionNotes"] if n.get("questionId")]
print(notes[0]["questionId"] if notes else "")')
if [ -n "$QUESTION_ID" ]; then
  EXPLAIN=$(req -X POST "$API/attempts/$ATTEMPT_ID/ai/explain" -H "$AUTH" -d "{\"questionId\":\"$QUESTION_ID\"}")
  LENGTH=$(echo "$EXPLAIN" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["message"]["content"]))')
  [ "$LENGTH" -gt 200 ] && ok "penjelasan soal dihasilkan ($LENGTH karakter)" || bad "penjelasan terlalu pendek ($LENGTH)"
  echo "$EXPLAIN" | python3 -c 'import sys,json;print("  ---");print(json.load(sys.stdin)["data"]["message"]["content"][:500])'
else
  bad "tidak ada catatan soal untuk dijelaskan"
fi

say "Tanya mentor (chat)"
CHAT=$(req -X POST "$API/attempts/$ATTEMPT_ID/ai/chat" -H "$AUTH" \
  -d '{"message":"Kenapa jawabanku salah? Apa yang harus aku perbaiki dulu?"}')
check "balasan tersimpan sebagai assistant" "$(echo "$CHAT" | jq1 "['message']['role']")" "assistant"
echo "$CHAT" | python3 -c 'import sys,json;print("  ---");print(json.load(sys.stdin)["data"]["message"]["content"][:520])'

say "Chat dengan fokus soal"
if [ -n "$QUESTION_ID" ]; then
  FOCUS=$(req -X POST "$API/attempts/$ATTEMPT_ID/ai/chat" -H "$AUTH" \
    -d "{\"message\":\"Jelaskan konsep di balik soal ini\",\"questionId\":\"$QUESTION_ID\"}")
  check "cakupan pesan = question" "$(echo "$FOCUS" | jq1 "['message']['scope']")" "question"
  QNUM=$(echo "$FOCUS" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["message"].get("questionNumber"))')
  [ "$QNUM" != "None" ] && ok "nomor soal terlampir pada pesan (nomor $QNUM)" || bad "nomor soal tidak ada"
fi

say "Streaming balasan (SSE)"
STREAM=$(curl -s -m 40 -N -X POST "$API/attempts/$ATTEMPT_ID/ai/chat/stream" -H "$AUTH" \
  -H 'Content-Type: application/json' -d '{"message":"Beri aku strategi membagi waktu antar subtes"}')
echo "$STREAM" | grep -q '^event: user' && ok "kejadian 'user' diterima" || bad "kejadian user tidak ada"
DELTAS=$(echo "$STREAM" | grep -c '^event: delta' || true)
[ "$DELTAS" -gt 0 ] && ok "potongan balasan diterima ($DELTAS delta)" || bad "tidak ada delta"
echo "$STREAM" | grep -q '^event: done' && ok "kejadian 'done' diterima" || bad "kejadian done tidak ada"
echo "$STREAM" | grep '^event: delta' | head -2 | sed 's/^/  /'

say "Riwayat percakapan"
CONV=$(req "$API/attempts/$ATTEMPT_ID/ai/conversation" -H "$AUTH")
MESSAGES=$(echo "$CONV" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["messages"]))')
[ "$MESSAGES" -ge 6 ] && ok "riwayat berisi $MESSAGES pesan" || bad "riwayat terlalu pendek ($MESSAGES)"
echo "$CONV" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]["messages"]
print("  user:", sum(1 for m in d if m["role"] == "user"), "| assistant:", sum(1 for m in d if m["role"] == "assistant"))'

say "Kuota pemakaian"
USAGE=$(req "$API/ai/usage" -H "$AUTH")
echo "$USAGE" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
print("  permintaan:", d["requests"], "/", d["dailyQuota"], "| token prompt:", d["promptTokens"], "| token balasan:", d["completionTokens"])'
REQ_COUNT=$(echo "$USAGE" | jq1 "['requests']")
[ "$REQ_COUNT" -ge 3 ] && ok "pemakaian tercatat ($REQ_COUNT permintaan)" || bad "pemakaian tidak tercatat"

say "Pagar pengaman"
NOT_MINE=$(req -X POST "$API/attempts/00000000-0000-0000-0000-000000000000/ai/chat" -H "$AUTH" -d '{"message":"halo"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "attempt tidak dikenal ditolak" "$NOT_MINE" "NOT_FOUND"

FOREIGN_Q=$(req -X POST "$API/attempts/$ATTEMPT_ID/ai/chat" -H "$AUTH" -d '{"message":"jelaskan","questionId":"soal-palsu"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "soal di luar hasil tryout ditolak" "$FOREIGN_Q" "BAD_REQUEST"

EMPTY=$(req -X POST "$API/attempts/$ATTEMPT_ID/ai/chat" -H "$AUTH" -d '{"message":""}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "pesan kosong ditolak" "$EMPTY" "VALIDATION_ERROR"

UNAUTH=$(req "$API/attempts/$ATTEMPT_ID/ai/analysis" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "tanpa token ditolak" "$UNAUTH" "UNAUTHORIZED"

say "Reset percakapan"
check "percakapan dikosongkan" "$(req -X DELETE "$API/attempts/$ATTEMPT_ID/ai/conversation" -H "$AUTH" | jq1 "['reset']")" "True"
AFTER=$(req "$API/attempts/$ATTEMPT_ID/ai/conversation" -H "$AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["messages"]))')
check "riwayat kosong setelah reset" "$AFTER" "0"
check "analisis tetap tersimpan setelah reset" "$(req "$API/attempts/$ATTEMPT_ID/ai/analysis" -H "$AUTH" | jq1 "['cached']")" "True"

printf '\n\033[1mLulus: %d, Gagal: %d\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
