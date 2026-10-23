#!/usr/bin/env bash
# Smoke test end-to-end API be-bumn-academy.
#
# Pemakaian:
#   scripts/smoke-test.sh [base-url]
#
# Contoh:
#   scripts/smoke-test.sh http://127.0.0.1:8080/api/v1
set -euo pipefail

API="${1:-http://127.0.0.1:8080/api/v1}"
BASE="${API%/api/v1}"
PASS=0
FAIL=0

say()  { printf '\n\033[36m== %s\033[0m\n' "$1"; }
ok()   { PASS=$((PASS+1)); printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  \033[31m✗\033[0m %s\n' "$1"; }
check(){ if [ "$2" = "$3" ]; then ok "$1 ($2)"; else bad "$1 — harap [$3], dapat [$2]"; fi; }

req() { curl -s -m 20 -H 'Content-Type: application/json' "$@"; }
jq1() { python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]'"$1"')'; }

say "Kesehatan"
STATUS=$(req "$BASE/healthz" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["status"])')
check "GET /healthz" "$STATUS" "ok"
READY=$(req "$BASE/readyz" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["database"]["status"])')
check "GET /readyz (database)" "$READY" "ok"

say "Katalog publik"
PRODUCTS=$(req "$API/products" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
check "jumlah produk" "$PRODUCTS" "4"
PROGRAMS=$(req "$API/programs" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
check "jumlah program" "$PROGRAMS" "4"
MATERI=$(req "$API/materi/categories" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
check "jumlah kategori materi" "$MATERI" "4"

say "Login peserta demo"
TOKEN=$(req -X POST "$API/auth/login" -d '{"email":"peserta@bumnacademy.id","password":"peserta123"}' | jq1 "['accessToken']")
[ -n "$TOKEN" ] && ok "login akun demo berhasil" || bad "login gagal"

say "Daftar peserta baru (agar skrip bisa dijalankan berulang)"
STAMP=$(date +%s)
EMAIL="smoke-$STAMP@bumnacademy.id"
REGISTER=$(req -X POST "$API/auth/register" -d "{\"name\":\"Peserta Smoke $STAMP\",\"email\":\"$EMAIL\",\"password\":\"rahasia123\",\"program\":\"Paket Simulasi & Evaluasi\"}")
TOKEN=$(echo "$REGISTER" | jq1 "['accessToken']")
REFERRAL=$(echo "$REGISTER" | jq1 "['user']['referralCode']")
[ -n "$TOKEN" ] && ok "peserta baru terdaftar: $EMAIL (kode referral $REFERRAL)" || bad "pendaftaran gagal"
AUTH="Authorization: Bearer $TOKEN"

say "Perpanjang token (refresh)"
REFRESH_TOKEN=$(echo "$REGISTER" | jq1 "['refreshToken']")
NEW_ACCESS=$(req -X POST "$API/auth/refresh" -d "{\"refreshToken\":\"$REFRESH_TOKEN\"}" | jq1 "['accessToken']")
[ -n "$NEW_ACCESS" ] && ok "access token baru diterbitkan" || bad "refresh gagal"
AUTH="Authorization: Bearer $NEW_ACCESS"

say "Daftar tryout"
COUNT=$(req "$API/tryouts" -H "$AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
check "jumlah paket tryout" "$COUNT" "5"

LOCKED=$(req "$API/tryouts/to-tkd-01" -H "$AUTH" | jq1 "['unlocked']")
check "tryout premium terkunci untuk peserta baru" "$LOCKED" "False"

say "Mulai sesi tryout (to-demo-nasional)"
EXPECTED_Q=$(req "$API/tryouts/to-demo-nasional" -H "$AUTH" | jq1 "['questionCount']")
SESSION=$(req -X POST "$API/tryouts/to-demo-nasional/sessions" -H "$AUTH")
SESSION_ID=$(echo "$SESSION" | jq1 "['id']")
TOTAL_Q=$(echo "$SESSION" | jq1 "['totalQuestions']")
check "total soal sesi sama dengan detail paket" "$TOTAL_Q" "$EXPECTED_Q"
[ -n "$SESSION_ID" ] && ok "sesi dibuat: $SESSION_ID" || bad "gagal membuat sesi"
SUB_LOCKED=$(echo "$SESSION" | jq1 "['subtests'][1]['started']")
check "subtes kedua belum berjalan" "$SUB_LOCKED" "False"

say "Mulai ulang sesi (uji idempotensi)"
SESSION2_ID=$(req -X POST "$API/tryouts/to-demo-nasional/sessions" -H "$AUTH" | jq1 "['id']")
check "sesi aktif tidak ganda" "$SESSION2_ID" "$SESSION_ID"

say "Autosave jawaban"
BODY=$(echo "$SESSION" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
answers = []
for st in d["subtests"]:
    for q in st["questions"]:
        answers.append({"questionId": q["id"], "selectedIndex": 0, "flagged": False})
print(json.dumps({"subtestIndex": len(d["subtests"]) - 1, "answers": answers}))')
SAVED=$(req -X PATCH "$API/tryout-sessions/$SESSION_ID/answers" -H "$AUTH" -d "$BODY" | jq1 "['savedCount']")
check "semua jawaban tersimpan" "$SAVED" "$EXPECTED_Q"

say "Jawaban tidak bisa diubah setelah subtes dikunci"
UNKNOWN=$(req -X PATCH "$API/tryout-sessions/$SESSION_ID/answers" -H "$AUTH" -d '{"answers":[{"questionId":"soal-tidak-ada","selectedIndex":0}]}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "soal di luar sesi ditolak" "$UNKNOWN" "BAD_REQUEST"

say "Kumpulkan sesi"
RESULT=$(req -X POST "$API/tryout-sessions/$SESSION_ID/submit" -H "$AUTH")
ATTEMPT_ID=$(echo "$RESULT" | jq1 "['id']")
[ -n "$ATTEMPT_ID" ] && ok "attempt dibuat: $ATTEMPT_ID" || bad "submit gagal"
REVIEW=$(echo "$RESULT" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["review"]))')
check "jumlah subtes pada pembahasan" "$REVIEW" "2"
SCORE=$(echo "$RESULT" | jq1 "['score']")
[ "$SCORE" -ge 0 ] && [ "$SCORE" -le 100 ] && ok "skor dalam rentang 0-100 ($SCORE)" || bad "skor tidak valid ($SCORE)"

say "Submit ulang (uji idempotensi)"
ATTEMPT2=$(req -X POST "$API/tryout-sessions/$SESSION_ID/submit" -H "$AUTH" | jq1 "['id']")
check "submit berulang mengembalikan attempt sama" "$ATTEMPT2" "$ATTEMPT_ID"

say "Riwayat & pembahasan"
TOTAL_ATTEMPTS=$(req "$API/attempts" -H "$AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["meta"]["total"])')
check "riwayat berisi 1 hasil" "$TOTAL_ATTEMPTS" "1"
QCOUNT=$(req "$API/attempts/$ATTEMPT_ID" -H "$AUTH" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(sum(len(s["questions"]) for s in d["review"]))')
check "soal pada pembahasan" "$QCOUNT" "$EXPECTED_Q"
EXPLANATION=$(req "$API/attempts/$ATTEMPT_ID" -H "$AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["review"][0]["questions"][0]["explanation"]))')
[ "$EXPLANATION" -gt 10 ] && ok "pembahasan soal tersedia ($EXPLANATION karakter)" || bad "pembahasan kosong"

say "Papan peringkat"
ENTRIES=$(req "$API/tryouts/to-demo-nasional/leaderboard?limit=10" -H "$AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]["entries"]))')
[ "$ENTRIES" -ge 1 ] && ok "leaderboard berisi $ENTRIES peserta" || bad "leaderboard kosong"

say "Ringkasan dashboard"
DASH=$(req "$API/dashboard/summary" -H "$AUTH" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["totalAttempts"], d["bestScore"], len(d["scoreTrend"]))')
ok "dashboard (attempts/best/trend) = $DASH"

say "Checkout: quote + pesanan + pembayaran"
QUOTE_TOTAL=$(req -X POST "$API/checkout/quote" -H "$AUTH" -d '{"productSlug":"kelas-intensif","voucherCode":"MULAI20","methodId":"qris"}' | jq1 "['total']")
# 399000 - 20% = 319200
check "quote kelas-intensif + MULAI20" "$QUOTE_TOTAL" "319200"

REFERRAL_TOTAL=$(req -X POST "$API/checkout/quote" -H "$AUTH" -d '{"productSlug":"kelas-intensif","referralCode":"ILHAM10","methodId":"qris"}' | jq1 "['total']")
# 399000 - 10% = 359100
check "quote dengan kode referral ILHAM10" "$REFERRAL_TOTAL" "359100"

FREE_TOTAL=$(req -X POST "$API/checkout/quote" -H "$AUTH" -d '{"productSlug":"tryout-demo"}' | jq1 "['total']")
check "quote produk gratis" "$FREE_TOTAL" "0"

ORDER=$(req -X POST "$API/orders" -H "$AUTH" -d '{"productSlug":"kelas-intensif","voucherCode":"MULAI20","methodId":"qris"}')
ORDER_ID=$(echo "$ORDER" | jq1 "['id']")
INVOICE=$(echo "$ORDER" | jq1 "['invoiceNo']")
[ -n "$ORDER_ID" ] && ok "pesanan dibuat: $INVOICE" || bad "gagal membuat pesanan"

PAID=$(req -X POST "$API/orders/$ORDER_ID/pay" -H "$AUTH" | jq1 "['order']['status']")
check "status setelah bayar" "$PAID" "paid"

ENROLLED=$(req "$API/enrollments" -H "$AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
check "akses produk setelah pembelian" "$ENROLLED" "1"

UNLOCKED=$(req "$API/tryouts/to-akhlak-01" -H "$AUTH" | jq1 "['unlocked']")
check "tryout AKHLAK terbuka setelah membeli kelas-intensif" "$UNLOCKED" "True"

say "Webhook Midtrans (idempoten)"
WEBHOOK=$(req -X POST "$API/webhooks/midtrans" -d "{\"order_id\":\"$INVOICE\",\"transaction_status\":\"settlement\",\"transaction_id\":\"smoke-1\"}" | jq1 "['status']")
check "webhook diproses" "$WEBHOOK" "settlement"

say "Admin"
ADMIN_TOKEN=$(req -X POST "$API/auth/login" -d '{"email":"admin@bumnacademy.id","password":"admin123"}' | jq1 "['accessToken']")
ADMIN_AUTH="Authorization: Bearer $ADMIN_TOKEN"
ROLE=$(req "$API/auth/me" -H "$ADMIN_AUTH" | jq1 "['role']")
check "role admin" "$ROLE" "admin"

QUESTIONS=$(req "$API/admin/questions?perPage=100" -H "$ADMIN_AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["meta"]["total"])')
[ "$QUESTIONS" -ge 45 ] && ok "bank soal berisi $QUESTIONS soal" || bad "jumlah soal kurang dari 45 ($QUESTIONS)"

DENIED=$(req "$API/admin/analytics" -H "$AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "peserta ditolak dari endpoint admin" "$DENIED" "FORBIDDEN"

ANALYTICS=$(req "$API/admin/analytics" -H "$ADMIN_AUTH" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["totalPeserta"], d["questionsTotal"], d["tryoutsTotal"])')
ok "analitik (peserta/soal/tryout) = $ANALYTICS"

say "Import soal (CSV)"
CSV="$(mktemp -t bumn-questions).csv"
cat > "$CSV" <<'EOF'
bankKey,text,optionA,optionB,optionC,optionD,answer,explanation,difficulty
numerik,"1 + 1 = ?","1","2","3","4",B,"Penjumlahan dasar: 1 + 1 = 2.",mudah
numerik,"10 / 2 = ?","4","5","6","7",B,"Pembagian dasar: 10 / 2 = 5.",mudah
EOF
IMPORTED=$(curl -s -m 20 -X POST "$API/admin/questions/import" -H "$ADMIN_AUTH" -F "file=@$CSV" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["imported"], d["failed"])')
check "import CSV (imported failed)" "$IMPORTED" "2 0"
rm -f "$CSV"

say "Broadcast WhatsApp"
RECIPIENTS=$(req -X POST "$API/admin/broadcast" -H "$ADMIN_AUTH" -d '{"audience":"all","message":"Selamat datang di BUMN Academy!"}' | jq1 "['recipients']")
[ "$RECIPIENTS" -ge 1 ] && ok "broadcast tercatat untuk $RECIPIENTS penerima" || bad "broadcast gagal"

say "Validasi input"
BAD=$(req -X POST "$API/auth/register" -d '{"name":"x","email":"bukan-email","password":"123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "validasi menolak payload tidak valid" "$BAD" "VALIDATION_ERROR"

UNAUTH=$(req "$API/orders" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "endpoint peserta butuh token" "$UNAUTH" "UNAUTHORIZED"

printf '\n\033[1mLulus: %d, Gagal: %d\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
