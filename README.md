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

> **Dokumen perencanaan produk** (di luar repo ini) ada di `../docs/`:
> `tindak-lanjut-pitching.md` (rencana seluruh masukan klien, termasuk status
> Fase 0) dan spesifikasi per fase — `fase-1-elearning-kelas-khusus.md`,
> `fase-2-kpi-kelulusan-bumn.md`, `fase-3-talent-mapping-psikolog.md`.

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
| 011 | `questions.tryout_id` — bank soal dikelompokkan per paket tryout (backfill menggandakan soal yang dipakai beberapa tryout) |
| 012 | taksonomi soal: `sector_key`/`company_key`/`test_type`/`position_family`, `variant_group`, `retention_until` |
| 013 | `question_licenses` — izin/hak pakai soal (scope, pemilik, masa berlaku, penggunaan komersial) |
| 014 | keunikan soal: `questions.fingerprint` + `question_access_logs` |
| 015 | `learning_paths`, `learning_path_steps`, `lesson_quizzes`, `lesson_quiz_questions`, `quiz_attempts`, `certificates`, `certificate_counters` (+ `course_modules.sector_key`/`test_type`, `lessons.is_preview`) |

Data demo di-embed dari `internal/seed/data.json` yang dihasilkan dari modul mock
front-end (45 soal, 5 paket tryout, 4 produk, 4 program, 3 voucher, 6 rekaman),
sehingga angka soal/menit pada kartu tryout **sama persis** dengan front-end.

Materi ruang kelas digital diambil dari `internal/seed/classroom.json` (8 modul,
15 materi) dengan tautan video mengarah ke berkas di `media/lessons/`. Berkas itu
dibuat oleh `scripts/generate-media.sh` (Pillow merender slide, ffmpeg mengemas
MP4 H.264 1280×720 + poster) sehingga peserta benar-benar memutar video asli.
Direktori media dilayani pada `/api/v1/media/*` (lihat `MEDIA_DIR`).

Selain katalog, `internal/seed/hr.go` menyiapkan **kohort demo analitik HR** agar
dasbor `/admin/hr` langsung penuh data: 10 peserta lintas segmen program
(`users.program`), 36 percobaan tryout tersebar dalam enam bulan beserta rincian
skor per subtes, dan 51 baris progres ruang kelas. Seluruh akun kohort memakai
kata sandi `peserta123` (mis. `dewi.lestari@bumnacademy.id`). Akun inti
`peserta@bumnacademy.id` **sengaja tidak** diberi percobaan/progres agar skrip
verifikasi (`smoke-test.sh`, `verify-integration.sh`) tetap berlaku.

`internal/seed/learning_path.go` menambahkan **jalur belajar & kuis demo** (Fase 1):
satu jalur `jalur-tkd-dasar` pada produk `kelas-intensif` berisi tiga modul (dua
wajib, satu opsional) dan satu kuis `Kuis Deret & Pola` pada pelajaran
`les-numerik-deret` (3 soal bank `numerik`, ambang lulus 70, maksimum 3 percobaan).
Soal kuis dipilih dari `data.json` (bukan id yang ditulis tangan) supaya tetap
stabil bila penamaan id soal berubah.

### Isi paket `internal/seed`

| Berkas | Isi |
| --- | --- |
| `seed.go` | orkestrasi `seed.Run(ctx, db, cfg)` + upsert seluruh katalog memakai UUID deterministik (`uuid.NewSHA1`) agar seed berulang tidak membuat baris baru |
| `data.go` | struktur JSON + `//go:embed data.json classroom.json` |
| `learning.go` | jadwal kelas live (disimpan sebagai selisih jam dari waktu seed) & 6 rekaman kelas |
| `learning_path.go` | jalur belajar & kuis demo Fase 1 (langkah jalur + susunan soal kuis) |
| `hr.go` | kohort demo analitik HR: akun peserta lintas segmen, riwayat percobaan tryout (dengan rincian subtes), akses produk, dan progres ruang kelas |
| `data.json` | program, kategori materi, produk, metode bayar, voucher, 45 definisi soal (digandakan per tryout saat disisipkan), 5 paket tryout, akun demo, enrollment demo |
| `classroom.json` | 8 modul & 15 materi (6 video dari `media/lessons/` + 9 e-book yang dirender jadi PDF saat dibuka) |
| `seed_test.go` | uji kontrak data: jumlah data, id unik, bank soal tiap subtes, produk wajib tiap paket, keberadaan berkas media, determinisme `stableID` |
| `hr_test.go` | uji kontrak kohort analitik HR: sebaran program, akun kohort tidak menimpa akun inti, rentang enam bulan, peserta tanpa percobaan, validitas rotasi tryout & pola progres |

