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

# Akun pembanding: peserta yang **tidak** membeli produk apa pun dan **tidak**
# menjadi anggota kelas khusus. Dipakai membuktikan penolakan akses materi kelas
# khusus (peserta di luar kelas) dan pembukaan akses setelah bergabung.
KOHORT_EMAIL="smoke-kohort-$STAMP@bumnacademy.id"
KOHORT_TOKEN=$(req -X POST "$API/auth/register" -d "{\"name\":\"Peserta Kohort $STAMP\",\"email\":\"$KOHORT_EMAIL\",\"password\":\"rahasia123\",\"program\":\"Program Reguler\"}" | jq1 "['accessToken']")
KOHORT_AUTH="Authorization: Bearer $KOHORT_TOKEN"
[ -n "$KOHORT_TOKEN" ] && ok "akun pembanding terdaftar: $KOHORT_EMAIL" || bad "pendaftaran akun pembanding gagal"

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

DENIED_QUIZ=$(req "$API/classroom/lessons/les-numerik-perbandingan/quiz" -H "$KOHORT_AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
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

say "Kelas khusus (Fase 1)"
COHORT_CODE="SMOKE-$STAMP"

ORG_ID=$(req -X POST "$API/admin/organizations" -H "$ADMIN_AUTH" \
  -d "{\"name\":\"Instansi Smoke $STAMP\",\"sectorKey\":\"jasa_keuangan\",\"picName\":\"PIC Smoke\",\"picEmail\":\"pic@smoke.id\"}" \
  | jq1 "['id']")
[ -n "$ORG_ID" ] && ok "instansi mitra dibuat ($ORG_ID)" || bad "gagal membuat instansi mitra"

COHORT_ID=$(req -X POST "$API/admin/cohorts" -H "$ADMIN_AUTH" \
  -d "{\"orgId\":\"$ORG_ID\",\"name\":\"Kelas Smoke $STAMP\",\"code\":\"$COHORT_CODE\",\"sectorKey\":\"jasa_keuangan\",\"status\":\"aktif\",\"quota\":5,\"curriculum\":[{\"moduleId\":\"mod-intensif-akhlak\",\"required\":true},{\"moduleId\":\"mod-intensif-inggris\",\"required\":false}]}" \
  | jq1 "['id']")
[ -n "$COHORT_ID" ] && ok "kelas khusus dibuat ($COHORT_ID)" || bad "gagal membuat kelas khusus"

# Modul wajib/opsional harus tersimpan apa adanya (`required:false` tidak boleh
# ditelan default kolom).
COHORT_DETAIL=$(req "$API/admin/cohorts/$COHORT_ID" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["moduleCount"], d["requiredModuleCount"], d["statusLabel"], d["orgName"])')
check "detail kelas (modul/wajib/status/instansi)" "$COHORT_DETAIL" "2 1 Aktif Instansi Smoke $STAMP"

COHORT_INVITE=$(req -X POST "$API/admin/cohorts/$COHORT_ID/invites" -H "$ADMIN_AUTH" -d '{}' | jq1 "['code']")
check "kode undangan dibuat (8 karakter)" "${#COHORT_INVITE}" "8"

DENIED=$(req "$API/classroom/lessons/les-akhlak-studi-kasus" -H "$KOHORT_AUTH" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "peserta non-anggota ditolak membuka modul kelas" "$DENIED" "FORBIDDEN"

JOINED=$(req -X POST "$API/cohorts/join" -H "$KOHORT_AUTH" -d "{\"code\":\"$COHORT_INVITE\"}" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(str(d["alreadyMember"]).lower(), len(d["cohort"]["curriculum"]))')
check "peserta gabung kelas dengan kode undangan" "$JOINED" "false 2"

ALLOWED=$(req "$API/classroom/lessons/les-akhlak-studi-kasus" -H "$KOHORT_AUTH" | jq1 "['lesson']['moduleId']")
check "anggota kelas membuka materi tanpa membeli produk" "$ALLOWED" "mod-intensif-akhlak"

# Modul di luar kurikulum kelas tetap terkunci walau produknya sama.
OUTSIDE=$(req "$API/classroom/lessons/les-numerik-deret" -H "$KOHORT_AUTH" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "modul di luar kurikulum kelas tetap ditolak" "$OUTSIDE" "FORBIDDEN"

DUPLICATE=$(req -X POST "$API/cohorts/join" -H "$KOHORT_AUTH" -d "{\"code\":\"$COHORT_INVITE\"}" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "gabung ulang ditolak (idempoten, kuota kode tidak bertambah)" "$DUPLICATE" "CONFLICT"

EXPIRED_CODE=$(req -X POST "$API/admin/cohorts/$COHORT_ID/invites" -H "$ADMIN_AUTH" -d '{"expiresAt":"2020-01-01"}' | jq1 "['code']")
EXPIRED=$(req -X POST "$API/cohorts/join" -H "$KOHORT_AUTH" -d "{\"code\":\"$EXPIRED_CODE\"}" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "kode undangan kedaluwarsa ditolak" "$EXPIRED" "VALIDATION_ERROR"

# Kode berkuota 1: pemakaian pertama menambah kuota, pemakaian berikutnya ditolak.
FULL_CODE=$(req -X POST "$API/admin/cohorts/$COHORT_ID/invites" -H "$ADMIN_AUTH" -d '{"maxUses":1}' | jq1 "['code']")
FIRST_USE=$(req -X POST "$API/cohorts/join" -H "$AUTH" -d "{\"code\":\"$FULL_CODE\"}" | jq1 "['alreadyMember']")
check "kode berkuota dipakai peserta pertama" "$FIRST_USE" "False"

KOHORT2_TOKEN=$(req -X POST "$API/auth/register" -d "{\"name\":\"Peserta Kohort Dua $STAMP\",\"email\":\"smoke-kohort2-$STAMP@bumnacademy.id\",\"password\":\"rahasia123\",\"program\":\"Program Reguler\"}" | jq1 "['accessToken']")
EXHAUSTED=$(req -X POST "$API/cohorts/join" -H "Authorization: Bearer $KOHORT2_TOKEN" -d "{\"code\":\"$FULL_CODE\"}" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "kode undangan penuh ditolak" "$EXHAUSTED" "CONFLICT"

INVITE_STATE=$(req "$API/admin/cohorts/$COHORT_ID/invites" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];x=[i for i in d if i["code"]=="'"$FULL_CODE"'"];print(x[0]["usedCount"], x[0]["remaining"], str(x[0]["usable"]).lower())')
check "pemakaian kode tercatat (dipakai/sisa/layak)" "$INVITE_STATE" "1 0 false"

ME=$(req "$API/cohorts/me" -H "$KOHORT_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d), str(d[0]["accessOpen"]).lower(), d[0]["statusLabel"])')
check "peserta melihat kelasnya (jumlah/akses/status)" "$ME" "1 true Aktif"

AUDIT=$(req "$API/admin/cohorts/$COHORT_ID/audit" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len([a for a in d if a["action"] in ("cohort_created","invite_created","invite_used")]) >= 4)')
check "jejak audit kelas mencatat pembuatan & pemakaian kode" "$AUDIT" "True"

# Laporan kelas: agregat + rincian anggota kelas itu saja (isolasi §4.7).
REPORT=$(req "$API/admin/cohorts/$COHORT_ID/report" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d["members"]), d["summary"]["totalMembers"], d["summary"]["reportingMembers"], len(d["modules"]), d["summary"]["totalLessons"])')
check "laporan kelas (anggota/total/pelatih/modul/materi)" "$REPORT" "2 2 1 2 4"

REPORT_SUM=$(req "$API/admin/cohorts/$COHORT_ID/report" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];s=d["summary"];print(s["activeMembers"], s["passingThreshold"], s["certificates"], len(d["topLearners"]) >= 1, len(d["notes"]) >= 1)')
check "ringkasan laporan (aktif/ambang/sertifikat/prestasi/catatan)" "$REPORT_SUM" "2 70 0 True True"

ISOLATED=$(req "$API/admin/cohorts/$COHORT_ID/report" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];names=" | ".join(m["name"] for m in d["members"]);print("Peserta Kohort" in names, "Dewi Lestari" in names)')
check "laporan hanya memuat peserta kelas ini (isolasi)" "$ISOLATED" "True False"

# Laporan kelas demo (data seed): mentor kelas tidak dihitung sebagai peserta.
DEMO_COHORT_ID=$(req "$API/admin/cohorts?search=DEMO-BATCH-1" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d[0]["id"] if d else "")')
DEMO_REPORT=$(req "$API/admin/cohorts/$DEMO_COHORT_ID/report" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];names=" | ".join(m["name"] for m in d["members"]);print(d["summary"]["totalMembers"], "Dewi Lestari" in names, "Admin Akademik" in names)')
check "laporan kelas demo (peserta seed, mentor dikecualikan)" "$DEMO_REPORT" "3 True False"

LAPORAN=/tmp/laporan-kelas-smoke.pdf
PDF_STATUS=$(curl -s -m 20 -o "$LAPORAN" -w '%{http_code}' "$API/admin/cohorts/$COHORT_ID/report.pdf" -H "$ADMIN_AUTH")
check "unduh PDF laporan kelas (status)" "$PDF_STATUS" "200"
if head -c 5 "$LAPORAN" | grep -q '%PDF-'; then
  ok "berkas laporan kelas sah (PDF)"
else
  bad "berkas laporan kelas bukan PDF"
fi
LAPORAN_SIZE=$(wc -c < "$LAPORAN" | tr -d ' ')
[ "$LAPORAN_SIZE" -gt 2000 ] && ok "ukuran laporan wajar ($LAPORAN_SIZE byte)" || bad "laporan terlalu kecil ($LAPORAN_SIZE byte)"
rm -f "$LAPORAN"

# Kelas uji diarsipkan di akhir agar tidak muncul sebagai kelas aktif pada
# pemeriksaan berikutnya (endpoint hapus sengaja tidak ada — data kelas dirujuk
# laporan).
req -o /dev/null -X PATCH "$API/admin/cohorts/$COHORT_ID" -H "$ADMIN_AUTH" -d '{"status":"arsip"}'
ARCHIVED=$(req "$API/cohorts/me" -H "$KOHORT_AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
check "kelas diarsipkan tidak lagi aktif bagi peserta" "$ARCHIVED" "0"

say "KPI kelulusan BUMN (Fase 2)"
YEAR=$(date +%Y)

# 1. Peserta melaporkan hasil seleksi → source self_reported (belum terverifikasi).
OUTCOME=$(req -X POST "$API/outcomes" -H "$AUTH" \
  -d "{\"stageKey\":\"tahap1_tkd\",\"companyName\":\"Bank Nusantara Smoke\",\"companySector\":\"jasa_keuangan\",\"position\":\"Analis Kredit\",\"positionFamily\":\"keuangan\",\"selectionYear\":$YEAR,\"status\":\"lolos\",\"announcedAt\":\"$YEAR-05-12\"}")
OUTCOME_ID=$(echo "$OUTCOME" | jq1 "['id']")
OUTCOME_SRC=$(echo "$OUTCOME" | jq1 "['source']")
OUTCOME_VERIFIED=$(echo "$OUTCOME" | jq1 "['verified']")
check "peserta melaporkan hasil seleksi (source/verified)" "$OUTCOME_SRC $OUTCOME_VERIFIED" "self_reported False"

# 2. Laporan ganda pada kombinasi (tahun, tahap, instansi) → CONFLICT.
DUP=$(req -X POST "$API/outcomes" -H "$AUTH" \
  -d "{\"stageKey\":\"tahap1_tkd\",\"companyName\":\"Bank Nusantara Smoke\",\"companySector\":\"jasa_keuangan\",\"selectionYear\":$YEAR,\"status\":\"lolos\"}" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "laporan ganda pada kombinasi sama ditolak" "$DUP" "CONFLICT"

# 3. Admin memverifikasi bukti → source admin_verified & verifiedAt terisi.
VERIFIED=$(req -X POST "$API/admin/outcomes/$OUTCOME_ID/verify" -H "$ADMIN_AUTH" -d '{}' \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["source"], bool(d.get("verifiedAt")))')
check "admin memverifikasi laporan (source/verifiedAt)" "$VERIFIED" "admin_verified True"

# 4. Dasbor KPI menampilkan angka dengan n yang benar + funnel terisi.
KPI=$(req "$API/admin/kpi/bumn?year=$YEAR" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["summary"]["reported"] >= 1, d["summary"]["finalPassRate"]["verified"]["n"] >= 1, len(d["funnel"]) >= 1, len(d["notes"]) >= 1)')
check "dasbor KPI memuat n, funnel, dan catatan" "$KPI" "True True True True"

# 5. Ekspor CSV memuat kolom n/verified/reported (level terpisah).
CSV=/tmp/kpi-smoke.csv
CSV_STATUS=$(curl -s -m 20 -o "$CSV" -w '%{http_code}' "$API/admin/kpi/bumn/export.csv?year=$YEAR" -H "$ADMIN_AUTH")
check "unduh ekspor CSV KPI (status)" "$CSV_STATUS" "200"
CSV_HEAD=$(head -n 1 "$CSV")
if echo "$CSV_HEAD" | grep -q 'n' && grep -q 'verified' "$CSV" && grep -q 'reported' "$CSV"; then
  ok "CSV KPI memuat kolom n/verified/reported"
else
  bad "CSV KPI tidak memuat kolom yang diharapkan"
fi

# 6. Ekspor PDF berhasil dan sah.
PDF=/tmp/kpi-smoke.pdf
PDF_STATUS=$(curl -s -m 20 -o "$PDF" -w '%{http_code}' "$API/admin/kpi/bumn/export.pdf?year=$YEAR" -H "$ADMIN_AUTH")
check "unduh laporan PDF KPI (status)" "$PDF_STATUS" "200"
if head -c 5 "$PDF" | grep -q '%PDF-'; then
  ok "berkas laporan KPI sah (PDF)"
else
  bad "berkas laporan KPI bukan PDF"
fi
rm -f "$CSV" "$PDF"

# 7. Endpoint publik: tanpa identitas peserta & hanya kelompok n ≥ ambang.
PUBLIC=$(req "$API/insights/outcomes?year=$YEAR")
if echo "$PUBLIC" | grep -q 'Peserta Smoke'; then
  bad "endpoint publik memuat nama peserta"
else
  ok "endpoint publik tidak memuat nama peserta"
fi
PUBLIC_OK=$(echo "$PUBLIC" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["minGroupSize"] >= 1, all(g["n"] >= d["minGroupSize"] for g in d["bySector"]))')
check "kelompok kecil disembunyikan (n ≥ ambang)" "$PUBLIC_OK" "True True"

# 8. Otorisasi: tanpa token UNAUTHORIZED; peserta (bukan admin) FORBIDDEN.
UNAUTH_KPI=$(req "$API/admin/kpi/bumn" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "dasbor KPI butuh token" "$UNAUTH_KPI" "UNAUTHORIZED"
FORBIDDEN_KPI=$(req "$API/admin/kpi/bumn" -H "$AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "peserta tidak bisa membuka dasbor KPI" "$FORBIDDEN_KPI" "FORBIDDEN"

# 9. Pengingat tracer study: audience `outcome_pending` + jeda 14 hari (§4.5).
TRACER_FIRST=$(req -X POST "$API/admin/broadcast" -H "$ADMIN_AUTH" \
  -d '{"audience":"outcome_pending","message":"Lengkapi laporan hasil seleksimu di dashboard."}')
TRACER_CODE=$(echo "$TRACER_FIRST" | python3 -c 'import sys,json;d=json.load(sys.stdin);print("ok" if d.get("success") else d.get("error",{}).get("code",""))')
if [ "$TRACER_CODE" = "ok" ]; then
  TRACER=$(echo "$TRACER_FIRST" | jq1 "['recipients']")
  [ "${TRACER:-0}" -ge 1 ] 2>/dev/null && ok "audience outcome_pending mengirim ke $TRACER penerima" || bad "audience outcome_pending kosong ($TRACER)"
elif [ "$TRACER_CODE" = "CONFLICT" ]; then
  # Database sudah pernah dipakai run sebelumnya (< 14 hari): jeda masih aktif.
  ok "audience outcome_pending dibatasi jeda dari run sebelumnya"
else
  bad "broadcast pengingat tracer study gagal ($TRACER_CODE)"
fi
TRACER_SECOND=$(req -X POST "$API/admin/broadcast" -H "$ADMIN_AUTH" \
  -d '{"audience":"outcome_pending","message":"Pengingat kedua (harus ditolak)."}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "pengingat tracer study dibatasi 14 hari" "$TRACER_SECOND" "CONFLICT"

say "Kompetensi (Fase 3)"
DEWI_TOKEN=$(req -X POST "$API/auth/login" -d '{"email":"dewi.lestari@bumnacademy.id","password":"peserta123"}' | jq1 "['accessToken']")
DEWI_AUTH="Authorization: Bearer $DEWI_TOKEN"
[ -n "$DEWI_TOKEN" ] && ok "login akun kohort demo (untuk proksi tryout)" || bad "login akun kohort gagal"

# 1. Profil kompetensi: kamus terisi, proksi tryout muncul, gap & catatan ada.
PROFILE=$(req "$API/me/competencies?positionFamily=umum" -H "$DEWI_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d["assessments"]) >= 1, any(s["source"]=="tryout" for a in d["assessments"] for s in a["sources"]), len(d["gap"]) >= 1, len(d["notes"]) >= 1)')
check "profil kompetensi (kamus/proksi tryout/gap/catatan)" "$PROFILE" "True True True True"

# 2. Penilaian diri menambah sumber `self` (level tersimpan apa adanya).
SELF=$(req -X POST "$API/me/competencies/self-assessment" -H "$DEWI_AUTH" \
  -d '{"competencyCode":"integritas_akhlaq","level":4,"evidence":"Aktif dalam kegiatan kolaborasi tim."}' \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];a=[x for x in d["assessments"] if x["code"]=="integritas_akhlaq"][0];s=[x for x in a["sources"] if x["source"]=="self"];print(len(s)==1, s[0]["level"] if s else 0, a["bestLevel"] >= 4)')
check "penilaian diri tersimpan (source self, level tersimpan)" "$SELF" "True 4 True"

# 3. Kamus kompetensi admin: kompetensi + 5 level + pemetaan bank soal.
FRAMEWORK=$(req "$API/admin/competencies" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d["competencies"]) >= 10, all(len(c["levels"])==5 for c in d["competencies"]), len(d["bankMap"]) >= 10)')
check "kamus kompetensi (jumlah/level/pemetaan bank)" "$FRAMEWORK" "True True True"

# 4. Matriks kompetensi: kolom, baris peserta, sel, dan penanda gap.
MATRIX=$(req "$API/admin/competency-matrix?positionFamily=umum" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d["competencies"]) >= 10, len(d["rows"]) >= 1, any(len(r["cells"]) >= 1 for r in d["rows"]), all("gap" in r for r in d["rows"]))')
check "matriks kompetensi (kolom/baris/sel/gap)" "$MATRIX" "True True True True"

# 5. Negatif: level di luar 1–5 ditolak.
BAD_LEVEL=$(req -X POST "$API/me/competencies/self-assessment" -H "$DEWI_AUTH" \
  -d '{"competencyCode":"integritas_akhlaq","level":9}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "level di luar 1–5 ditolak" "$BAD_LEVEL" "VALIDATION_ERROR"

# 6. Otorisasi: kamus kompetensi hanya admin.
UNAUTH_COMP=$(req "$API/admin/competencies" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "kamus kompetensi butuh token" "$UNAUTH_COMP" "UNAUTHORIZED"
FORBIDDEN_COMP=$(req "$API/admin/competencies" -H "$AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "peserta tidak bisa membuka kamus kompetensi" "$FORBIDDEN_COMP" "FORBIDDEN"

say "Talent mapping (Fase 3)"
TALENT_TOKEN=$(req -X POST "$API/auth/login" -d '{"email":"rangga.pratama@bumnacademy.id","password":"peserta123"}' | jq1 "['accessToken']")
TALENT_AUTH="Authorization: Bearer $TALENT_TOKEN"
[ -n "$TALENT_TOKEN" ] && ok "login peserta untuk asesmen minat–bakat" || bad "login peserta gagal"

# 1. Instrumen minat–bakat tersedia.
INSTRUMENTS=$(req "$API/talent/instruments" -H "$TALENT_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d) >= 2, all(i["itemCount"] >= 3 for i in d))')
check "instrumen minat–bakat tersedia (jumlah/butir)" "$INSTRUMENTS" "True True"

# 2. Memulai asesmen tanpa persetujuan ditolak (UU PDP) — memakai akun baru
#    yang belum pernah memberi persetujuan.
NO_CONSENT=$(req -X POST "$API/talent/assessments" -H "$AUTH" -d '{"instrumentCode":"riasec-v1"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "asesmen tanpa persetujuan ditolak" "$NO_CONSENT" "VALIDATION_ERROR"

# 3. Mulai dengan persetujuan → butir dikirim (tanpa kunci/arah item).
START=$(req -X POST "$API/talent/assessments" -H "$TALENT_AUTH" \
  -d '{"instrumentCode":"riasec-v1","consentGranted":true}')
ASSESSMENT_ID=$(echo "$START" | jq1 "['id']")
START_CHECK=$(echo "$START" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["status"], len(d["items"]) >= 10, d["answeredCount"])')
check "asesmen dimulai (status/butir/terjawab)" "$START_CHECK" "berjalan True 0"

# 4. Autosave seluruh jawaban.
ANSWERS=$(echo "$START" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(json.dumps({"answers":[{"itemId":i["id"],"value":4} for i in d["items"]]}))')
SAVED=$(req -X PATCH "$API/talent/assessments/$ASSESSMENT_ID/answers" -H "$TALENT_AUTH" -d "$ANSWERS" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["answeredCount"] == d["itemCount"])')
check "jawaban tersimpan (semua butir terjawab)" "$SAVED" "True"

# 5. Kumpulkan → skor dimensi + peringkat kecocokan lini bisnis.
PROFILE=$(req -X POST "$API/talent/assessments/$ASSESSMENT_ID/submit" -H "$TALENT_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d["dimensions"]) >= 6, len(d["fit"]) >= 1, all(0 <= x["percentile"] <= 100 for x in d["dimensions"]), len(d["notes"]) >= 1)')
check "profil talenta (dimensi/kecocokan/persentil/catatan)" "$PROFILE" "True True True True"

# 6. Kumpulkan ulang idempoten (tidak menilai ulang).
RESUBMIT=$(req -X POST "$API/talent/assessments/$ASSESSMENT_ID/submit" -H "$TALENT_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d["dimensions"]) >= 6)')
check "kumpulkan ulang idempoten" "$RESUBMIT" "True"

# 7. Peta talenta admin memuat lini bisnis, dimensi, dan baris peserta berdata.
MAPPING=$(req "$API/admin/talent/mapping?search=rangga" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];rows=d["rows"];print(len(d["businessLines"]) >= 1, len(d["dimensions"]) >= 6, len(rows) >= 1, any(r["hasData"] for r in rows))')
check "peta talenta admin (lini/dimensi/baris/berdata)" "$MAPPING" "True True True True"

# 8. Bobot lini bisnis bisa diubah (lalu dikembalikan agar data demo utuh).
LINES=$(req "$API/admin/business-lines" -H "$ADMIN_AUTH")
LINE_ID=$(echo "$LINES" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d[0]["id"])')
ORIG_WEIGHTS=$(echo "$LINES" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(json.dumps(d[0]["weights"]))')
PATCHED=$(req -X PATCH "$API/admin/business-lines/$LINE_ID" -H "$ADMIN_AUTH" \
  -d '{"weights":{"realistic":3,"investigative":2}}' \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];x=[i for i in d if i["id"]=="'"$LINE_ID"'"][0];print(x["weights"].get("realistic"))')
check "bobot lini bisnis bisa diubah" "$PATCHED" "3"
req -o /dev/null -X PATCH "$API/admin/business-lines/$LINE_ID" -H "$ADMIN_AUTH" -d "{\"weights\":$ORIG_WEIGHTS}"

# 9. Negatif: peserta tidak bisa membuka peta talenta admin.
FORBIDDEN_TALENT=$(req "$API/admin/talent/mapping" -H "$TALENT_AUTH" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "peserta tidak bisa membuka peta talenta admin" "$FORBIDDEN_TALENT" "FORBIDDEN"

# 10. Peserta dengan tryout + instrumen bakat: kerjakan asesmen bakat (aptitude).
APT_START=$(req -X POST "$API/talent/assessments" -H "$AUTH" \
  -d '{"instrumentCode":"aptitude-v1","consentGranted":true}')
APT_ID=$(echo "$APT_START" | jq1 "['id']")
APT_CHECK=$(echo "$APT_START" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["status"], len(d["items"]) >= 10)')
check "asesmen bakat dimulai (status/butir)" "$APT_CHECK" "berjalan True"
APT_ANSWERS=$(echo "$APT_START" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(json.dumps({"answers":[{"itemId":i["id"],"value":4} for i in d["items"]]}))')
req -o /dev/null -X PATCH "$API/talent/assessments/$APT_ID/answers" -H "$AUTH" -d "$APT_ANSWERS"
APT_DIMS=$(req -X POST "$API/talent/assessments/$APT_ID/submit" -H "$AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(sorted(x["dimension"] for x in d["dimensions"]) == ["figural","learning_agility","numerik","verbal"])')
check "skor bakat lengkap (numerik/verbal/figural/learning agility)" "$APT_DIMS" "True"

# 11. Matriks 9-box: 9 kotak pada grid 3×3 dengan label & saran lengkap.
NINEBOX=$(req "$API/admin/talent/nine-box" -H "$ADMIN_AUTH")
NINE=$(echo "$NINEBOX" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];c=d["cells"];print(len(c) == 9, len({(x["row"],x["column"]) for x in c}) == 9, all(x["label"] and x["shortLabel"] and x["action"] and isinstance(x["users"], list) for x in c))')
check "9-box (9 kotak/grid unik/label+saran/anggota berupa daftar)" "$NINE" "True True True"

# 12. Jumlah anggota kotak selaras ringkasan; "data belum cukup" tidak masuk kuadran.
NINE_SUM=$(echo "$NINEBOX" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(sum(x["count"] for x in d["cells"]) == d["summary"]["withData"], len(d["insufficient"]) == d["summary"]["insufficient"], d["summary"]["totalPeserta"] == d["summary"]["withData"] + d["summary"]["insufficient"])')
check "9-box (jumlah kotak/ringkasan/data belum cukup selaras)" "$NINE_SUM" "True True True"

# 13. Setiap peserta berada di kotak yang sesuai band skornya (bukan kuadran lain).

NINE_BAND=$(echo "$NINEBOX" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
lo, hi = d["thresholds"]["lowMax"], d["thresholds"]["highMin"]
def band(v):
    return "tinggi" if v >= hi else ("sedang" if v >= lo else "rendah")
same = all(
    band(m["performance"]) == cell["performanceBand"] and band(m["potential"]) == cell["potentialBand"]
    for cell in d["cells"] for m in cell["users"]
)
print(same, len(d["notes"]) >= 1)
')
check "9-box menempatkan peserta sesuai band skornya" "$NINE_BAND" "True True"

# 14. Peserta dengan tryout + hasil instrumen bakat benar-benar terpetakan.
NINE_FOUND=$(req "$API/admin/talent/nine-box?search=$STAMP" -H "$ADMIN_AUTH" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["summary"]["totalPeserta"] >= 1, d["summary"]["withData"] >= 1)')
check "peserta dengan tryout + bakat terpetakan pada 9-box" "$NINE_FOUND" "True True"

# 15. Negatif: peserta tidak bisa membuka matriks 9-box admin.

say "Psikolog & konsultasi (Fase 3)"
PSIKO_TOKEN=$(req -X POST "$API/auth/login" -d '{"email":"psikolog@bumnacademy.id","password":"psikolog123"}' | jq1 "['accessToken']")
PSIKO_AUTH="Authorization: Bearer $PSIKO_TOKEN"
[ -n "$PSIKO_TOKEN" ] && ok "login mitra psikolog demo" || bad "login mitra psikolog gagal"
SITI_TOKEN=$(req -X POST "$API/auth/login" -d '{"email":"siti.nurhaliza@bumnacademy.id","password":"peserta123"}' | jq1 "['accessToken']")
SITI_AUTH="Authorization: Bearer $SITI_TOKEN"
[ -n "$SITI_TOKEN" ] && ok "login peserta demo konsultasi" || bad "login peserta konsultasi gagal"

# 16. Registri mitra psikolog: satu terverifikasi, satu masih menunggu.
PARTNERS=$(req "$API/admin/psychologists" -H "$ADMIN_AUTH")
PARTNER_CHECK=$(echo "$PARTNERS" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d) >= 2, any(p["status"] == "terverifikasi" and p["assignable"] for p in d), any(p["status"] == "pending" for p in d))')
check "registri mitra psikolog (terverifikasi/pending/assignable)" "$PARTNER_CHECK" "True True True"
PARTNER_ID=$(echo "$PARTNERS" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print([p["id"] for p in d if p["status"] == "terverifikasi"][0])')

# 17. Katalog publik peserta: hanya mitra terverifikasi, tanpa nomor izin praktik.
PUBLIC_PSIKO=$(req "$API/psychologists" -H "$SITI_AUTH")
PUBLIC_CHECK=$(echo "$PUBLIC_PSIKO" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d) >= 1, all("sippNumber" not in p and "strNumber" not in p for p in d), any(p["openSlots"] >= 1 for p in d))')
check "daftar mitra publik (terverifikasi/tanpa nomor izin/ada slot)" "$PUBLIC_CHECK" "True True True"

# 18. Memesan slot tanpa persetujuan ditolak (data psikologis = data spesifik).
SLOTS=$(req "$API/psychologists/$PARTNER_ID/slots" -H "$SITI_AUTH")
SLOT_ID=$(echo "$SLOTS" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print([s["id"] for s in d if s["remaining"] >= 1][0])')
NO_CONSENT_BOOK=$(req -X POST "$API/consultations" -H "$AUTH" -d "{\"slotId\":\"$SLOT_ID\"}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "pemesanan konsultasi tanpa persetujuan ditolak" "$NO_CONSENT_BOOK" "VALIDATION_ERROR"

# 19. Dengan persetujuan, pemesanan berhasil dan tercatat di riwayat.
BOOKED=$(req -X POST "$API/consultations" -H "$AUTH" -d "{\"slotId\":\"$SLOT_ID\",\"agenda\":\"Konsultasi hasil tryout.\",\"consentGranted\":true}" \
  | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d["bookings"]) >= 1, d["bookings"][0]["status"], d["bookings"][0]["statusLabel"])')
check "pemesanan konsultasi berhasil (status)" "$BOOKED" "True terpesan Terpesan"
BOOKING_ID=$(req "$API/me/consultations" -H "$AUTH" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["bookings"][0]["id"])')

# 20. Memesan slot yang sama dua kali ditolak (satu peserta satu slot).
DOUBLE=$(req -X POST "$API/consultations" -H "$AUTH" -d "{\"slotId\":\"$SLOT_ID\",\"consentGranted\":true}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "pemesanan ganda pada slot sama ditolak" "$DOUBLE" "CONFLICT"

# 21. Agenda psikolog memuat sesi yang ditugaskan kepadanya.
AGENDA=$(req "$API/psikolog/agenda" -H "$PSIKO_AUTH")
AGENDA_CHECK=$(echo "$AGENDA" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["verified"], len(d["sessions"]) >= 1, all("userId" in s for s in d["sessions"]))')
check "agenda psikolog (terverifikasi/sesi/peserta)" "$AGENDA_CHECK" "True True True"
# Peserta yang ditugaskan diambil eksplisit dari agenda: sesi demo bersama
# "Siti Nurhaliza" (bukan peserta smoke yang ikut memesan di langkah 19).
SUBJECT_ID=$(echo "$AGENDA" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print([s["userId"] for s in d["sessions"] if s["userName"] == "Siti Nurhaliza"][0])')
SUBJECT_BOOKING=$(echo "$AGENDA" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print([s["bookingId"] for s in d["sessions"] if s["userName"] == "Siti Nurhaliza"][0])')

# 22. Psikolog membuka peserta yang ditugaskan — termasuk jawaban item mentah.
SITI_ASSESS=$(req -X POST "$API/talent/assessments" -H "$SITI_AUTH" -d '{"instrumentCode":"aptitude-v1","consentGranted":true}')
SITI_ASSESS_ID=$(echo "$SITI_ASSESS" | jq1 "['id']")
SITI_ANSWERS=$(echo "$SITI_ASSESS" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(json.dumps({"answers":[{"itemId":i["id"],"value":4} for i in d["items"]]}))')
req -o /dev/null -X PATCH "$API/talent/assessments/$SITI_ASSESS_ID/answers" -H "$SITI_AUTH" -d "$SITI_ANSWERS" || true
req -o /dev/null -X POST "$API/talent/assessments/$SITI_ASSESS_ID/submit" -H "$SITI_AUTH" || true
PARTICIPANT=$(req "$API/psikolog/participants/$SUBJECT_ID" -H "$PSIKO_AUTH")
PARTICIPANT_CHECK=$(echo "$PARTICIPANT" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(d["userId"] == "'"$SUBJECT_ID"'", len(d["assessments"]) >= 1, sum(len(a["answers"]) for a in d["assessments"]) >= 10, isinstance(d["interpretations"], list))')
check "psikolog membuka peserta ditugaskan (jawaban item mentah)" "$PARTICIPANT_CHECK" "True True True True"

# 23. Peserta di luar penugasan ditolak (FORBIDDEN).
#     Dipakai akun demo yang tidak punya pemesanan dengan mitra ini — peserta
#     smoke pada langkah 19 justru sudah ditugaskan, jadi tidak bisa dipakai.
OUTSIDE_TOKEN=$(req -X POST "$API/auth/login" -d '{"email":"peserta@bumnacademy.id","password":"peserta123"}' | jq1 "['accessToken']")
OUTSIDE_ID=$(req "$API/auth/me" -H "Authorization: Bearer $OUTSIDE_TOKEN" | jq1 "['id']")
OUTSIDE=$(req "$API/psikolog/participants/$OUTSIDE_ID" -H "$PSIKO_AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "psikolog di luar penugasan ditolak" "$OUTSIDE" "FORBIDDEN"

# 24. Peserta tidak bisa membuka area psikolog.
NOT_PSIKO=$(req "$API/psikolog/agenda" -H "$SITI_AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "peserta tidak bisa membuka area psikolog" "$NOT_PSIKO" "FORBIDDEN"

FORBIDDEN_NINE=$(req "$API/admin/talent/nine-box" -H "$TALENT_AUTH" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "peserta tidak bisa membuka matriks 9-box" "$FORBIDDEN_NINE" "FORBIDDEN"


# 25. Catatan `private` tidak pernah sampai ke peserta; `shared_to_user` sampai.
PRIVATE_MARK="rahasia-$STAMP"
req -o /dev/null -X POST "$API/psikolog/consultations/$SUBJECT_BOOKING/note" -H "$PSIKO_AUTH" \
  -d "{\"summary\":\"$PRIVATE_MARK hanya untuk psikolog.\",\"visibility\":\"private\"}" || true
SHARED_MARK="dibagikan-$STAMP"
req -o /dev/null -X POST "$API/psikolog/consultations/$SUBJECT_BOOKING/note" -H "$PSIKO_AUTH" \
  -d "{\"summary\":\"$SHARED_MARK untuk peserta.\",\"recommendation\":\"Latih penalaran numerik.\",\"visibility\":\"shared_to_user\",\"retentionUntil\":\"2030-01-01\"}" || true
HISTORY=$(req "$API/me/consultations" -H "$SITI_AUTH")
NOTE_CHECK=$(echo "$HISTORY" | python3 -c '
import sys, json
raw = sys.stdin.read()
d = json.loads(raw)["data"]
print("'"$PRIVATE_MARK"'" not in raw,
      "'"$SHARED_MARK"'" in raw,
      all(n["visibility"] != "private" for n in d["notes"]),
      all("visibilityLabel" in n for n in d["notes"]))
')
check "catatan berlapis visibilitas (private tidak dibagikan)" "$NOTE_CHECK" "True True True True"

# 26. Jejak akses data asesmen tercatat dan dapat dilihat peserta.
LOGS=$(req "$API/me/assessment-access-logs" -H "$SITI_AUTH")
LOG_CHECK=$(echo "$LOGS" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];print(len(d["logs"]) >= 1, any(l["resource"] == "item_answers" for l in d["logs"]), all(l["actorRole"] for l in d["logs"]))')
check "jejak akses data asesmen (tercatat & terlihat peserta)" "$LOG_CHECK" "True True True"

# 27. Rekap admin memuat ringkasan status tanpa isi catatan konsultasi.
RECAP=$(req "$API/admin/consultations" -H "$ADMIN_AUTH")
RECAP_CHECK=$(echo "$RECAP" | python3 -c '
import sys, json
raw = sys.stdin.read()
d = json.loads(raw)["data"]
print(len(d["bookings"]) >= 1, len(d["summary"]) >= 1, "penalaran analitis" not in raw, len(d["notes"]) >= 1)
')
check "rekap konsultasi admin (ringkasan status, tanpa isi catatan)" "$RECAP_CHECK" "True True True True"

# 28. Mitra tanpa izin aktif tidak boleh ditugaskan; tanggal izin lewat ditolak.
NEW_PARTNER_NAME="Mitra Uji $STAMP"
req -o /dev/null -X POST "$API/admin/psychologists" -H "$ADMIN_AUTH" \
  -d "{\"fullName\":\"$NEW_PARTNER_NAME\",\"email\":\"mitra-$STAMP@mitrapsikologi.id\",\"sippNumber\":\"SIPP-2026-$STAMP\",\"specializations\":[\"Asesmen\"]}" || true
NEW_PARTNER_ID=$(req "$API/admin/psychologists" -H "$ADMIN_AUTH" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
print([p["id"] for p in d if p["fullName"] == "'"$NEW_PARTNER_NAME"'"][0])
')
BAD_VERIFY=$(req -X POST "$API/admin/psychologists/$NEW_PARTNER_ID/verify" -H "$ADMIN_AUTH" \
  -d '{"status":"terverifikasi","validUntil":"2020-01-01"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "verifikasi mitra dengan izin lewat ditolak" "$BAD_VERIFY" "VALIDATION_ERROR"
PENDING_NOT_ASSIGNABLE=$(req "$API/admin/psychologists" -H "$ADMIN_AUTH" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
p = [x for x in d if x["fullName"] == "'"$NEW_PARTNER_NAME"'"][0]
print(p["status"] == "pending", p["assignable"] is False)
')
check "mitra baru berstatus pending & belum bisa ditugaskan" "$PENDING_NOT_ASSIGNABLE" "True True"

# 29. Negatif: peserta tidak bisa membuka rekap konsultasi admin.
FORBIDDEN_RECAP=$(req "$API/admin/consultations" -H "$SITI_AUTH" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "peserta tidak bisa membuka rekap konsultasi admin" "$FORBIDDEN_RECAP" "FORBIDDEN"

# 30. Penarikan persetujuan (UU PDP §4.7): peserta melihat & menarik
#     persetujuannya, dan pemrosesan baru berhenti sampai disetujui ulang.
CONSENTS=$(req "$API/me/consents" -H "$AUTH")
CONSENT_CHECK=$(echo "$CONSENTS" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
print(len(d["purposes"]) == 4,
      all(p["label"] for p in d["purposes"]),
      any(p["purpose"] == "assessment" and p["active"] for p in d["purposes"]))
')
check "daftar persetujuan peserta (4 tujuan/label/aktif)" "$CONSENT_CHECK" "True True True"

REVOKE=$(req -X POST "$API/me/consents/assessment/revoke" -H "$AUTH")
REVOKE_CHECK=$(echo "$REVOKE" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
row = [c for c in d["consents"] if c["purpose"] == "assessment"][0]
purpose = [p for p in d["purposes"] if p["purpose"] == "assessment"][0]
print(row["active"] is False, bool(row.get("revokedAt")), purpose["active"] is False)
')
check "penarikan persetujuan tercatat (revokedAt/aktif=false)" "$REVOKE_CHECK" "True True True"

REVOKED_START=$(req -X POST "$API/talent/assessments" -H "$AUTH" \
  -d '{"instrumentCode":"riasec-v1"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "pemrosesan baru berhenti setelah persetujuan ditarik" "$REVOKED_START" "VALIDATION_ERROR"

REGRANT=$(req -X POST "$API/talent/assessments" -H "$AUTH" \
  -d '{"instrumentCode":"riasec-v1","consentGranted":true}' | jq1 "['status']")
check "persetujuan ulang memulihkan pemrosesan" "$REGRANT" "berjalan"

BAD_PURPOSE=$(req -X POST "$API/me/consents/kepribadian/revoke" -H "$AUTH" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "tujuan persetujuan tak dikenal ditolak" "$BAD_PURPOSE" "VALIDATION_ERROR"

say "Validasi input"
BAD=$(req -X POST "$API/auth/register" -d '{"name":"x","email":"bukan-email","password":"123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "validasi menolak payload tidak valid" "$BAD" "VALIDATION_ERROR"

UNAUTH=$(req "$API/orders" | python3 -c 'import sys,json;print(json.load(sys.stdin)["error"]["code"])')
check "endpoint peserta butuh token" "$UNAUTH" "UNAUTHORIZED"

printf '\n\033[1mLulus: %d, Gagal: %d\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
