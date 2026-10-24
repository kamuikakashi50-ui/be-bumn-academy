# be-bumn-academy — Backend API

Backend **Go** untuk front-end `bumn-academy` (Next.js). Melayani katalog
program, sistem pembelian (voucher & referral), ruang kelas digital, serta
**mesin tryout bergaya CBT** yang dirancang kuat saat **banyak peserta
mengerjakan tryout serentak**.

| Bagian | Teknologi |
| --- | --- |
| Bahasa | Go 1.25 |
| Router | chi v5 (idiomatik `net/http`) |
| ORM | GORM v1.31 + driver PostgreSQL (pgx v5) |
| Database | PostgreSQL 16 |
| Migrasi | SQL berversi di-embed (`embed.FS`), dijalankan otomatis saat start |
| Cache | Redis 7 (`go-redis` v9) — opsional, ada fallback in-memory |
| Auth | JWT HS256 (stateless) + refresh token berotasi (hash SHA-256) |
| Validasi | go-playground/validator v10 |
| Import soal | Excel (.xlsx, excelize) & CSV |
| Observabilitas | `log/slog` terstruktur, `/healthz`, `/readyz` |

---

## 1. Menjalankan

### Cara tercepat (Docker)

```bash
cp .env.example .env
docker compose up -d --build      # postgres + redis + api
curl http://127.0.0.1:8080/readyz
```

API akan otomatis menjalankan migrasi (`DB_AUTO_MIGRATE=true`) dan menyisipkan
data demo (`SEED_ON_START=true`) saat pertama kali start.