Seed dipanggil otomatis saat start (`SEED_ON_START=true`) maupun manual lewat
`make seed` / `go run ./cmd/seed`. Seluruh operasi idempoten (upsert), sehingga
aman dijalankan berulang — termasuk saat beberapa replika API start bersamaan.

### Bank soal per paket tryout (migrasi 011)

Sejak migrasi `000011_question_tryout`, satu soal **dimiliki satu paket tryout**:

- `questions.tryout_id` → FK ke `tryout_packages` (`ON DELETE CASCADE`): menghapus
  paket tryout ikut menghapus soalnya.
- `questions.bank_key` tetap menentukan **subtes** mana yang mengambil soal (lewat
  `subtests.banks`), sehingga penyusunan sesi mencari
  `tryout_id = <paket> AND bank_key IN (banks subtes)`.
- Soal yang bank-nya dipakai beberapa tryout **digandakan** ke tiap tryout.
  Migrasi 011 melakukannya untuk data lama (baris asli dipertahankan, tryout lain
  mendapat salinan ber-id baru) dan seed melakukannya untuk data demo — karena itu
  45 definisi soal di `data.json` menjadi **103 baris** di database, sementara
  jumlah soal tiap tryout tetap sama seperti sebelum perubahan (22/26/30/11/14).
- `tryout_id` boleh `NULL` untuk soal warisan yang bank-nya tidak dipakai tryout
  mana pun; soal itu tampil di panel admin sebagai “belum ditautkan” dan tidak
  dipakai sesi mana pun.
- Seed menguasai seluruh soal pada paket tryout demo: baris di luar himpunan
  kanonik (mis. hasil penggandaan migrasi, atau soal yang dibuat admin pada tryout
  demo) dibersihkan agar hasilnya sama setiap kali dijalankan.

Konsekuensi praktis: mengubah teks soal berarti mengubahnya **per tryout**
(salinan tidak ikut berubah otomatis), dan `GET /admin/questions` menerima filter
`?tryoutId=` untuk menampilkan soal satu paket.

### Taksonomi soal, izin, dan keunikan (migrasi 012–014)

Masukan klien saat pitching melahirkan tiga kemampuan baru pada bank soal:

| Migrasi | Isi |
| --- | --- |
| `000012_question_taxonomy` | `questions.sector_key` (klaster BUMN), `company_key`, `test_type` (jenis tes), `position_family`, `variant_group`, `retention_until` + backfill `test_type` dari kategori bank |
| `000013_question_license` | tabel `question_licenses` (izin/hak pakai soal) & `psychological_instruments`; backfill izin `internal` untuk seluruh soal lama |
| `000014_question_uniqueness` | tabel `question_fingerprints` (sidik jari teks) & `question_access_logs` (jejak audit) |

**Kategori BUMN.** Soal ditandai menurut 12 klaster BUMN (mis. `jasa_keuangan`,
`pertahanan_manufaktur`) dan jenis tesnya mengikuti tahapan Rekrutmen Bersama
BUMN: `tkd`, `akhlaq`, `twk`, `bing`, `learning_agility`, `tkb`, `psikotes`.
Daftar pilihan dikirim backend lewat `GET /admin/questions/taxonomy` agar label di
panel admin tidak pernah berbeda dari sumbernya di `internal/pkg/bank`.

**Izin soal (fail-closed).** Setiap soal wajib punya baris izin berisi cakupan
(`internal`/`vendor`/`public_domain`/`client_owned`), pemilik hak cipta, nomor
perjanjian, berkasnya, serta izin pemakaian komersial & asesmen. Bila
`QUESTION_LICENSE_ENFORCE=true` (bawaan), soal tanpa izin aktif **tidak dikirim
ke peserta** — `TryoutService.bankQuestions` menyaringnya sebelum sesi disusun,
sehingga kelalaian melengkapi izin berhenti sebagai soal yang tidak terpakai,
bukan sebagai pelanggaran perjanjian. Soal lisensi `vendor` wajib mencantumkan
nomor perjanjian (ditolak di create maupun import). `GET /admin/licenses/summary`
melaporkan soal yang izinnya belum lengkap dan izin yang akan berakhir ≤ 30 hari.

