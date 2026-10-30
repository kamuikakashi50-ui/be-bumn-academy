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
ORDER_TOTAL=$(echo "$ORDER" | jq1 "['total']")
[ -n "$ORDER_ID" ] && ok "pesanan dibuat: $INVOICE" || bad "gagal membuat pesanan"

PAID=$(req -X POST "$API/orders/$ORDER_ID/pay" -H "$AUTH" | jq1 "['order']['status']")
check "status setelah bayar" "$PAID" "paid"

ENROLLED=$(req "$API/enrollments" -H "$AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
check "akses produk setelah pembelian" "$ENROLLED" "1"

UNLOCKED=$(req "$API/tryouts/to-akhlak-01" -H "$AUTH" | jq1 "['unlocked']")
check "tryout AKHLAK terbuka setelah membeli kelas-intensif" "$UNLOCKED" "True"

say "Webhook Midtrans (bertanda tangan, idempoten)"
# Backend memverifikasi signature notifikasi: SHA512(order_id + status_code +
# gross_amount + Server Key). Tanpa kredensial Midtrans asli, backend memakai
# kunci simulator lokal (internal/pkg/midtrans.SandboxDemoServerKey), jadi skrip
# ini menghitung tanda tangan yang sama agar jalur webhook tetap teruji.
GROSS_AMOUNT="$ORDER_TOTAL.00"
SIGNATURE=$(python3 -c 'import hashlib,sys;print(hashlib.sha512((sys.argv[1]+sys.argv[2]+sys.argv[3]+"SB-Mid-server-SANDBOX-SIMULATOR").encode()).hexdigest())' "$INVOICE" "200" "$GROSS_AMOUNT")
WEBHOOK=$(req -X POST "$API/webhooks/midtrans" -d "{\"order_id\":\"$INVOICE\",\"status_code\":\"200\",\"gross_amount\":\"$GROSS_AMOUNT\",\"transaction_status\":\"settlement\",\"transaction_id\":\"smoke-1\",\"signature_key\":\"$SIGNATURE\"}" | jq1 "['status']")
check "webhook diproses" "$WEBHOOK" "settlement"

BAD_SIGNATURE=$(req -X POST "$API/webhooks/midtrans" -d "{\"order_id\":\"$INVOICE\",\"status_code\":\"200\",\"gross_amount\":\"$GROSS_AMOUNT\",\"transaction_status\":\"settlement\",\"transaction_id\":\"smoke-2\",\"signature_key\":\"tanda-tangan-palsu\"}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "webhook tanpa tanda tangan sah ditolak" "$BAD_SIGNATURE" "UNAUTHORIZED"

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

say "Jalur belajar & kuis (Fase 1)"
# Peserta demo punya produk kelas-intensif; akun smoke yang baru mendaftar tidak,
# sehingga bagian ini memakai token peserta demo (diambil ulang di sini).
PT=$(req -X POST "$API/auth/login" -d '{"email":"peserta@bumnacademy.id","password":"peserta123"}' | jq1 "['accessToken']")
PT_AUTH="Authorization: Bearer $PT"

PATHS=$(req "$API/learning/paths" -H "$PT_AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
[ "$PATHS" -ge 1 ] && ok "peserta melihat $PATHS jalur belajar" || bad "daftar jalur belajar kosong"

DETAIL=$(req "$API/learning/paths/jalur-tkd-dasar" -H "$PT_AUTH" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["stepCount"], d["requiredStepCount"], d["steps"][0]["moduleId"])')
check "detail jalur (langkah/wajib/modul pertama)" "$DETAIL" "3 2 mod-intensif-numerik"

ADMIN_PATHS=$(req "$API/admin/learning/paths" -H "$ADMIN_AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
[ "$ADMIN_PATHS" -ge 1 ] && ok "panel admin menampilkan $ADMIN_PATHS jalur belajar" || bad "panel jalur belajar kosong"

# Susun ulang langkah dengan susunan yang sama: idempoten, sekaligus menguji jalur
# tulis yang dulu gagal karena asosiasi GORM (constraint unik path_id+module_id).
STEPS=$(req -X POST "$API/admin/learning/paths/path-tkd-dasar/steps" -H "$ADMIN_AUTH" \
  -d '{"steps":[{"moduleId":"mod-intensif-numerik"},{"moduleId":"mod-intensif-verbal"},{"moduleId":"mod-intensif-analitis","required":false}]}' \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["stepCount"], d["requiredStepCount"])')
check "susun ulang langkah jalur (total/wajib)" "$STEPS" "3 2"

BAD_MODULE=$(req -X POST "$API/admin/learning/paths" -H "$ADMIN_AUTH" \
  -d '{"slug":"jalur-uji-smoke","title":"Jalur Uji Smoke","productSlug":"kelas-intensif","steps":[{"moduleId":"mod-latihan-numerik"}]}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "modul dari produk lain ditolak" "$BAD_MODULE" "VALIDATION_ERROR"

DUP_SLUG=$(req -X POST "$API/admin/learning/paths" -H "$ADMIN_AUTH" \
  -d '{"slug":"jalur-tkd-dasar","title":"Jalur Duplikat","productSlug":"kelas-intensif"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "slug jalur duplikat ditolak" "$DUP_SLUG" "VALIDATION_ERROR"

# Kuis: jalankan pada pelajaran uji agar jatah percobaan kuis demo tidak habis.
# Kuis aktif lama pada pelajaran uji dimatikan lebih dulu sehingga skrip aman
# dijalankan berulang (percobaan peserta dihitung per kuis).
for OLD in $(req "$API/admin/classroom/quizzes?lessonId=les-numerik-perbandingan" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;print(" ".join(q["id"] for q in json.load(sys.stdin)["data"] if q["active"]))'); do
  req -o /dev/null -X DELETE "$API/admin/classroom/quizzes/$OLD" -H "$ADMIN_AUTH"
done

QUIZ=$(req -X POST "$API/admin/classroom/quizzes" -H "$ADMIN_AUTH" \
  -d '{"lessonId":"les-numerik-perbandingan","title":"Kuis Smoke Perbandingan","passScore":60,"maxAttempts":0,"timeLimit":5,"questionIds":["num-04","num-05"]}' \
  | jq1 "['id']")
[ -n "$QUIZ" ] && ok "kuis uji dibuat ($QUIZ)" || bad "pembuatan kuis gagal"

# Soal tanpa izin aktif tidak boleh masuk kuis (penegakan Fase 0).
UNLICENSED=$(req -X POST "$API/admin/classroom/quizzes" -H "$ADMIN_AUTH" \
  -d '{"lessonId":"les-numerik-perbandingan","title":"Kuis Soal Tak Dikenal","questionIds":["soal-tidak-ada"]}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "soal tidak dikenal ditolak dari kuis" "$UNLICENSED" "VALIDATION_ERROR"

LICENSED=$(req "$API/admin/licenses?questionId=num-04&status=aktif" -H "$ADMIN_AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
[ "$LICENSED" -ge 1 ] && ok "soal kuis punya izin aktif ($LICENSED izin)" || bad "izin soal kuis tidak ditemukan"

PESERTA_QUIZ=$(req "$API/classroom/lessons/les-numerik-perbandingan/quiz" -H "$PT_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["title"], len(d["questions"]), d["attemptsLeft"])')
check "peserta membuka kuis (judul/soal/sisa percobaan)" "$PESERTA_QUIZ" "Kuis Smoke Perbandingan 2 -1"

RAW=$(req "$API/classroom/lessons/les-numerik-perbandingan/quiz" -H "$PT_AUTH")
LEAK=$(printf '%s' "$RAW" | grep -c 'answerIndex\|explanation' || true)
check "kunci jawaban tidak terkirim sebelum dikumpulkan" "$LEAK" "0"

# num-04 kunci 2, num-05 kunci 1: satu benar satu salah -> 50 (ambang 60).
SCORE1=$(req -X POST "$API/classroom/lessons/les-numerik-perbandingan/quiz" -H "$PT_AUTH" \
  -d '{"answers":[{"questionId":"num-04","selectedIndex":2},{"questionId":"num-05","selectedIndex":0}]}' \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["score"], d["correct"], d["total"], str(d["passed"]).lower(), len(d["review"]))')
check "penilaian kuis (skor/benar/total/lulus/pembahasan)" "$SCORE1" "50 1 2 false 2"

SCORE2=$(req -X POST "$API/classroom/lessons/les-numerik-perbandingan/quiz" -H "$PT_AUTH" \
  -d '{"answers":[{"questionId":"num-04","selectedIndex":2},{"questionId":"num-05","selectedIndex":1}]}' \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["score"], str(d["passed"]).lower())')
check "kuis kedua lulus" "$SCORE2" "100 true"

ATTEMPTS=$(req "$API/admin/classroom/quizzes/$QUIZ/attempts" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d), d[0]["initials"], d[0]["score"])')
check "rekap percobaan kuis admin (jumlah/inisial/skor)" "$ATTEMPTS" "2 PD 100"

DENIED_QUIZ=$(req "$API/classroom/lessons/les-numerik-perbandingan/quiz" -H "$ADMIN_AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "materi kelas khusus ditolak tanpa kepemilikan produk" "$DENIED_QUIZ" "FORBIDDEN"

# Kuis uji dimatikan setelah dipakai: pelajaran ini berada di dalam jalur belajar
# contoh, dan kuis yang masih aktif akan menjadi syarat tambahan sertifikat jalur
# tersebut (aturan §4.3: semua kuis wajib lulus).
req -o /dev/null -X DELETE "$API/admin/classroom/quizzes/$QUIZ" -H "$ADMIN_AUTH"
STILL_ACTIVE=$(req "$API/admin/classroom/quizzes?lessonId=les-numerik-perbandingan" -H "$ADMIN_AUTH" | python3 -c 'import sys,json;print(len([q for q in json.load(sys.stdin)["data"] if q["active"]]))')
check "kuis uji dinonaktifkan setelah dipakai" "$STILL_ACTIVE" "0"

say "Sertifikat (Fase 1)"
# Sertifikat demo akun peserta: bila belum ada, selesaikan langkah wajib jalur
# contoh (modul numerik + verbal) lalu minta penerbitannya. Bila sudah ada
# (skrip dijalankan ulang), langkah ini dilewati agar tidak menghabiskan jatah
# percobaan kuis.
CERT=$(req "$API/certificates" -H "$PT_AUTH" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d[0]["number"] if d else "")')
if [ -z "$CERT" ]; then
  for LESSON in les-numerik-deret les-numerik-perbandingan les-ebook-numerik les-verbal-analogi les-ebook-verbal; do
    req -o /dev/null -X POST "$API/classroom/lessons/$LESSON/progress" -H "$PT_AUTH" -d '{"completed":true}'
  done

  # Kunci jawaban diambil dari API admin (endpoint peserta memang tidak
  # mengirimnya). Susunan soal diambil dari kuis itu sendiri, lalu dicocokkan
  # dengan bank soal demo — tanpa filter tryout, halaman pertama bank berisi
  # salinan soal milik tryout lain.
  QIDS=$(req "$API/admin/classroom/quizzes?lessonId=les-numerik-deret" -H "$ADMIN_AUTH" \
    | python3 -c 'import sys,json;d=[q for q in json.load(sys.stdin)["data"] if q["active"]];print(" ".join(d[0]["questionIds"]) if d else "")')
  ANSWERS=$(req "$API/admin/questions?tryoutId=to-demo-nasional&bankKey=numerik&perPage=100" -H "$ADMIN_AUTH" \
    | QIDS="$QIDS" python3 -c '
import os, sys, json
ids = set(os.environ["QIDS"].split())
by_id = {q["id"]: q for q in json.load(sys.stdin)["data"]}
print(json.dumps([{"questionId": i, "selectedIndex": by_id[i]["answerIndex"]} for i in ids if i in by_id]))')

  LEFT=$(req "$API/classroom/lessons/les-numerik-deret/quiz" -H "$PT_AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["attemptsLeft"])')
  if [ "$LEFT" != "0" ] && [ "$ANSWERS" != "[]" ]; then
    req -o /dev/null -X POST "$API/classroom/lessons/les-numerik-deret/quiz" -H "$PT_AUTH" -d "{\"answers\":$ANSWERS}"
  fi

  CERT=$(req -X POST "$API/learning/paths/jalur-tkd-dasar/certificate" -H "$PT_AUTH" | jq1 "['number']")
fi

CERT_FORMAT=$(printf '%s' "$CERT" | grep -cE '^BA-[0-9]{4}-[0-9]{6}$' || true)
check "nomor sertifikat berformat BA-<tahun>-<6 digit>" "$CERT_FORMAT" "1"

CERT_ID=$(req "$API/certificates" -H "$PT_AUTH" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d[0]["id"] if d else "")')
CERT_LIST=$(req "$API/certificates" -H "$PT_AUTH" | python3 -c "import sys,json;d=json.load(sys.stdin)['data'];print(d[0]['number'], d[0]['status'], d[0]['pathTitle'])")
check "daftar sertifikat peserta" "$CERT_LIST" "$CERT sah Jalur TKD Dasar"

SAME=$(req -X POST "$API/learning/paths/jalur-tkd-dasar/certificate" -H "$PT_AUTH" | jq1 "['number']")
check "penerbitan ulang idempoten (nomor sama)" "$SAME" "$CERT"

PDF_HEAD=$(req -D - -o /tmp/ba-smoke-cert.pdf "$API/certificates/$CERT_ID/pdf" -H "$PT_AUTH")
check "unduh PDF sertifikat" "$(printf '%s' "$PDF_HEAD" | grep -ci 'content-type: application/pdf')" "1"
check "berkas PDF sah" "$(head -c 5 /tmp/ba-smoke-cert.pdf)" "%PDF-"
rm -f /tmp/ba-smoke-cert.pdf

VERIFY=$(req "$API/verifikasi/$CERT" | python3 -c "import sys,json;d=json.load(sys.stdin)['data'];print(str(d['valid']).lower(), d['status'], d['participant'], d['pathTitle'])")
check "verifikasi publik sertifikat" "$VERIFY" "true sah Peserta Demo Jalur TKD Dasar"

# Peserta tanpa progres (akun smoke yang membeli kelas-intensif) belum layak.
NOT_ELIGIBLE=$(req -X POST "$API/learning/paths/jalur-tkd-dasar/certificate" -H "$AUTH" | python3 -c 'import sys,json;e=json.load(sys.stdin)["error"];print(e["code"], len(e.get("details",{}).get("missingSteps",[])) > 0)')
check "sertifikat ditolak bila syarat belum lengkap" "$NOT_ELIGIBLE" "VALIDATION_ERROR True"

ADMIN_CERT=$(req "$API/admin/certificates?search=$CERT" -H "$ADMIN_AUTH" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d), d[0]["number"], d[0]["status"])')
check "rekap sertifikat admin" "$ADMIN_CERT" "1 $CERT sah"

req -o /dev/null -X DELETE "$API/admin/certificates/$CERT_ID" -H "$ADMIN_AUTH"
REVOKED=$(req "$API/verifikasi/$CERT" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(str(d["valid"]).lower(), d["status"])')
check "sertifikat dicabut tidak lagi valid" "$REVOKED" "false dicabut"

req -o /dev/null -X POST "$API/admin/certificates/$CERT_ID/restore" -H "$ADMIN_AUTH"
RESTORED=$(req "$API/verifikasi/$CERT" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(str(d["valid"]).lower(), d["status"])')
check "sertifikat dipulihkan kembali sah" "$RESTORED" "true sah"

say "Validasi input"
BAD=$(req -X POST "$API/auth/register" -d '{"name":"x","email":"bukan-email","password":"123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "validasi menolak payload tidak valid" "$BAD" "VALIDATION_ERROR"

UNAUTH=$(req "$API/orders" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "endpoint peserta butuh token" "$UNAUTH" "UNAUTHORIZED"

printf '\n\033[1mLulus: %d, Gagal: %d\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