> **Port host:** PostgreSQL dipetakan ke `55432` dan Redis ke `56379` agar tidak
> bentrok dengan PostgreSQL/Redis lokal yang mungkin sudah berjalan di mesin
> pengembangan. Ubah lewat `POSTGRES_HOST_PORT` / `REDIS_HOST_PORT`.
>
> **Build gagal dengan `no required module provides package .../internal/seed`?**
> Artinya berkas sumber tidak ikut terbawa ke build context — biasanya karena
> build dilakukan dari hasil clone/arsip git yang tidak memuat direktori tersebut.
> Pastikan `.gitignore` memakai pola ber-anchor (`/seed`, `/migrate`) sehingga
> `internal/seed`, `cmd/seed`, dan `cmd/migrate` tetap terlacak; cek dengan
> `git ls-files internal/seed cmd/seed cmd/migrate`. Detailnya di bagian
> [Troubleshooting](#troubleshooting).

### Cara lokal (Go langsung)

```bash
docker compose up -d postgres redis   # cukup dependency-nya
cp .env.example .env
make migrate-up                       # jalankan migrasi
make seed                             # isi data demo
make run                              # http://localhost:8080
```

### Skala horizontal

```bash
docker compose up -d --scale api=3
```

Karena access token bersifat stateless dan migrasi dilindungi advisory lock
PostgreSQL, beberapa replika API bisa start bersamaan dengan aman.

---

## 2. Struktur proyek

```
be-bumn-academy/
├── cmd/
│   ├── api/            # HTTP server (+ wiring.go menyusun dependency)
│   ├── migrate/        # CLI migrasi: up | down | status
│   └── seed/           # CLI penyisip data demo
├── internal/
│   ├── ai/             # penyedia model: OpenAI-compatible, mentor heuristik lokal,
│   │                   # prompt builder, konteks hasil tryout, baseline analisis
│   ├── auth/           # JWT, refresh token, principal di context
│   ├── cache/          # Redis, pola "remember" + singleflight, rate limiter
│   ├── config/         # pembacaan & validasi environment
│   ├── database/       # koneksi GORM (pool di-tuning) + runner migrasi
│   ├── dto/            # kontrak request/response (camelCase)
│   ├── handler/        # lapisan HTTP
│   ├── middleware/     # auth, admin guard, rate limit, logging
│   ├── models/         # model GORM + tipe kolom JSONB
│   ├── pkg/            # apperror, bank, ebook (generator PDF), logger, midtrans,
│   │                   # password, response, validator
│   ├── repository/     # seluruh akses database
│   ├── router/         # peta seluruh route
│   ├── seed/           # data demo (data.json dari modul mock front-end)
│   └── service/        # aturan bisnis
├── media/              # berkas media ruang kelas (video MP4 + poster)
├── migrations/         # SQL berversi (kompatibel penamaan golang-migrate)
└── scripts/            # smoke-test.sh, loadtest.go, generate-media.sh
```

Alur dependensi satu arah: `handler → service → repository → database`.

> **Kelengkapan berkas sumber.** Folder `cmd/migrate`, `cmd/seed`, dan
> `internal/seed` sempat tidak ikut ter-commit karena pola `.gitignore` (`seed`
> dan `migrate` tanpa garis miring depan cocok ke folder mana pun bernama sama),
> sehingga `go build ./...` gagal dengan pesan
> `no required module provides package .../internal/seed`. Pola itu kini dibatasi
> ke root (`/be-bumn-academy`, `/migrate`, `/seed`) dan ketiga folder tersebut
> sudah ter-versi, jadi `git clone` + `go build ./...` berhasil tanpa langkah
> tambahan.

---

## 3. Migrasi & seed

```bash
make migrate-status   # lihat migrasi diterapkan & tertunda
make migrate-up       # terapkan semua
make migrate-down     # rollback 1 migrasi terakhir
make seed             # isi data demo (idempotent)
```

Berkas migrasi memakai penamaan `NNNNNN_nama.up.sql` / `.down.sql` sehingga juga
bisa dijalankan dengan `migrate` CLI bila tim memakai tooling tersebut.

| Versi | Isi |
| --- | --- |
| 001 | users, refresh_tokens |
| 002 | programs, materi_categories, products, payment_methods, vouchers |
| 003 | orders, enrollments, notification_logs |
| 004 | questions, tryout_packages, subtests |
| 005 | tryout_sessions, tryout_answers, attempts, attempt_categories |
| 006 | live_sessions, recordings, session_reminders, broadcast_logs |
| 007 | sequence nomor invoice |
| 008 | attempt_analyses, ai_conversations, ai_messages, ai_usage_counters |
| 009 | kolom Midtrans pada orders (payment_provider/status/snap_token/redirect) |
| 010 | course_modules, lessons, lesson_progress (ruang kelas digital) |

Data demo di-embed dari `internal/seed/data.json` yang dihasilkan dari modul mock
front-end (45 soal, 5 paket tryout, 4 produk, 4 program, 3 voucher, 6 rekaman),
sehingga angka soal/menit pada kartu tryout **sama persis** dengan front-end.

Materi ruang kelas digital diambil dari `internal/seed/classroom.json` (8 modul,
15 materi) dengan tautan video mengarah ke berkas di `media/lessons/`. Berkas itu
dibuat oleh `scripts/generate-media.sh` (Pillow merender slide, ffmpeg mengemas
MP4 H.264 1280×720 + poster) sehingga peserta benar-benar memutar video asli.
Direktori media dilayani pada `/api/v1/media/*` (lihat `MEDIA_DIR`).

### Isi paket `internal/seed`

| Berkas | Isi |
| --- | --- |
| `seed.go` | orkestrasi `seed.Run(ctx, db, cfg)` + upsert seluruh katalog memakai UUID deterministik (`uuid.NewSHA1`) agar seed berulang tidak membuat baris baru |
| `data.go` | struktur JSON + `//go:embed data.json classroom.json` |
| `learning.go` | jadwal kelas live (disimpan sebagai selisih jam dari waktu seed) & 6 rekaman kelas |
| `data.json` | program, kategori materi, produk, metode bayar, voucher, 45 soal, 5 paket tryout, akun demo, enrollment demo |
| `classroom.json` | 8 modul & 15 materi (6 video dari `media/lessons/` + 9 e-book yang dirender jadi PDF saat dibuka) |
| `seed_test.go` | uji kontrak data: jumlah data, id unik, bank soal tiap subtes, produk wajib tiap paket, keberadaan berkas media, determinisme `stableID` |

Seed dipanggil otomatis saat start (`SEED_ON_START=true`) maupun manual lewat
`make seed` / `go run ./cmd/seed`. Seluruh operasi idempoten (upsert), sehingga
aman dijalankan berulang — termasuk saat beberapa replika API start bersamaan.

### Akun demo

| Role | Email | Kata sandi |
| --- | --- | --- |
| Peserta | `peserta@bumnacademy.id` | `peserta123` |
| Admin | `admin@bumnacademy.id` | `admin123` |

---

## 4. Endpoint

Semua endpoint berada di bawah `/api/v1`. Kolom **Akses**: `publik`, `peserta`
(butuh token), `opsional` (token dipakai bila ada), `admin`.

### Autentikasi

| Metode | Path | Akses | Keterangan |
| --- | --- | --- | --- |
| POST | `/auth/register` | publik | daftar peserta + kode referral otomatis |
| POST | `/auth/login` | publik | terbitkan access + refresh token |
| POST | `/auth/refresh` | publik | rotasi token (token lama dicabut) |
| POST | `/auth/logout` | publik | cabut refresh token |
| GET | `/auth/me` | peserta | profil pengguna aktif |
| PUT | `/auth/me` | peserta | perbarui nama/program |

### Katalog & ruang belajar

| Metode | Path | Akses | Keterangan |
| --- | --- | --- | --- |
| GET | `/programs` | publik | daftar program |
| GET | `/materi/categories` | publik | kategori materi + topik |
| GET | `/payment-methods` | publik | metode pembayaran |
| GET | `/products` | opsional | produk + status kepemilikan (`owned`) |
| GET | `/live-sessions` | peserta | jadwal live + status pengingat |
| POST | `/live-sessions/{id}/reminder` | peserta | nyalakan/matikan pengingat |
| GET | `/recordings` | peserta | rekaman (filter `category`, `search`) |
| GET | `/dashboard/summary` | peserta | ringkasan progres, tren skor, jadwal (+ `continueLearning`) |

### Ruang kelas digital

| Metode | Path | Akses | Keterangan |
| --- | --- | --- | --- |
| GET | `/classroom` | peserta | daftar kelas yang dimiliki + ringkasan progres |
| GET | `/classroom/courses/{slug}` | peserta | modul, materi, dan urutan materi satu kelas |
| GET | `/classroom/lessons/{id}` | peserta | detail materi (+ navigasi sebelumnya/berikutnya) |
| POST | `/classroom/lessons/{id}/progress` | peserta | simpan posisi menonton / tandai selesai |
| GET | `/classroom/lessons/{id}/ebook` | peserta | e-book PDF (dibuat dari isi materi; `?download=1` memaksa unduhan) |
| GET | `/media/*` | publik | berkas media pembelajaran (MP4/poster) dengan dukungan HTTP Range |

Aturan progres: posisi dibatasi oleh durasi materi, persentase dihitung ulang di
server, materi ditandai selesai otomatis pada >= 90% (atau eksplisit), dan status
selesai bersifat menetap. Semua endpoint kelas memerlukan kepemilikan produk
(`403` bila belum).

### Pembelian

| Metode | Path | Akses | Keterangan |
| --- | --- | --- | --- |
| POST | `/checkout/quote` | peserta | hitung harga (voucher, referral, biaya metode) |
| POST | `/orders` | peserta | buat pesanan (produk gratis → langsung lunas) |
| GET | `/orders` | peserta | riwayat pesanan (paginasi) |
| GET | `/orders/{id}` | peserta | detail pesanan |
| POST | `/orders/{id}/cancel` | peserta | batalkan pesanan pending |
| POST | `/orders/{id}/pay` | peserta | simulasi pembayaran berhasil (tanpa penyedia) |
| POST | `/orders/{id}/payment` | peserta | buat sesi pembayaran Midtrans (Snap token) — `?refresh=true` memaksa token baru |
| GET | `/orders/{id}/payment/status` | peserta | sinkronkan status pesanan dengan Midtrans |
| POST | `/orders/{id}/sandbox/settle` | peserta | simulator sandbox lokal (`settlement`/`pending`/`expire`/`deny`) |
| GET | `/midtrans/config` | publik | konfigurasi publik Midtrans (client key, snap.js, mode) |
| GET | `/enrollments` | peserta | produk yang dimiliki |
| POST | `/webhooks/midtrans` | publik | callback pembayaran (idempotent, **wajib signature sah**) |

### Pembayaran Midtrans (Snap)

Alur pembayaran berbayar:

1. `POST /orders` → pesanan `pending` (harga dihitung server, kuota voucher dicatat).
2. `POST /orders/{id}/payment` → backend memanggil
   `POST {snap}/v1/transactions` memakai Server Key dan mengembalikan
   `snapToken` + `snapRedirectUrl` (disimpan di kolom `orders.snap_token`).
   Rincian item (`item_details`) selalu berjumlah tepat sama dengan
   `gross_amount`: produk, diskon (negatif), dan biaya metode bila ada.
3. Front-end menampilkan halaman pembayaran lewat `snap.js`
   (`window.snap.pay(token, …)`) atau mengalihkan ke `snapRedirectUrl`.
4. Midtrans mengirim `POST /webhooks/midtrans`. Backend:
   - memverifikasi `signature_key` = `SHA512(order_id + status_code +
     gross_amount + ServerKey)` secara constant-time (permintaan palsu → 401),
   - memetakan status: `settlement`/`capture` → **lunas + akses produk**,
     `pending` → hanya dicatat, `expire`/`cancel`/`deny`/`failure` →
     kedaluwarsa, `refund`/`partial_refund` → dicatat (pesanan tetap lunas),
   - idempotent: notifikasi ganda tidak memberi akses dobel.
5. `GET /orders/{id}/payment/status` menjadi jaring pengaman: backend menanyakan
   status ke Core API (`GET /v2/{order_id}/status`) lalu menerapkan pemetaan yang
   sama. Berguna saat notifikasi tidak bisa menjangkau server (mis. backend di
   localhost tanpa tunnel).

**Simulator sandbox lokal.** Bila `MIDTRANS_SERVER_KEY` kosong, `POST
/orders/{id}/payment` mengembalikan `mode: "simulator"` dan front-end
menampilkan tombol simulasi. `POST /orders/{id}/sandbox/settle` menyusun
notifikasi tiruan **bertanda tangan kunci sandbox** dan mengirimkannya ke
`HandleWebhook` — jalur verifikasi signature, pelunasan, dan pemberian akses
benar-benar dijalankan tanpa akun Midtrans maupun URL publik. Simulator aktif
hanya bila `MIDTRANS_SANDBOX_SIMULATOR=true` dan `MIDTRANS_IS_PRODUCTION=false`.

**Menghubungkan Midtrans sandbox asli.**

1. Daftar akun Midtrans → ambil **Server Key** & **Client Key** mode Sandbox.
2. Isi `.env` (atau env host untuk Docker):
   ```env
   MIDTRANS_SERVER_KEY=SB-Mid-server-xxxxxxxxxxxx
   MIDTRANS_CLIENT_KEY=SB-Mid-client-xxxxxxxxxxxx
   MIDTRANS_IS_PRODUCTION=false
   MIDTRANS_SANDBOX_SIMULATOR=false   # opsional; matikan agar murni Snap
   FRONTEND_BASE_URL=http://localhost:3000
   ```
3. Di MAP: **Settings → Configuration**, isi *Payment Notification URL* dengan
   `https://<domain-publik>/api/v1/webhooks/midtrans` (butuh tunnel seperti ngrok
   bila backend masih di localhost) dan *Finish/Unfinish/Error Redirect URL* ke
   `http://localhost:3000/dashboard/pembayaran`.
4. Uji di sandbox: buat pesanan → tekan **Bayar dengan Midtrans** → pada halaman
   Snap gunakan menu **Simulator** untuk menandai pembayaran berhasil. Midtrans
   lalu memanggil webhook; status pesanan menjadi `paid` dan akses produk terbuka.

> Status tidak terbarui? Tekan **Cek status pembayaran** di halaman pembayaran —
> backend akan menanyakan status langsung ke Midtrans (Core API).

### Mesin tryout

| Metode | Path | Akses | Keterangan |
| --- | --- | --- | --- |
| GET | `/tryouts` | opsional | daftar paket + status akses & kuota percobaan |
| GET | `/tryouts/{id}` | opsional | detail paket + aturan pengerjaan |
| POST | `/tryouts/{id}/sessions` | peserta | mulai/lanjutkan sesi (idempotent) |
| GET | `/tryout-sessions/{id}` | peserta | keadaan sesi + sisa waktu server |
| PATCH | `/tryout-sessions/{id}/answers` | peserta | autosave jawaban (batch) |
| POST | `/tryout-sessions/{id}/submit` | peserta | kumpulkan + skor + pembahasan |
| GET | `/tryouts/{id}/leaderboard` | opsional | papan peringkat + peringkat pribadi |
| GET | `/attempts` | peserta | riwayat tryout |
| GET | `/attempts/{id}` | peserta | pembahasan lengkap satu hasil |

### Mentor AI ("MinSmart")

| Metode | Path | Akses | Keterangan |
| --- | --- | --- | --- |
| GET | `/attempts/{id}/ai/analysis` | peserta | analisis hasil tryout (`?force=true` untuk hitung ulang) |
| GET | `/attempts/{id}/ai/conversation` | peserta | riwayat percakapan mentor |
| POST | `/attempts/{id}/ai/chat` | peserta | tanya bebas (opsional `questionId`) |
| POST | `/attempts/{id}/ai/chat/stream` | peserta | balasan bertahap (Server-Sent Events) |
| POST | `/attempts/{id}/ai/explain` | peserta | penjelasan bertahap satu soal |
| DELETE | `/attempts/{id}/ai/conversation` | peserta | kosongkan percakapan |
| GET | `/ai/usage` | peserta | kuota mentor AI hari ini |
| GET | `/ai/status` | publik | kesiapan provider (`provider`, `model`, `degraded`) |

### Admin

| Metode | Path | Keterangan |
| --- | --- | --- |
| GET | `/admin/analytics` | ringkasan pengguna, pendapatan, distribusi skor |
| GET | `/admin/users` | daftar pengguna + aktivitas |
| GET | `/admin/orders` | semua pesanan (filter status/pencarian) |
| POST | `/admin/orders/{id}/mark-paid` | tandai lunas manual + beri akses |
| GET/POST | `/admin/questions` | daftar/tambah soal |
| PATCH/DELETE | `/admin/questions/{id}` | ubah/hapus soal |
| POST | `/admin/questions/import` | import soal dari Excel/CSV (multipart `file`) |
| POST/PATCH/DELETE | `/admin/tryouts[/{id}]` | CRUD paket tryout + subtes |
| POST/PATCH/DELETE | `/admin/products[/{slug}]` | CRUD produk |
| GET/POST | `/admin/vouchers`, PATCH/DELETE `/admin/vouchers/{code}` | CRUD voucher |
| POST | `/admin/broadcast` | broadcast WhatsApp ke segmen peserta |
| GET | `/admin/broadcasts` | riwayat broadcast |
| GET | `/admin/products?includeInactive=true` | daftar produk termasuk yang dinonaktifkan (soft delete) |
| GET | `/admin/tryouts?includeInactive=true` | daftar paket tryout termasuk yang dinonaktifkan |
| GET/POST | `/admin/classroom/modules` | daftar & tambah modul ruang kelas |
| PATCH/DELETE | `/admin/classroom/modules/{id}` | ubah/hapus modul (cascade materi) |
| GET/POST | `/admin/classroom/lessons` | daftar & tambah materi (video/e-book) |
| PATCH/DELETE | `/admin/classroom/lessons/{id}` | ubah/hapus materi |

### Soft delete (nonaktifkan) produk & paket tryout

Item tidak dihapus, hanya ditandai `active = false`. Efeknya:

| Perilaku | Produk nonaktif | Paket tryout nonaktif |
| --- | --- | --- |
| Katalog peserta (`GET /products`, `GET /tryouts`) | tersembunyi | tersembunyi |
| Pembelian (`POST /checkout/quote`, `POST /orders`) | ditolak: *Produk ini sedang tidak dijual.* | — |
| Detail & mulai sesi (`GET /tryouts/{id}`, `POST /tryouts/{id}/sessions`) | — | `404` untuk peserta, tetap terbuka untuk admin |
| Akses yang sudah dimiliki (enrollments, ruang kelas, riwayat) | tetap berjalan | sesi yang sedang berjalan tetap bisa diselesaikan |
| Panel admin (`GET /admin/products`, `GET /admin/tryouts`) | tetap terlihat + bisa diaktifkan kembali | tetap terlihat + bisa diaktifkan kembali |

Catatan implementasi: kolom `active` selalu ditulis eksplisit oleh Go (tag GORM
`not null` tanpa `default`), sehingga membuat item langsung dalam keadaan
nonaktif ikut bekerja — sebelumnya nilai `false` diabaikan karena default kolom.

### Kesehatan

| Metode | Path | Keterangan |
| --- | --- | --- |
| GET | `/healthz` | proses hidup (liveness) |
| GET | `/readyz` | status database + statistik pool + Redis (readiness) |

---

## 5. Kontrak balasan

```jsonc
// sukses
{ "success": true, "data": { /* … */ } }

// sukses dengan paginasi
{ "success": true, "data": [ /* … */ ],
  "meta": { "page": 1, "perPage": 20, "total": 45, "totalPages": 3 } }

// gagal
{ "success": false,
  "error": { "code": "VALIDATION_ERROR", "message": "…",
             "details": { "email": "email harus berupa alamat email yang valid." } } }
```

Kode error: `BAD_REQUEST`, `VALIDATION_ERROR`, `UNAUTHORIZED`, `FORBIDDEN`,
`NOT_FOUND`, `CONFLICT`, `LOCKED` (mis. waktu subtes habis), `RATE_LIMITED`,
`INTERNAL_ERROR`.

Seluruh nama field JSON memakai **camelCase** agar cocok langsung dengan tipe
TypeScript pada front-end (`TryoutPackage`, `Attempt`, `Order`, `LiveSession`, …).

---

## 6. Alur mesin tryout (server-authoritative)

```
POST /tryouts/{id}/sessions
   ├─ cek akses (produk wajib dimiliki) & sisa kuota percobaan
   ├─ jika sudah ada sesi aktif → dikembalikan apa adanya (idempotent)
   ├─ snapshot susunan soal per subtes (kolom question_order)
   └─ tenggat subtes pertama diset di server

PATCH /tryout-sessions/{id}/answers     (autosave, boleh batch)
   ├─ soal harus benar-benar bagian dari sesi
   ├─ subtes yang waktunya habis → 423 LOCKED
   └─ subtes baru otomatis mulai berjalan saat pertama diisi

POST /tryout-sessions/{id}/submit
   ├─ row lock (SELECT … FOR UPDATE) → submit ganda aman
   ├─ skor dihitung dari jawaban di DATABASE (bukan payload klien)
   ├─ attempt + rincian per subtes disimpan
   └─ pembahasan (kunci jawaban) baru dibuka di sini
```

Poin penting:

- **Waktu berjalan di server.** `subtestDeadlines` dan `expiresAt` disimpan di
  database; klien hanya menampilkan hitungan mundur dari `remainingSeconds`.
  Menutup browser tidak menghentikan waktu.
- **Kunci jawaban tidak pernah bocor** sebelum submit: `SessionQuestionView`
  tidak memuat `answerIndex`/`explanation`.
- **Sesi kedaluwarsa otomatis dikumpulkan** sehingga peserta tetap mendapat
  hasil dari jawaban yang sudah tersimpan (tidak hilang begitu saja).
- **Snapshot soal** (`question_order`) membuat perubahan bank soal di tengah
  sesi tidak mengacaukan penilaian.

---

## 7. Mentor AI ("MinSmart") — analisis & chat ulasan hasil tryout

Ini fitur yang di front-end masih ditandai *"ulasan Mentor AI masih dalam
pengembangan"*. Backend kini menyediakannya penuh: analisis otomatis satu hasil
tryout, tanya-jawab kontekstual, dan penjelasan per soal.

### Alur pemakaian

```
GET  /attempts/{id}/ai/analysis      → analisis hasil (dibuat sekali, lalu di-cache)
POST /attempts/{id}/ai/explain       → "jelaskan soal ini" untuk satu soal
POST /attempts/{id}/ai/chat          → tanya bebas tentang hasil tryout
POST /attempts/{id}/ai/chat/stream   → versi SSE (balasan bertahap)
GET  /attempts/{id}/ai/conversation  → riwayat percakapan
DELETE /attempts/{id}/ai/conversation→ kosongkan percakapan
GET  /ai/usage                       → kuota harian peserta
GET  /ai/status                      → kesiapan provider (bisa diakses tamu)
```

### Apa isi analisisnya

| Bagian | Isi |
| --- | --- |
| `headline` | judul singkat, mis. *"Skor 70 — Penalaran Verbal menahan skormu"* |
| `summary` | 2–4 kalimat: pencapaian, kekuatan, penghambat, catatan waktu |
| `readiness` + `readinessScore` | label kesiapan (`siap` / `hampir siap` / `perlu perbaikan`) + skor 0–100 |
| `strengths` | subtes yang sudah di atas ambang (atau yang relatif terbaik) |
| `weaknesses` | subtes di bawah ambang + berapa poin kurangnya |
| `focusAreas` | 3 prioritas berurutan dengan tindakan konkret |
| `studyPlan` | 3–5 langkah belajar yang bisa dikerjakan minggu ini |
| `questionNotes` | catatan per soal yang belum benar, lengkap dengan `questionId` |
| `suggestions` | tombol pertanyaan lanjutan siap pakai untuk UI |

`readinessScore` **selalu dihitung server** (bukan model), dengan rumus yang bisa
dipertanggungjawabkan:

```
readiness = 0.70 × skor_total
          + 0.30 × (100 − selisih skor subtes tertinggi & terendah)
          − 12 × (proporsi soal tidak dijawab)
```

Pemerataan subtes diberi bobot karena seleksi biasanya menetapkan ambang per
subtes, sedangkan soal kosong menandakan masalah manajemen waktu.

### Penyedia model

Tiga pilihan lewat `AI_PROVIDER`:

| Nilai | Keterangan |
| --- | --- |
| `stub` | **mentor heuristik lokal** — tanpa jaringan, selalu tersedia (default) |
| `ark` | BytePlus Ark (`https://ark.ap-southeast.bytepluses.com/api/v3`) |
| `openai` | endpoint apa pun yang kompatibel OpenAI (OpenAI, DeepSeek, Ollama, vLLM) |

Ketiganya memakai kontrak yang sama: satu blok `<KONTEKS_HASIL_TRYOUT>` berisi
fakta hasil tryout yang dikirim ke model, dan `stub` membacanya juga — sehingga
berpindah provider tidak mengubah perilaku fitur.

```bash
# memakai model sungguhan
AI_PROVIDER=ark
AI_API_KEY=<kunci>
AI_MODEL=deepseek-v4-flash-260425
```

`AI_FALLBACK_ENABLED=true` (default) membuat permintaan otomatis dialihkan ke
mentor heuristik lokal bila provider utama gagal — kuota habis, model belum
diaktifkan di konsol, atau gangguan jaringan. Peserta tidak pernah menemui
halaman hasil tryout yang rusak; operator melihat peringatan di log dan
`GET /ai/status` menampilkan `degraded: true` beserta penyebabnya.

### Pagar pengaman

| Risiko | Penanganan |
| --- | --- |
| Data peserta lain terbaca | konteks hanya dibangun dari `attempt` milik pemanggil; kepemilikan diperiksa sebelum apa pun dijalankan |
| Kunci jawaban bocor sebelum dikerjakan | konteks hanya dibuat dari attempt yang sudah `submitted` |
| Klaim kelulusan seleksi | aturan eksplisit pada system prompt + disclaimer ambang internal latihan |
| Prompt injection dari peserta | penanda internal (`<KONTEKS…>`, `[[SOAL:…]]`, `[[AUTO]]`) dinetralkan sebelum dikirim; isi pesan peserta diperlakukan sebagai data |
| Model mengarang soal | catatan soal yang menunjuk `questionId` di luar attempt dibuang, bagian kosong diisi baseline deterministik |
| Konteks membengkak | `AI_MAX_CONTEXT_QUESTIONS` + `AI_MAX_HISTORY_MESSAGES`, soal salah diprioritaskan saat dipotong |
| Biaya tak terkendali | kuota harian per peserta (`AI_DAILY_QUOTA`) dicatat atomik, plus rate limit umum |
| Analisis dihitung berulang | hasil disimpan per attempt (`attempt_analyses`, unique index) dan dipakai ulang |

### Contoh balasan mentor

```
### Soal 1 · Penalaran Numerik

> 3, 6, 11, 18, 27, … — angka berikutnya adalah?

**Jawaban tepat:** 38

**Jawabanmu:** 36

**Kenapa demikian**

Selisih antar suku naik 2 setiap langkah: +3, +5, +7, +9. Selisih berikutnya
+11, sehingga 27 + 11 = 38.

**Yang perlu kamu ingat**

Soal numerik hampir selalu bisa disederhanakan: ubah soal cerita menjadi satu
persamaan, lalu hitung. Kalau muncul deret, periksa selisih antar suku terlebih
dahulu, baru rasio.
```

### Format streaming (SSE)

`POST /attempts/{id}/ai/chat/stream` mengirim kejadian berikut:

```
event: user    data: { pesan peserta yang tersimpan }
event: delta   data: { "delta": "### Soal 1 · " }
event: delta   data: { "delta": "Penalaran Numerik\n" }
event: done    data: { "done": true, "message": { … }, "usage": { … } }
```

Balasan tetap disimpan walau klien menutup koneksi di tengah jalan, sehingga
riwayat percakapan tidak kehilangan jawaban yang sudah dihasilkan model.

---

## 8. Optimasi untuk tryout serentak

Ini bagian yang paling diperhatikan karena ribuan peserta bisa menekan
"Mulai Sesi" pada jam yang sama.

### a. Skema database

| Optimasi | Detail |
| --- | --- |
| Partial unique index | `tryout_sessions (user_id, tryout_id) WHERE status='active'` → mustahil ada dua sesi aktif; klik ganda tidak menghasilkan sesi ganda |
| PK gabungan untuk jawaban | `tryout_answers (session_id, question_id)` → autosave bisa `INSERT … ON CONFLICT DO UPDATE` dalam satu statement |
| Index panas | `questions (bank_key, position, id) WHERE active`, `attempts (tryout_id, score DESC, taken_at ASC)`, `attempts (user_id, taken_at DESC)`, `orders (user_id, created_at DESC)` |
| Sequence invoice | `nextval('order_invoice_seq')` → nomor invoice unik tanpa race condition |
| Kolom JSONB | `options`, `banks`, `question_order`, `subtest_deadlines` → snapshot tanpa tabel tambahan |

### b. Connection pool

```go
sqlDB.SetMaxOpenConns(cfg.DB.MaxOpenConns)   // default 60
sqlDB.SetMaxIdleConns(cfg.DB.MaxIdleConns)   // default 30
sqlDB.SetConnMaxLifetime(cfg.DB.ConnMaxLifetime)
sqlDB.SetConnMaxIdleTime(cfg.DB.ConnMaxIdleTime)
```

Tanpa batas ini, lonjakan trafik akan membuka koneksi tanpa henti sampai
PostgreSQL kehabisan slot `max_connections`.

### c. GORM

- `SkipDefaultTransaction: true` — mematikan transaksi implisit pada setiap
  penulisan tunggal (hemat round-trip & WAL).
- `PrepareStmt: true` — query berulang (autosave, ambil soal) tidak di-parse ulang.
- **Transaksi eksplisit hanya di jalur yang butuh atomicity** (submit tryout,
  upsert paket tryout).

### d. Caching (Redis) + anti cache-stampede

Bank soal, katalog, daftar tryout, dan leaderboard dibaca lewat pola
`Remember` dengan **singleflight**: saat cache kosong dan ribuan request datang
bersamaan, hanya **satu** request memuat data dari database.

```
request → cache hit?  ── ya ──▶ balas dari Redis
             │
             tidak
             ▼
   singleflight.Do(key)  → hanya 1 loader dijalankan
             ▼
   muat dari PostgreSQL → simpan ke Redis → balas
```

Redis bersifat opsional: bila mati, seluruh operasi cache menjadi no-op dan
aplikasi tetap berjalan (rate limit otomatis memakai jaring lokal).

### e. Penulisan & penguncian

| Titik | Teknik |
| --- | --- |
| Autosave jawaban | satu statement upsert, `CreateInBatches(…, 200)` |
| Submit sesi | `SELECT … FOR UPDATE` + idempotent (submit ulang mengembalikan attempt yang sama) |
| Pembayaran | `UPDATE … WHERE status='pending'` + `INSERT … ON CONFLICT DO NOTHING` untuk enrollment |
| Kuota voucher | `UPDATE … SET used_count = used_count + 1 WHERE … AND used_count < max_uses` |
| Import soal | `ON CONFLICT DO UPDATE` per batch 200 baris |

### f. Runtime

- **JWT stateless** → replika API tidak perlu session store bersama.
- **Timeout per request** (`REQUEST_TIMEOUT`) → query menggantung tidak menahan
  koneksi pool.
- **Graceful shutdown** → request yang sedang berjalan diselesaikan dulu.
- **Rate limit** per pengguna (bukan hanya per IP) agar satu jaringan kampus/NAT
  tidak saling memblokir.
- **Worker latar** menyapu sesi kedaluwarsa dan refresh token lama setiap 2 menit.
- **Health check** `/readyz` mengembalikan 503 saat database tidak siap sehingga
  load balancer berhenti mengirim trafik.

### g. Hasil pengukuran nyata

`go run ./scripts/loadtest.go -users 1000 -concurrency 150` (PostgreSQL 16 +
Redis 7 di Docker, API lokal, alur penuh: daftar → mulai sesi → autosave →
submit):

```
Fase 1/2 — mendaftarkan akun peserta…
  1000/1000 akun siap dalam 10.246s
Fase 2/2 — menjalankan tryout serentak…
----------------------------------------------------------
Total waktu tryout   : 2.335s
Peserta berhasil     : 1000/1000
Sesi dikumpulkan     : 1000
Latensi p50/p95/p99  : 79ms / 142ms / 173ms
Latensi maksimum     : 234ms
Throughput           : 428.2 peserta/detik
Error aplikasi       : 0
Error jaringan       : 0
Skor tidak valid     : 0
```

Reproduksi:

```bash
make loadtest                          # 200 peserta, 50 paralel
make loadtest USERS=1000 CONCURRENCY=150
```

---

## 9. Pengujian

```bash
make test        # unit test seluruh paket
make smoke       # 41 pemeriksaan end-to-end (butuh API hidup)
make ai-test     # 29 pemeriksaan mentor AI pada ulasan hasil tryout
make loadtest    # uji tryout serentak
```

### Unit test

Mencakup perhitungan skor, pemetaan DTO, tipe kolom JSONB, rate limiter,
middleware auth (401/403), parser import soal, dan helper error.

Ditambah pengujian integrasi Midtrans (`internal/pkg/midtrans`): rumus
`signature_key` SHA512, penolakan signature palsu, pemetaan metode pembayaran ke
kanal Snap, header `Authorization` Basic, payload transaksi, dan penerjemahan
galat dari API Midtrans (memakai `httptest`).

Ditambah pengujian lapisan mentor AI: urutan kekuatan/kelemahan subtes,
perhitungan kesiapan, pemotongan konteks, netralisasi prompt injection,
pembersihan balasan model, serta batas jumlah butir analisis.

```
ok  internal/ai         ok  internal/cache      ok  internal/middleware
ok  internal/models     ok  internal/pkg/apperror   ok  internal/pkg/bank
ok  internal/pkg/midtrans   ok  internal/pkg/password   ok  internal/pkg/validator
ok  internal/seed       ok  internal/service
```

### Smoke test

`scripts/smoke-test.sh` menjalankan alur lengkap: katalog → registrasi → refresh
token → sesi tryout → autosave → submit (termasuk uji idempotensi) → riwayat &
pembahasan → leaderboard → dashboard → checkout dengan voucher & referral →
pembayaran → webhook bertanda tangan → endpoint admin → import CSV → broadcast →
validasi input.

Langkah webhook menghitung signature `SHA512(order_id + status_code +
gross_amount + Server Key)` memakai kunci simulator lokal
(`internal/pkg/midtrans.SandboxDemoServerKey`) sehingga jalur notifikasi tetap
teruji tanpa akun Midtrans; satu pemeriksaan tambahan memastikan notifikasi
dengan tanda tangan palsu ditolak `UNAUTHORIZED`.

```
Lulus: 42, Gagal: 0
```

### Uji mentor AI

`scripts/ai-mentor-test.sh` mengerjakan satu tryout dengan jawaban campuran
(benar, salah, dan satu dibiarkan kosong), lalu memeriksa seluruh fitur mentor:

```
== Analisis hasil tryout      headline, kesiapan, kekuatan, kelemahan, rencana
== Mentor menjelaskan 1 soal  penjelasan bertahap untuk soal yang diminta
== Tanya mentor (chat)        balasan tersimpan + konteks terjawab
== Chat dengan fokus soal     scope=question, nomor soal terlampir
== Streaming balasan (SSE)    event user → 22 delta → done
== Riwayat percakapan         4 pesan peserta + 4 balasan mentor
== Kuota pemakaian            4/120 permintaan, token tercatat
== Pagar pengaman             NOT_FOUND, BAD_REQUEST, VALIDATION_ERROR, UNAUTHORIZED
== Reset percakapan           riwayat kosong, analisis tetap tersimpan

Lulus: 29, Gagal: 0
```

---

## 10. Menghubungkan ke front-end

Front-end `bumn-academy` saat ini masih memakai data mock + `localStorage`.
Untuk menyambungkannya:

1. Tambahkan env di front-end, mis. `.env.local`:
   ```
   NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
   ```
2. Ganti sumber data mock dengan panggilan API (bentuk field sudah identik):
   - `src/lib/mock/tryout.ts` → `GET /tryouts`, `GET /tryouts/{id}`
   - `src/lib/mock/session.ts` → `POST /tryouts/{id}/sessions`,
     `PATCH /tryout-sessions/{id}/answers`, `POST /tryout-sessions/{id}/submit`
   - `src/lib/mock/billing.ts` → `GET /products`, `POST /checkout/quote`,
     `POST /orders`, `GET /orders`
   - `src/lib/mock/learning.ts` → `GET /live-sessions`, `GET /recordings`
   - `src/lib/data.ts` → `GET /programs`, `GET /materi/categories`
   - `src/lib/auth/*` → `POST /auth/login`, `GET /auth/me` (ganti cookie demo
     dengan menyimpan `accessToken` + `refreshToken`)
3. Timer pada `session-engine.tsx` diubah agar memakai `remainingSeconds` dari
   server (`GET /tryout-sessions/{id}`) alih-alih hitungan lokal.
4. Panel Mentor AI ditambahkan pada halaman hasil (`session-result.tsx`) dan
   dialog riwayat (`history-view.tsx`), menggantikan catatan
   *"ulasan Mentor AI masih dalam pengembangan"*:
   - saat halaman hasil dibuka → `GET /attempts/{id}/ai/analysis`;
   - tombol **"Jelaskan soal ini"** pada tiap kartu pembahasan →
     `POST /attempts/{id}/ai/explain`;
   - kotak chat → `POST /attempts/{id}/ai/chat/stream` (SSE) dengan fallback ke
     `POST /attempts/{id}/ai/chat` bila streaming tidak tersedia;
   - riwayat percakapan dipulihkan lewat `GET /attempts/{id}/ai/conversation`;
   - tombol pertanyaan cepat memakai `suggestions` dari balasan analisis.

Contoh pemakaian SSE di klien:

```ts
const res = await fetch(`${API}/attempts/${id}/ai/chat/stream`, {
  method: "POST",
  headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
  body: JSON.stringify({ message, questionId }),
});

const reader = res.body!.getReader();
const decoder = new TextDecoder();
let buffer = "";

for (;;) {
  const { value, done } = await reader.read();
  if (done) break;
  buffer += decoder.decode(value, { stream: true });

  const frames = buffer.split("\n\n");
  buffer = frames.pop() ?? "";
  for (const frame of frames) {
    const event = frame.match(/^event: (.+)$/m)?.[1];
    const data = JSON.parse(frame.match(/^data: (.+)$/m)?.[1] ?? "{}");
    if (event === "delta") appendToBubble(data.delta);
    if (event === "done") finish(data.message, data.usage);
    if (event === "error") showError(data.error);
  }
}
```

Catatan: `answerIndex`/`explanation` tidak tersedia selama sesi berjalan — itu
memang disengaja. Pembahasan lengkap dikirim pada balasan `submit` dan
`GET /attempts/{id}`.

---

## 11. Konfigurasi

Seluruh variabel ada di `.env.example`. Yang paling sering disesuaikan:

| Variabel | Default | Keterangan |
| --- | --- | --- |
| `HTTP_PORT` | `8080` | port API |
| `DB_MAX_OPEN_CONNS` | `60` | batas koneksi PostgreSQL |
| `DB_PREPARE_STMT` | `true` | prepared statement GORM |
| `DB_AUTO_MIGRATE` | `true` | jalankan migrasi saat start |
| `REDIS_ENABLED` | `true` | matikan bila tidak memakai Redis |
| `CACHE_QUESTION_BANK_TTL` | `15m` | TTL cache bank soal |
| `JWT_SECRET` | — | **wajib ≥ 32 karakter di produksi** |
| `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` | `24h` / `720h` | masa berlaku token |
| `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` | `30` / `60` | batas per pengguna/IP |
| `PASSING_THRESHOLD` | `70` | ambang batas kelulusan simulasi |
| `REFERRAL_DISCOUNT_PERCENT` | `10` | diskon kode referral |
| `MIDTRANS_SERVER_KEY` | — | Server Key Midtrans (kosong = simulator sandbox lokal) |
| `MIDTRANS_CLIENT_KEY` | — | Client Key Midtrans (dipakai snap.js di browser) |
| `MIDTRANS_IS_PRODUCTION` | `false` | `true` untuk kredensial produksi |
| `MIDTRANS_SANDBOX_SIMULATOR` | `true` | simulator pembayaran lokal (wajib `false` di produksi) |
| `MIDTRANS_TIMEOUT` | `20s` | batas waktu permintaan ke Midtrans |
| `MIDTRANS_SNAP_BASE_URL` / `MIDTRANS_CORE_BASE_URL` | — | override domain Midtrans (pengujian/proxy) |
| `FRONTEND_BASE_URL` | `http://localhost:3000` | basis URL pengalihan Snap |
| `MEDIA_DIR` | `./media` | direktori berkas media ruang kelas (`/app/media` di Docker) |
| `SEED_ON_START` | `true` | matikan di produksi |
| `AI_PROVIDER` | `stub` | `stub` (lokal), `ark`, atau `openai` |
| `AI_BASE_URL` | Ark ap-southeast-1 | endpoint kompatibel OpenAI |
| `AI_API_KEY` | — | wajib bila provider bukan `stub` |
| `AI_MODEL` | `deepseek-v4-flash-260425` | model yang sudah diaktifkan |
| `AI_FALLBACK_ENABLED` | `true` | alihkan ke mentor lokal saat provider gagal |
| `AI_DAILY_QUOTA` | `120` | batas permintaan AI per peserta per hari |
| `AI_MAX_CONTEXT_QUESTIONS` | `40` | batas soal yang dikirim sebagai konteks |
| `AI_MAX_HISTORY_MESSAGES` | `12` | batas riwayat percakapan pada prompt |
| `AI_STREAM_ENABLED` | `true` | aktifkan endpoint SSE |
| `AI_ANALYSIS_CACHE_TTL` | `168h` | masa pakai analisis sebelum dihitung ulang |

---

## 12. Catatan produksi

- Set `APP_ENV=production` → `JWT_SECRET` wajib ≥ 32 karakter dan log memakai
  format JSON.
- Set `SEED_ON_START=false` agar data demo tidak ikut terisi.
- Jalankan migrasi sebagai langkah deploy terpisah (`go run ./cmd/migrate up`)
  bila ingin kontrol penuh, lalu set `DB_AUTO_MIGRATE=false`.
- Gunakan TLS di depan API (reverse proxy) dan set `DB_SSLMODE=require`.
- Sesuaikan `DB_MAX_OPEN_CONNS` dengan `max_connections` PostgreSQL dibagi jumlah
  replika API.
- `POST /webhooks/midtrans` saat ini memproses notifikasi secara idempotent;
  tambahkan verifikasi `signature_key` memakai `MIDTRANS_SERVER_KEY` sebelum
  dipakai dengan akun Midtrans sungguhan.
- Broadcast WhatsApp hanya mencatat penerima & log; pengiriman nyata perlu worker
  yang memanggil penyedia pesan.
- **Mentor AI:** aktifkan model pada konsol penyedia lalu isi `AI_API_KEY` dan
  `AI_MODEL`. Sebelum itu, biarkan `AI_PROVIDER=stub` — fitur tetap berjalan
  penuh dengan mesin heuristik lokal dan tanpa biaya API. Pantau
  `GET /ai/status` (`degraded`, `lastError`) untuk mendeteksi provider bermasalah,
  dan sesuaikan `AI_DAILY_QUOTA` dengan anggaran yang tersedia.

## 13. Troubleshooting

**`no required module provides package github.com/bumn-academy/be-bumn-academy/internal/seed`**
Build context tidak memuat berkas sumber `internal/seed`, `cmd/seed`, atau
`cmd/migrate`. Penyebab paling umum: pola `.gitignore` **tanpa garis miring**
(`seed`, `migrate`) — pola seperti itu cocok di semua level direktori, sehingga
direktori sumber bernama sama ikut terabaikan dan tidak pernah ikut ter-commit.
Akibatnya build dari hasil clone/arsip git (mis. `docker compose up --build` di
mesin bersih, CI, atau build Docker dari repositori Git) gagal, walau di mesin
yang berkasnya masih ada build berjalan normal.

Perbaikan (sudah diterapkan sejak commit `7f350a3`):

```bash
grep -n 'seed\|migrate' .gitignore      # harus ber-anchor: /seed, /migrate
git ls-files internal/seed cmd/seed cmd/migrate   # harus menampilkan 5 berkas
git pull                                # bila memakai hasil clone lama
docker compose up -d --build
```

**`role "bumn" does not exist` / tersambung ke PostgreSQL lain**
Ada PostgreSQL lokal di port 5432. Docker Compose proyek ini memakai port host
`55432`; pastikan `DB_PORT=55432` di `.env`.

**`migrate: gagal menyiapkan tabel schema_migrations`**
Database belum siap. Tunggu `docker compose ps` menampilkan `healthy`, atau
periksa kembali `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`.

**`redis tidak tersedia — cache dilewati`**
Bukan error fatal: API tetap jalan tanpa cache. Set `REDIS_ENABLED=false` bila
memang tidak memakai Redis.

**Tryout tidak bisa dimulai (`FORBIDDEN` / `LOCKED`)**
Paket premium memerlukan produk tertentu (`requiredProduct`) dan setiap paket
punya kuota percobaan (`attempts`). Selesaikan pembelian atau pakai akun peserta
demo yang sudah memiliki `simulasi-evaluasi`.

**Mentor AI menjawab dari mesin lokal, bukan model sungguhan**
Periksa `GET /ai/status`. Bila `provider: "stub"`, penyebabnya salah satu dari:
`AI_PROVIDER` masih `stub`, `AI_API_KEY` kosong, atau model belum diaktifkan pada
akun penyedia. Bila `degraded: true`, lihat `lastError` — pesan seperti
`ModelNotOpen` berarti model harus diaktifkan dulu di konsol Ark.

**`POST /ai/chat/stream` langsung berhenti tanpa balasan**
Pastikan `AI_STREAM_ENABLED=true` dan reverse proxy tidak menahan respons —
backend sudah mengirim header `X-Accel-Buffering: no` dan `Cache-Control:
no-transform` untuk itu. Endpoint non-streaming (`POST /ai/chat`) tetap bisa
dipakai sebagai fallback.

**`429 RATE_LIMITED` saat memakai mentor AI**
Kuota harian peserta habis (`AI_DAILY_QUOTA`) atau rate limit umum terpicu.
Naikkan `AI_DAILY_QUOTA`, atau biarkan analisis (yang di-cache per attempt)
sebagai sumber utama dan chat sebagai pelengkap.