**Instrumen psikologi.** `psychological_instruments` membedakan psikotes
`latihan` (boleh dilatihkan peserta) dan `berlisensi` (hanya boleh dijalankan
psikolog berkompeten). Instrumen berlisensi wajib punya vendor, nomor lisensi,
dan psikolog penanggung jawab — tanpa itu instrumen tidak bisa dipakai.

**Keunikan soal.** `internal/pkg/fingerprint` menyimpan tiga tingkat sidik jari
teks: hash ternormalisasi (kembar persis), template hash (sama kecuali angka),
dan himpunan kata untuk kemiripan Jaccard (ambang 0,75). Dipakai untuk:
`POST /admin/questions/check-duplicate` sebelum menyimpan, temuan `duplicates`
pada hasil import, dan laporan `GET /admin/compliance/question-uniqueness`.
Soal yang sengaja dijadikan varian (`variant_group` yang sama) tidak dianggap
temuan; sesi tryout hanya memakai **satu soal per grup varian**, dipilih
deterministik per (tryout, bank, grup) agar seluruh peserta satu paket
mengerjakan susunan yang sama.

> Catatan data demo: seed menggandakan soal ke setiap tryout yang memakai bank
> yang sama (konsekuensi migrasi 011). Karena itu laporan keunikan pada data demo
> menampilkan pasangan `identik` antar-tryout — itu memang temuan yang benar
> (soal yang sama muncul di dua paket), bukan cacat alat deteksinya.

**Jejak audit.** Setiap soal dibuat/diubah/dihapus/diimpor tercatat di
`question_access_logs` (pelaku, aksi, waktu), bisa dibaca lewat
`GET /admin/compliance/access-log`. Ini yang dipakai saat ada dugaan kebocoran
soal yang terikat perjanjian kerahasiaan.

### Jalur belajar, kuis, & sertifikat (migrasi 015)

Fondasi Fase 1 (e-learning) ada di migrasi `000015_learning_paths`:

| Tabel | Isi |
| --- | --- |
| `learning_paths` / `learning_path_steps` | kurikulum berurutan (langkah = modul, wajib/opsional) dengan dimensi taksonomi Fase 0 |
| `lesson_quizzes` / `lesson_quiz_questions` | kuis per pelajaran; susunan soal adalah **snapshot** pilihan admin (posisi `question_id` menjadi `NULL` bila soalnya dihapus → kuis ditandai perlu ditinjau) |
| `quiz_attempts` | percobaan peserta (skor, benar, lulus, jawaban) — hanya yang sudah dikumpulkan dihitung terhadap `max_attempts` |
| `certificates` / `certificate_counters` | sertifikat penyelesaian jalur + penomoran aman per tahun (`SELECT … FOR UPDATE`) |

Catatan model: `quiz_attempts` dan `certificates` **tidak** memakai
`models.Base` (kolom waktu standar) karena tabelnya memakai kolom waktu domain
sendiri (`started_at`/`finished_at`, `issued_at`); keduanya memakai
`models.IDBase`. Uji `internal/repository/learning_path_schema_test.go` menjaga
kolom model tetap sama dengan migrasi.

### Akun demo

| Role | Email | Kata sandi |
| --- | --- | --- |
| Peserta | `peserta@bumnacademy.id` | `peserta123` |
| Admin | `admin@bumnacademy.id` | `admin123` |

### Kohort demo analitik HR

`internal/seed/hr.go` menambahkan 10 akun peserta (kata sandi `peserta123`) untuk
mengisi dasbor `/admin/hr` dengan data nyata:

| Segmen (`users.program`) | Akun |
| --- | --- |
| Kelas Intensif BUMN | `dewi.lestari@`, `rangga.pratama@`, `rian.kusuma@` |
| Program Reguler | `siti.nurhaliza@`, `bayu.saputra@`, `hana.maulida@` |
| Kelas Korporasi | `intan.permata@`, `fajar.ramadhan@` |
| Alumni & Retaker | `nadia.safira@`, `yoga.pratama@` |

(semua beralamat `@bumnacademy.id`). Tiap akun mendapat akses seluruh produk demo,
lalu sebagian diberi 3–6 percobaan tryout yang tersebar 4–170 hari ke belakang
beserta rincian skor per subtes, dan 3–8 baris progres ruang kelas. `hana.maulida@`
dan `rian.kusuma@` sengaja dibiarkan tanpa percobaan agar metrik "belum pernah
latihan" serta daftar peserta perlu pendampingan terisi.

Semua penulisan memakai UUID deterministik + upsert (termasuk penulisan ulang
rincian subtes per percobaan), sehingga `make seed` berulang tidak menambah baris
baru. Akun inti di atas tidak pernah diberi percobaan/progres supaya skrip
verifikasi tetap sahih.

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
| GET | `/insights/corporate` | publik | insight agregat kohort untuk halaman "Solusi Korporasi" (tanpa data pribadi) |
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

### Jalur belajar & kuis pelajaran (Fase 1)

| Metode | Path | Akses | Keterangan |
| --- | --- | --- | --- |
| GET | `/learning/paths` | peserta | jalur belajar yang bisa diakses + progres ringkas (hanya jalur yang produknya dimiliki) |
| GET | `/learning/paths/{slug}` | peserta | detail jalur: langkah, progres per modul, langkah berikutnya, kelayakan sertifikat |
| GET | `/classroom/lessons/{id}/quiz` | peserta | soal kuis pelajaran (**tanpa** kunci jawaban) + sisa percobaan |
| POST | `/classroom/lessons/{id}/quiz` | peserta | kumpulkan jawaban → skor, lulus/tidak, pembahasan, sisa percobaan |

Aturan kuis: soal diambil dari bank soal dan hanya soal **berizin aktif** yang
dipakai (soal tanpa izin tidak dikirim dan kuisnya ditandai perlu ditinjau); kunci
jawaban baru dibuka setelah kuis dikumpulkan; nilai = benar/jumlah soal × 100 dan
lulus bila `>= passScore`; `maxAttempts` 0 berarti tanpa batas, dan hanya percobaan
yang sudah dikumpulkan yang dihitung. Kuis yang dipakai peserta adalah kuis aktif
**terbaru** pada pelajaran itu.

Progres jalur dihitung dari langkah **wajib** (bila tidak ada langkah wajib, seluruh
langkah): sebuah langkah selesai bila seluruh pelajaran aktif modulnya sudah selesai.

### Sertifikat & verifikasi publik (Fase 1)

| Metode | Path | Akses | Keterangan |
| --- | --- | --- | --- |
| GET | `/certificates` | peserta | sertifikat milik peserta (nomor, jalur, status, tautan unduh & verifikasi) |
| GET | `/certificates/{id}/pdf` | peserta | PDF sertifikat lanskap A4 (`?download=1` memaksa unduhan) |
| POST | `/learning/paths/{slug}/certificate` | peserta | minta penerbitan sertifikat (idempoten); bila syarat belum lengkap → `422` + rincian |
| GET | `/verifikasi/{nomor}` | publik | verifikasi sertifikat: nama, jalur, jenis tes/klaster, instansi, tanggal, status |

Syarat terbit: seluruh langkah **wajib** jalur selesai **dan** seluruh kuis aktif pada
pelajaran modul wajib pernah lulus. Penerbitan otomatis berjalan di dua titik —
setelah progres materi disimpan dan setelah kuis lulus — serta idempoten (satu
sertifikat per peserta per jalur) dan atomik (baris peserta dikunci sebelum nomor
diambil, sehingga nomor `BA-<tahun>-<6 digit>` tidak kembar). Sertifikat yang dicabut
tidak diterbitkan ulang otomatis. Detail kelayakan (mis. kuis yang belum lulus) bisa
dilihat pada `certificateEligibility` di detail jalur.

> Catatan operasional: menambahkan kuis pada pelajaran yang sudah menjadi bagian jalur
> aktif menaikkan syarat sertifikat jalur tersebut (semua kuis wajib lulus).

Tautan verifikasi pada PDF memakai `PUBLIC_BASE_URL` (lihat tabel env).

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

### Asisten HR (AI)

| Metode | Path | Akses | Keterangan |
| --- | --- | --- | --- |
| POST | `/admin/hr/ai/chat` | admin | tanya jawab analitik HR (`message` + `history` opsional) |
| POST | `/admin/hr/ai/chat/stream` | admin | balasan bertahap (Server-Sent Events) |
| GET | `/admin/hr/ai/usage` | admin | kuota asisten AI hari ini |

Percakapan asisten HR **tidak disimpan** di server: konteksnya data kohort
agregat, bukan satu hasil tryout, sehingga riwayat dikirim ulang klien pada
setiap pertanyaan (dibatasi 12 pesan dan disanitasi ulang di server).

### Admin

| Metode | Path | Keterangan |
| --- | --- | --- |
| GET | `/admin/analytics` | ringkasan pengguna, pendapatan, distribusi skor |
| GET | `/admin/hr/analytics` | analitik SDM: partisipasi, kelengkapan belajar, kesiapan, peserta berprestasi, peserta perlu pendampingan |
| GET | `/admin/users` | daftar pengguna + aktivitas |
| GET | `/admin/orders` | semua pesanan (filter status/pencarian) |
| POST | `/admin/orders/{id}/mark-paid` | tandai lunas manual + beri akses |
| GET/POST | `/admin/questions` | daftar/tambah soal (filter `bankKey`, `tryoutId`, `sectorKey`, `testType`, `positionFamily`, `variantGroup`, `difficulty`, `activeOnly`, `search`) |
| GET | `/admin/questions/taxonomy` | pilihan kategori, sektor BUMN, jenis tes, keluarga jabatan + nilai bawaan |
| PATCH/DELETE | `/admin/questions/{id}` | ubah/hapus soal |
| POST | `/admin/questions/import` | import soal dari Excel/CSV (multipart `file`, opsional `tryoutId` tujuan) |
| POST | `/admin/questions/check-duplicate` | periksa soal kembar sebelum menyimpan (`kind`: `identik`/`keluarga`/`mirip`/`unik`) |
| POST | `/admin/questions/{id}/variant` | catat penautan soal ke grup varian di jejak audit |
| GET/POST | `/admin/licenses` | daftar/tambah izin soal |
| PATCH/DELETE | `/admin/licenses/{id}` | ubah/hapus izin soal |
| GET | `/admin/licenses/summary` | soal tanpa izin aktif, izin akan berakhir ≤ 30 hari, rekap per cakupan, status penegakan |
| GET/POST | `/admin/instruments` | daftar/tambah instrumen psikologi |
| PATCH/DELETE | `/admin/instruments/{id}` | ubah/hapus instrumen psikologi |
| GET | `/admin/compliance/question-uniqueness` | laporan soal kembar per bank (filter `bankKey`, `sectorKey`, `testType`, `tryoutId`) |
| GET | `/admin/compliance/access-log` | jejak audit bank soal (filter `questionId`, `actorId`, `action`) |
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
| GET/POST | `/admin/learning/paths` | daftar (termasuk nonaktif) & tambah jalur belajar |
| PATCH/DELETE | `/admin/learning/paths/{id}` | ubah / nonaktifkan jalur (soft delete) |
| POST | `/admin/learning/paths/{id}/steps` | susun langkah jalur (modul, wajib, urutan) |
| GET | `/admin/classroom/quizzes?lessonId=` | daftar kuis sebuah pelajaran (termasuk nonaktif) |
| POST | `/admin/classroom/quizzes` | buat kuis + pilih soal (soal tanpa izin aktif ditolak) |
| PATCH/DELETE | `/admin/classroom/quizzes/{id}` | ubah / nonaktifkan kuis |
| GET | `/admin/classroom/quizzes/{id}/attempts` | rekap percobaan peserta pada satu kuis |
| GET | `/admin/certificates` | rekap sertifikat (filter `search`/`pathId`/`status`) |
| GET | `/admin/certificates/{id}/pdf` | unduh PDF sertifikat peserta mana pun |
| DELETE | `/admin/certificates/{id}` | cabut sertifikat |
| POST | `/admin/certificates/{id}/restore` | pulihkan sertifikat yang salah dicabut |

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

### Insight korporasi (B2B) — `GET /insights/corporate`

Endpoint publik yang menyuplai halaman **“Solusi Korporasi”** di front-end. Semua
angkanya agregat dan di-cache (`insights:corporate`, TTL katalog):

| Field | Isi |
| --- | --- |
| `totalPeserta` / `activePeserta` | akun peserta & yang sudah pernah mengerjakan tryout |
| `totalAttempts` / `distinctTryouts` | jumlah percobaan & paket tryout yang dikerjakan |
| `averageScore` / `topScore` / `passingRate` | rata-rata, skor tertinggi, dan persentase di atas `PASSING_THRESHOLD` |
| `scoreDistribution` | jumlah percobaan per rentang nilai |
| `skillProfile` | rata-rata skor per kategori/subtes (dari `attempt_categories`) |
| `topPerformers` | 8 skor teratas dengan **nama disamarkan** (`MaskName`: “I*** P***”) |
| `monthlyTrend` | jumlah percobaan & rata-rata skor 6 bulan terakhir |

Privasi: nama peserta tidak pernah keluar apa adanya (hanya huruf pertama tiap
kata + `***`), tidak ada `userId`, email, atau nomor telepon pada balasan, dan
halaman front-end tidak meminta data pribadi. Penyamaran diuji unit
(`TestMaskNameTidakMembocorkanNama`) dan diverifikasi di skrip integrasi.

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

### 7.1 Asisten HR — tanya jawab analitik SDM

Dasbor analitik HR (`GET /admin/hr/analytics`) punya pendamping percakapan: tim
HR bisa menanyakan angka, penyebab, dan tindak lanjut tanpa berpindah halaman.

```
POST   /admin/hr/ai/chat          → { message, history? } → { message, usage, degraded, suggestions }
POST   /admin/hr/ai/chat/stream   → versi SSE (delta → done)
GET    /admin/hr/ai/usage         → kuota harian admin
```

Perbedaan penting dari mentor:

| Aspek | Mentor ("MinSmart") | Asisten HR |
| --- | --- | --- |
| Cakupan data | satu hasil tryout milik peserta | agregat kohort (11 peserta demo) |
| Riwayat percakapan | disimpan di server (`ai_conversations`) | dikirim klien, tidak disimpan |
| Blok konteks | `<KONTEKS_HASIL_TRYOUT>` | `<DATA_ANALITIK_HR>` |
| Identitas peserta | hanya dirinya sendiri | nama pada daftar berprestasi & pendampingan |
| Endpoint | `/attempts/{id}/ai/*` | `/admin/hr/ai/*` (wajib admin) |

Fakta yang dikirim ke model identik dengan yang tampil di dasbor: headcount &
partisipasi, kesiapan (rata-rata, ambang, tingkat di atas ambang), kelengkapan
belajar, sebaran skor, profil kompetensi per kategori, tren enam bulan, capaian
per program, serta daftar peserta berprestasi dan perlu pendampingan. Konteks
dibangun **ulang setiap permintaan** sehingga jawaban selalu mengikuti data
terbaru.

Pagar pengaman yang berlaku: ambang hanya ambang latihan internal (bukan
prediksi kelulusan), kunci jawaban/isi soal tidak pernah dibocorkan, data
pribadi (email, telepon, unit kerja) tidak tersedia di konteks, dan instruksi di
dalam pesan pengguna yang mencoba menimpa aturan akan diabaikan. Riwayat dari
klien dibatasi 12 pesan, disanitasi, dan penanda blok data dinetralkan agar
tidak bisa dipalsukan.

Pada provider `stub` (default), asisten HR menjawab dari heuristik lokal:
maksud pertanyaan dipetakan ke bagian data yang relevan (partisipasi, kesiapan,
kelengkapan belajar, profil kompetensi, program, tren, sebaran, peserta
berprestasi, peserta perlu pendampingan) dengan angka nyata dari basis data.

```json
// POST /admin/hr/ai/chat
{ "message": "berapa peserta yang belum berlatih?", "history": [] }

// 200 OK
{
  "message": { "role": "assistant", "content": "### Partisipasi latihan\n\n- Peserta terdaftar: **11**\n…" },
  "usage": { "requests": 1, "dailyQuota": 120, "remaining": 119 },
  "degraded": false,
  "suggestions": ["Ringkas kondisi kesiapan peserta dan soroti angkanya", "…"]
}
```

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

Ditambah pengujian asisten HR: blok `<DATA_ANALITIK_HR>` bolak-balik
(render → parse), turunan fakta (kategori terlemah/terkuat, program terbesar,
peserta belum berlatih), pemetaan DTO analitik → fakta prompt, penyaringan
riwayat dari klien, serta heuristik stub untuk tiap maksud pertanyaan —
termasuk penjagaan agar kata kunci bertabrakan tidak salah tangkap
(mis. "siapa" tidak dianggap "siap").

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
| `QUESTION_LICENSE_ENFORCE` | `true` | tegakkan izin soal: soal tanpa izin aktif tidak dikirim ke peserta |
| `REFERRAL_DISCOUNT_PERCENT` | `10` | diskon kode referral |
| `MIDTRANS_SERVER_KEY` | — | Server Key Midtrans (kosong = simulator sandbox lokal) |
| `MIDTRANS_CLIENT_KEY` | — | Client Key Midtrans (dipakai snap.js di browser) |
| `MIDTRANS_IS_PRODUCTION` | `false` | `true` untuk kredensial produksi |
| `MIDTRANS_SANDBOX_SIMULATOR` | `true` | simulator pembayaran lokal (wajib `false` di produksi) |
| `MIDTRANS_TIMEOUT` | `20s` | batas waktu permintaan ke Midtrans |
| `MIDTRANS_SNAP_BASE_URL` / `MIDTRANS_CORE_BASE_URL` | — | override domain Midtrans (pengujian/proxy) |
| `FRONTEND_BASE_URL` | `http://localhost:3000` | basis URL pengalihan Snap |
| `MEDIA_DIR` | `./media` | direktori berkas media ruang kelas (`/app/media` di Docker) |
| `PUBLIC_BASE_URL` | `https://bumnacademy.id` | alamat situs publik untuk tautan verifikasi sertifikat pada PDF |
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

**`lookup postgres on 127.0.0.53:53: read: connection refused` (ba-api restart terus)**
Container tidak bisa me-resolve nama service `postgres`/`redis`: `/etc/resolv.conf`
di dalam container memakai stub resolver host (systemd-resolved `127.0.0.53`),
padahal alamat itu tidak ada di dalam network namespace container. Container yang
sehat memakai DNS internal Docker di **`127.0.0.11`**.

**Langkah 1 — diagnosa (30 detik)**

```bash
make docker-dns-check      # resolv.conf container, isi daemon.json, resolusi nama service
# atau manual:
docker run --rm alpine cat /etc/resolv.conf       # cari "nameserver 127.0.0.11"
cat /etc/docker/daemon.json 2>/dev/null           # cari entri "dns": ["127.0.0.53", ...]
readlink -f /etc/resolv.conf                      # biasanya .../systemd/resolve/stub-resolv.conf
```

**Langkah 2 — perbaiki DNS Docker di host (solusi paling benar)**

```bash
# hapus entri "dns" yang menunjuk 127.0.0.53 dari /etc/docker/daemon.json, lalu:
sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf   # hindari stub 127.0.0.53
sudo systemctl restart systemd-resolved docker
docker compose up -d --force-recreate api
```

Pada host yang menjalankan Docker di dalam container/LXC (netfilter dibatasi),
DNS internal Docker sering tidak bisa dipakai — lanjut ke Langkah 3.

**Langkah 3 — jalankan tanpa DNS sama sekali (nama service lewat `/etc/hosts`)**

```bash
make docker-up-dns-workaround
# ekuivalen dengan:
docker compose -f docker-compose.yml -f docker-compose.host-gateway.yml up -d --build
```

Override `docker-compose.host-gateway.yml` memetakan `postgres`/`redis` ke gateway
host lewat `extra_hosts`, sehingga koneksi memakai port publikasi host
(`POSTGRES_HOST_PORT`/`REDIS_HOST_PORT`, default `55432`/`56379`) dan **tidak
membutuhkan resolusi nama apa pun**.

**Langkah 4 — fallback terakhir: jaringan host** (bila port publikasi antar
container juga diblokir)

```bash
docker compose up -d postgres redis
docker run -d --name ba-api --restart unless-stopped --network host   -e APP_ENV=production -e HTTP_PORT=8080 -e MEDIA_DIR=/app/media   -e DB_HOST=127.0.0.1 -e DB_PORT=55432 -e DB_USER=bumn -e DB_PASSWORD=bumn   -e DB_NAME=bumn_academy -e DB_SSLMODE=disable -e DB_AUTO_MIGRATE=true   -e REDIS_ENABLED=true -e REDIS_ADDR=127.0.0.1:56379   -e JWT_SECRET=ganti-dengan-secret-acak-minimal-32-karakter   -e SEED_ON_START=true be-bumn-academy-api
```

Container ini memakai network namespace host sehingga menyambung ke PostgreSQL dan
Redis lewat `127.0.0.1` (port publikasi di atas) — sama seperti menjalankan
backend langsung di host.

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
