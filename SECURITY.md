# be-bumn-academy — Audit Keamanan & Pedoman Hardening

Dokumen ini merangkum hasil audit keamanan mandiri atas backend **`be-bumn-academy`**
(Go + chi + PostgreSQL + Redis) beserta interaksinya dengan front-end
**`bumn-academy`** (Next.js). Seluruh temuan di bawah berasal dari tinjauan kode
statis dan belum diperbaiki — daftar ini dipakai sebagai backlog perbaikan.

| | |
|---|---|
| **Tanggal audit** | 26 Oktober 2026 |
| **Commit acuan** | `29d4740` (backend), front-end pada kondisi kerja saat audit |
| **Cakupan** | Konfigurasi, middleware, autentikasi/otorisasi, handler, repository, migrasi, Docker & deployment |
| **Metode** | Tinjauan kode statis + verifikasi langsung terhadap berkas sumber |

## Cara melaporkan kerentanan

Jangan membuka issue publik untuk kerentanan yang belum diperbaiki. Kirim laporan
ke kanal keamanan internal (Slack privat / surel tim) yang memuat: deskripsi,
langkah reproduksi, dampak, dan usulan perbaikan. Beri waktu perbaikan sebelum
memublikasikan.

---

## 1. Ringkasan prioritas

| Prioritas | Jumlah | Ringkasan |
|---|---|---|
| 🔴 **P0 — Kritis** | 3 | Endpoint pelunasan pesanan terbuka tanpa pembayaran; konfigurasi produksi masih memakai setelan demo; secret hardcoded di `docker-compose.yml` |
| 🟠 **P1 — Tinggi** | 6 | Rate limit dapat dilewati; tanpa security header; media publik tanpa auth; `/readyz` membocorkan detail internal; access token 24 jam & logout tidak mencabut token; tanpa audit log |
| 🟡 **P2 — Sedang** | 8 | Cookie sesi tidak bertanda tangan; tanpa CSRF/cek `Origin`; tanpa deteksi reuse refresh token; validasi berkas import minim; prompt injection AI; Redis tanpa password; tanpa CI/scanning; port infra terbuka |

Urutan pengerjaan yang disarankan ada di [§8](#8-rencana-perbaikan-bertahap).

---

## 2. Kondisi saat ini yang sudah baik

Pertahankan bagian ini agar tidak mundur saat refactor.

| Aspek | Status | Lokasi |
|---|---|---|
| Hash kata sandi | ✅ bcrypt (`DefaultCost`) dengan salt acak | `internal/pkg/password/password.go` |
| JWT | ✅ HS256, `WithValidMethods`, issuer divalidasi, `nbf/iat/exp/jti` | `internal/auth/jwt.go:97-113` |
| Refresh token | ✅ acak 32 byte, disimpan sebagai hash SHA-256, rotasi + revoke | `internal/auth/jwt.go:129-142`, `internal/service/auth.go` (`Refresh`) |
| Anti user-enumeration login | ✅ pesan error disamakan untuk email tak dikenal & sandi salah | `internal/service/auth.go` (`Login`) |
| Validasi input | ✅ `DisallowUnknownFields`, validator, body dibatasi 1 MiB | `internal/handler/handler.go:22-23,42-75` |
| Path traversal media | ✅ tolak `..`/path absolut, allowlist ekstensi, `Content-Type` dipaksa | `internal/handler/classroom.go:260-300` |
| Verifikasi webhook Midtrans | ✅ SHA512 + `subtle.ConstantTimeCompare` | `internal/pkg/midtrans/midtrans.go:128-141` |
| Otorisasi admin | ✅ middleware `Required` + `Admin` pada grup `/admin` | `internal/router/router.go:242-244` |
| Kepemilikan pesanan | ✅ `order.UserID != userID` → 403 | `internal/service/billing.go:258-267` |
| Idempotensi pelunasan | ✅ `settleOrder` aman dipanggil berulang | `internal/service/billing.go:284-292` |
| SQL | ✅ seluruh raw query memakai parameter binding (tanpa interpolasi) | `internal/repository/*.go` |
| Rahasia tidak bocor ke log | ✅ `Describe()` & klien AI tidak mencetak kunci | `internal/pkg/midtrans/midtrans.go`, `internal/ai/openai.go:305` |
| Cookie token (front-end) | ✅ `httpOnly`, `Secure` di produksi, `SameSite=Lax` | `bumn-academy/src/lib/auth/session.ts` |
| Docker | ✅ multi-stage build, proses berjalan sebagai user non-root | `Dockerfile:20,32` |
| Secrets di git | ✅ `.env` tidak ter-commit (hanya `.env.example`) | `.gitignore:11-13` |

---

## 3. Temuan kritis (P0)

### 3.1 `POST /orders/{id}/pay` membuka akses kelas berbayar tanpa pembayaran

| | |
|---|---|
| **Lokasi** | `internal/router/router.go:191`, `internal/service/billing.go:258-282` |
| **Klasifikasi** | CWE-602 (Enforcement aturan bisnis di sisi klien), OWASP A04:2021 Insecure Design |

**Deskripsi.** Endpoint `POST /orders/{id}/pay` dipasang pada grup peserta (hanya
butuh login) dan langsung menandai pesanan milik pemanggil sebagai **lunas**
(`settleOrder`) tanpa memverifikasi pembayaran apa pun. Berbeda dengan
`POST /orders/{id}/sandbox/settle` yang dijaga `SandboxSimulatorAllowed()`
(`internal/config/config.go:170-175`), endpoint ini **tidak memiliki gate sama sekali**.

```go
// internal/service/billing.go:258
func (s *BillingService) PayOrder(ctx context.Context, userID, orderID string) (*dto.PayOrderResponse, error) {
	order, err := s.orders.FindByID(ctx, orderID)
	...
	paid, err := s.settleOrder(ctx, order, "manual-"+order.InvoiceNo) // ← lunas + grant akses
```

**Dampak.** Setiap peserta yang login dapat membuka seluruh kelas berbayar secara
gratis (kebocoran pendapatan langsung), dan pemberian akses tercatat sebagai
pembayaran sah sehingga menyulitkan rekonsiliasi.

**Perbaikan.**
1. Hapus rute di produksi, atau aktifkan hanya bila simulator sandbox diizinkan:

```go
// internal/router/router.go — dekat baris 191
if opts.Config.SandboxSimulatorAllowed() {
	r.Post("/orders/{id}/pay", h.billing.PayOrder)
}
```

2. Tambahkan guard di lapisan service (defense in depth):

```go
func (s *BillingService) PayOrder(ctx context.Context, userID, orderID string) (*dto.PayOrderResponse, error) {
	if !s.cfg.SandboxSimulatorAllowed() {
		return nil, apperror.Forbidden("Pembayaran manual tidak tersedia. Selesaikan lewat kanal pembayaran resmi.")
	}
	...
}
```

**Verifikasi.** Tambahkan unit test di `internal/service/billing_test.go` yang
memastikan `PayOrder` mengembalikan `FORBIDDEN` saat `MIDTRANS_SANDBOX_SIMULATOR=false`.

---

### 3.2 Konfigurasi produksi masih memakai setelan demo (`.env`)

| | |
|---|---|
| **Lokasi** | `.env`, `internal/config/config.go:224,236,263,290-307`, `internal/seed/seed.go:113-118` |
| **Klasifikasi** | CWE-1188 (Default/Weak Credentials), OWASP A05:2021 Security Misconfiguration |

**Deskripsi.** Berkas `.env` pada lingkungan ini memakai `APP_ENV=production`
sementara beberapa nilai masih bernuansa demo:

| Variabel | Nilai teramati | Risiko |
|---|---|---|
| `SEED_ON_START` | `true` | Membuat akun demo termasuk admin ber-kata-sandi lemah |
| `MIDTRANS_SANDBOX_SIMULATOR` | `true` | Endpoint simulator pembayaran aktif di env "production" |
| `JWT_SECRET` | panjang 29 karakter | Di bawah minimum 32 yang divalidasi sendiri (`config.go:297`) |
| `DB_AUTO_MIGRATE` | `true` | Migrasi otomatis dijalankan saat start |

Kredensial demo didokumentasikan terbuka di `README.md:202-203`
(`admin@bumnacademy.id` / `admin123`), dan `seedUsers` **selalu** me-hash ulang
kata sandi tersebut setiap seed dijalankan, sehingga akun itu pasti dapat dipakai.

**Dampak.**
- **Pengambilalihan akun admin penuh** bila seed berjalan di lingkungan yang
  terjangkau publik (kredensial publik + panel `/admin` lengkap: data pengguna,
  pesanan, bank soal, voucher, broadcast).
- **JWT secret lemah/pendek** memungkinkan pemalsuan token admin (full takeover)
  dan/atau membuat aplikasi gagal start (validasi produksi menolak < 32 karakter).
- Simulator pembayaran aktif → pesanan dapat dilunasi tanpa uang sungguhan.

**Perbaikan.**
1. Bangkitkan secret kuat (`openssl rand -hex 32`), simpan di secret manager /
   env runtime (bukan berkas yang ikut ter-deploy).
2. Set di produksi: `SEED_ON_START=false`, `DB_AUTO_MIGRATE=false`,
   `MIDTRANS_SANDBOX_SIMULATOR=false`, `DB_SSLMODE=require` (atau `verify-full`).
3. Tambahkan guard fail-fast di `internal/config/config.go` (`validate()`):

```go
if c.IsProduction() {
	if len(c.Auth.JWTSecret) < 32 {
		return fmt.Errorf("config: JWT_SECRET minimal 32 karakter di produksi")
	}
	if strings.Contains(c.Auth.JWTSecret, "ganti-dengan-secret") ||
		c.Auth.JWTSecret == "dev-secret-jangan-dipakai-di-produksi-32" {
		return fmt.Errorf("config: JWT_SECRET masih memakai nilai contoh")
	}
	if c.Seed.OnStart {
		return fmt.Errorf("config: SEED_ON_START wajib false di produksi")
	}
	if c.DB.AutoMigrate {
		return fmt.Errorf("config: DB_AUTO_MIGRATE wajib false di produksi")
	}
	if c.SandboxSimulatorAllowed() {
		return fmt.Errorf("config: MIDTRANS_SANDBOX_SIMULATOR wajib false di produksi")
	}
	if strings.EqualFold(c.DB.SSLMode, "disable") {
		return fmt.Errorf("config: DB_SSLMODE tidak boleh 'disable' di produksi")
	}
}
```

4. Ubah default `getBool("SEED_ON_START", true)` (`config.go:263`) dan
   `getBool("DB_AUTO_MIGRATE", true)` (`config.go:224`) menjadi `false`, lalu
   aktifkan eksplisit di `.env` pengembangan.

---

### 3.3 Secret hardcoded & port infrastruktur terbuka di `docker-compose.yml`

| | |
|---|---|
| **Lokasi** | `docker-compose.yml:29,53,78,87,89` |
| **Klasifikasi** | CWE-798 (Hardcoded Credentials), OWASP A05:2021 Security Misconfiguration |

**Deskripsi.** Compose menetapkan `APP_ENV: production` tetapi menuliskan kredensial
secara literal dan mengesankan nilai contoh:

```yaml
environment:
  DB_PASSWORD: bumn
  JWT_SECRET: ganti-dengan-secret-acak-minimal-32-karakter   # ← nilai contoh
  SEED_ON_START: "true"
ports:
  - "${POSTGRES_HOST_PORT:-55432}:5432"   # terikat ke 0.0.0.0
  - "${REDIS_HOST_PORT:-56379}:6379"      # terikat ke 0.0.0.0, tanpa password
```

**Dampak.** Karena `JWT_SECRET` tertulis di repo, siapa pun yang melihat kode dapat
**memalsukan token admin** dan menguasai sistem. PostgreSQL & Redis terbuka ke
jaringan host; Redis tanpa `requirepass` memungkinkan penyalahgunaan cache/rate limit.

**Perbaikan.**

```yaml
environment:
  DB_PASSWORD: ${DB_PASSWORD:?DB_PASSWORD wajib diisi}
  JWT_SECRET: ${JWT_SECRET:?JWT_SECRET wajib diisi}
  SEED_ON_START: "false"
  DB_AUTO_MIGRATE: "false"
ports:
  - "127.0.0.1:${POSTGRES_HOST_PORT:-55432}:5432"
redis:
  command: [ "redis-server", "--requirepass", "${REDIS_PASSWORD:?REDIS_PASSWORD wajib diisi}", ... ]
```

Sediakan `.env.docker.example` (tanpa nilai) + dokumentasi di README, dan pastikan
berkas `.env` nyata tetap di-`.gitignore`.

---

## 4. Temuan tinggi (P1)

### 4.1 Rate limit dapat dilewati lewat header `X-Forwarded-For`

| | |
|---|---|
| **Lokasi** | `internal/middleware/common.go:72-83`, `internal/router/router.go:76` |
| **Klasifikasi** | CWE-290 (Authentication Bypass by Spoofing), OWASP A04:2021 |

**Deskripsi.** `clientKey` membentuk kunci pembatas dari `X-Forwarded-For` /
`X-Real-IP` **tanpa memverifikasi bahwa permintaan memang datang dari proxy
terpercaya**; `chimiddleware.RealIP` juga mempercayai header yang sama. Selain itu
kunci hanya dibuat dari IP pengguna (atau `user:<id>` bila sudah login), sedangkan
`/auth/login` dan `/auth/register` tetap memakai limiter global yang longgar
(`RATE_LIMIT_RPS=30`, `RATE_LIMIT_BURST=60`, `config.go:247-248`).

**Dampak.** Penyerang dapat mengirim nilai `X-Forwarded-For` acak pada setiap
percobaan sehingga **rate limit (dan lockout apa pun) tidak berfungsi** →
credential stuffing / password spraying bebas. Endpoint login juga hanya dibatasi
60 percobaan/detik.

**Perbaikan.**
1. Tambahkan `TRUSTED_PROXIES` (daftar CIDR) di config. Hanya percayai
   `X-Forwarded-For`/`X-Real-IP` bila `r.RemoteAddr` berada di daftar tersebut;
   selain itu gunakan `RemoteAddr` apa adanya. Ganti `chimiddleware.RealIP`
   dengan middleware sendiri yang menerapkan aturan ini.
2. Buat limiter khusus jalur auth (jauh lebih ketat) dan pasang pada grup `/auth`:

```go
// cmd/api/main.go
authLimiter := cache.NewLimiter(cacheClient, cfg.RateLimit.AuthRPS, cfg.RateLimit.AuthBurst)

// internal/router/router.go — grup /auth (baris 149)
r.Group(func(r chi.Router) {
	r.Use(middleware.RateLimit(authLimiter)) // mis. 5 rps / burst 10
	r.Post("/register", h.auth.Register)
	r.Post("/login", h.auth.Login)
	r.Post("/refresh", h.auth.Refresh)
	r.Post("/logout", h.auth.Logout)
	...
})
```

3. Tambahkan **lockout per akun** berbasis Redis dengan backoff eksponensial
   (kunci `auth.HashToken(strings.ToLower(email))`), di-reset saat login sukses,
   dengan pesan error tetap generik agar tidak membocorkan keberadaan email.

---

### 4.2 Tidak ada security header (backend & front-end)

| | |
|---|---|
| **Lokasi** | `internal/router/router.go:75-89`, `bumn-academy/next.config.ts` |
| **Klasifikasi** | CWE-693 (Protection Mechanism Failure), OWASP A05:2021 |

**Deskripsi.** Pencarian menyeluruh atas `X-Frame-Options`,
`Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`,
`Referrer-Policy`, dan `Permissions-Policy` pada kedua repo **tidak menemukan
hasil apa pun**.

**Dampak.** Rentan clickjacking, MIME-sniffing (berbahaya khusus untuk endpoint
`/media/*` yang menyajikan berkas), kebocoran referrer, dan tanpa CSP tidak ada
pertahanan berlapis terhadap XSS.

**Perbaikan (backend).** Tambahkan `internal/middleware/security.go` dan pasang
paling awal pada `router.go`:

```go
package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders menambahkan header pengaman standar pada setiap balasan API.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Cross-Origin-Resource-Policy", "same-site")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
```

**Perbaikan (front-end).** Tambahkan blok `headers()` pada `next.config.ts`
(lihat `bumn-academy/SECURITY.md` untuk CSP lengkap beserta domain Snap Midtrans).

---

### 4.3 Endpoint media publik tanpa autentikasi

| | |
|---|---|
| **Lokasi** | `internal/router/router.go:239`, `internal/handler/classroom.go:260-300` |
| **Klasifikasi** | CWE-284 (Improper Access Control), OWASP A01:2021 |

**Deskripsi.** `/media/*` sengaja publik (agar `<video>`/`<iframe>` mudah memuatnya)
dan URL-nya berpola prediktabel (`/media/lessons/<slug>.mp4`). Tidak ada verifikasi
bahwa pemanggil memiliki enrollment atas materi tersebut.

**Dampak.** Video pelajaran dan e-book berbayar dapat diunduh/dibagikan bebas oleh
siapa pun yang mengetahui/menebak URL, serta dapat di-enumerasi.

**Perbaikan.** Terapkan **signed URL**: sertakan `?exp=<epoch>&sig=<hmac>` di mana
`sig = HMAC-SHA256(secret, "<path>|<exp>")`, diverifikasi di `MediaHandler` dengan
masa berlaku pendek (mis. 15 menit) dan `Cache-Control: private`. Endpoint yang
mengembalikan URL media (lesson, ebook, recordings) yang menandatangani URL.
Alternatif minimal: bungkus `/media/*` dengan `authMid.Required` dan verifikasi
enrollment di handler.

---

### 4.4 `/readyz` membocorkan detail internal

| | |
|---|---|
| **Lokasi** | `internal/handler/health.go:53,56,65` |
| **Klasifikasi** | CWE-209 (Generation of Error Message Containing Sensitive Information) |

**Deskripsi.** `/readyz` mengembalikan `err.Error()` mentah dari database dan Redis
tanpa autentikasi, serta menyertakan `version`.

**Dampak.** Membocorkan informasi topologi/driver (host DB, jenis error koneksi,
versi) yang memudahkan penyerang memetakan sistem.

**Perbaikan.** Log detail via `slog` saja, balas hanya `{"status":"error"}`; sembunyikan
`version` di produksi atau lindungi `/readyz` agar hanya dapat diakses jaringan internal.

---

### 4.5 Access token berumur 24 jam & logout tidak mencabut access token

| | |
|---|---|
| **Lokasi** | `internal/config/config.go:238`, `internal/service/auth.go` (`Logout`) |
| **Klasifikasi** | CWE-613 (Insufficient Session Expiration) |

**Deskripsi.** Default `JWT_ACCESS_TTL=24h`. `Logout` hanya mencabut refresh token;
access token bersifat stateless sehingga tetap valid sampai kedaluwarsa.

**Dampak.** Token akses yang dicuri (mis. dari log proxy, ekstensi browser, atau
perangkat bersama) tetap dapat dipakai hingga 24 jam meski pengguna sudah logout.

**Perbaikan.** Turunkan `JWT_ACCESS_TTL` ke 15–30 menit (front-end sudah punya
mekanisme refresh otomatis, sehingga dampak UX minimal), dan/atau tambahkan denylist
`jti` di Redis yang dicek middleware `Auth.Required` untuk kasus logout paksa.

---

### 4.6 Tidak ada audit log untuk aksi sensitif

| | |
|---|---|
| **Lokasi** | `internal/router/router.go:242-330` (seluruh rute `/admin`), `internal/handler/admin.go` |
| **Klasifikasi** | CWE-778 (Insufficient Logging), OWASP A09:2021 |

**Deskripsi.** Aksi berisiko tinggi — `POST /admin/orders/{id}/mark-paid`,
ubah/hapus soal, buat/ubah voucher, `POST /admin/broadcast`, serta perubahan data
pengguna — tidak meninggalkan jejak siapa melakukan apa dan kapan.

**Dampak.** Insiden (pemberian akses tanpa pembayaran, kebocoran data peserta,
broadcast tak sah) sulit dideteksi, ditelusuri, dan dibuktikan.

**Perbaikan.** Tambahkan tabel `audit_logs` (id, actor_id, action, entity,
entity_id, metadata jsonb, ip, user_agent, created_at) melalui migrasi baru
(`migrations/000012_audit_log.up.sql`), catat di lapisan service untuk aksi admin,
dan sediakan endpoint `GET /admin/audit-logs` untuk penelusuran.

---

## 5. Temuan sedang (P2)

| # | Temuan | Lokasi | Risiko | Perbaikan |
|---|---|---|---|---|
| 5.1 | Cookie sesi `ba_session` tidak ditandatangani/di-HMAC — berisi `role` dan dipercaya `src/proxy.ts` untuk gate `/admin` | `bumn-academy/src/lib/auth/session.ts`, `bumn-academy/src/proxy.ts` | Tampering cookie tidak terdeteksi (meski httpOnly + otorisasi backend tetap jadi penentu) | Tandatangani nilai cookie dengan HMAC, atau jangan jadikan cookie sebagai sumber kebenaran otorisasi |
| 5.2 | Belum ada token CSRF / cek `Origin` eksplisit untuk request state-changing lewat proxy `/api/be/*` | `bumn-academy/src/app/api/be/[...path]/route.ts` | `SameSite=Lax` menahan POST lintas-site, tetapi subdomain same-site masih bisa | Validasi `Origin`/`Sec-Fetch-Site` di proxy + token double-submit |
| 5.3 | Pemakaian ulang refresh token yang sudah dirotasi tidak dianggap indikasi pencurian | `internal/service/auth.go` (`Refresh`) | Token yang dicuri & dipakai ulang tidak memicu tindakan | Bila token revoked dipakai lagi → revoke **semua** sesi pengguna tersebut |
| 5.4 | Validasi berkas import soal minim (hanya ukuran 10 MiB + field `file`) | `internal/handler/admin.go:148-175` | Berkas tak terduga / jumlah baris ekstrem membebani parser | Allowlist ekstensi & magic bytes (`.xlsx`/`.csv`), batas jumlah baris/kolom |
| 5.5 | Belum ada batas panjang input pada endpoint AI (chat/explain) | `internal/dto/ai.go`, `internal/handler/ai.go` | Prompt injection & pembengkakan biaya kuota model | Tambahkan `max` pada DTO + instruksi sistem yang menegaskan peran, di luar `AI_DAILY_QUOTA` |
| 5.6 | Redis tanpa password & port DB/Redis terikat ke semua antarmuka | `docker-compose.yml:29,53` | Akses tak sah ke cache/rate limit | `--requirepass`, bind `127.0.0.1`, jaringan Docker internal saja |
| 5.7 | Belum ada CI & pemindaian dependensi/rahasia | — (tidak ada workflow) | Kerentanan dependensi & secret ter-commit lolos | `go vet`, `gofmt -l`, `go test`, `govulncheck ./...`, `npm audit`, `gitleaks`, Dependabot |
| 5.8 | `DB_SSLMODE=disable` sebagai default & di compose | `internal/config/config.go:217`, `docker-compose.yml:80` | Trafik DB tidak terenkripsi (sniffing/MITM) | Wajibkan `require`/`verify-full` di produksi (lihat §3.2) |

---

## 6. Sudah terverifikasi aman

Diverifikasi langsung pada kode agar tidak ada upaya perbaikan yang keliru sasaran:

- **SQL injection** — seluruh raw query di `internal/repository/attempt.go`,
  `order.go`, dan `classroom.go` memakai parameter binding
  (`Raw(query, arg...)`), tanpa interpolasi string.
- **Kebocoran kunci ke log** — `midtrans.Client.Describe()` hanya mengembalikan
  mode (`sandbox`/`production`); klien AI menaruh kunci hanya di header
  (`internal/ai/openai.go:305`), tidak pernah dicetak.
- **Path traversal media** — `MediaHandler` menolak `..`, path absolut, dan
  ekstensi di luar allowlist, serta memaksa `Content-Type`
  (`internal/handler/classroom.go:260-300`, diuji di `internal/handler/media_test.go`).
- **Otorisasi admin** — grup `/admin` dilindungi `Required` + `Admin`;
  enforcement dilakukan di backend, bukan hanya di UI.
- **Kepemilikan pesanan** — `GetOrder`/`CancelOrder`/`PayOrder` memeriksa
  `order.UserID != userID` → 403.
- **Verifikasi webhook** — signature Midtrans diverifikasi dengan
  `subtle.ConstantTimeCompare`; pelunasan bersifat idempotent (`settleOrder`).
- **Secrets di git** — hanya `.env.example` yang ter-track.

---

## 7. Checklist konfigurasi produksi

Gunakan tabel ini saat menyiapkan deploy. Kolom "Produksi" adalah nilai wajib.

| Variabel | Pengembangan | Produksi | Alasan |
|---|---|---|---|
| `APP_ENV` | `development` | `production` | Mengaktifkan guard & cookie `Secure` |
| `JWT_SECRET` | nilai apa pun | **≥ 32 karakter acak** (`openssl rand -hex 32`), dari secret manager | Mencegah pemalsuan token (§3.2) |
| `JWT_ACCESS_TTL` | `24h` | `15m`–`30m` | Membatasi jendela token dicuri (§4.5) |
| `JWT_REFRESH_TTL` | `720h` | sesuaikan kebijakan (mis. `168h`) | Memperpendek masa sesi |
| `SEED_ON_START` | `true` | **`false`** | Mencegah akun demo (§3.2) |
| `DB_AUTO_MIGRATE` | `true` | **`false`** (jalankan migrasi terpisah) | Hindari perubahan skema tak terduga |
| `DB_SSLMODE` | `disable` | `require` / `verify-full` | Enkripsi trafik DB (§5.8) |
| `MIDTRANS_IS_PRODUCTION` | `false` | `true` bila memakai akun nyata | Menonaktifkan simulator |
| `MIDTRANS_SANDBOX_SIMULATOR` | `true` | **`false`** | Mencegah pelunasan tanpa bayar (§2.1, §3.2) |
| `CORS_ALLOWED_ORIGINS` | localhost | domain front-end resmi saja | Jangan pernah pakai `*` bersama `AllowCredentials` |
| `RATE_LIMIT_RPS` / `BURST` | `30` / `60` | sesuaikan beban | Batas global |
| `RATE_LIMIT_AUTH_RPS` / `BURST` | — | tambahkan (mis. `5` / `10`) | Belum ada di kode — lihat §4.1 |
| `TRUSTED_PROXIES` | — | CIDR proxy/LB | Belum ada di kode — lihat §4.1 |
| `MEDIA_SIGNING_SECRET` | — | secret acak | Untuk signed URL media — lihat §4.3 |
| `REDIS_PASSWORD` | kosong | **wajib diisi** | Redis tanpa auth (§5.6) |

---

## 8. Rencana perbaikan bertahap

| Fase | Isi | Butir | Perkiraan |
|---|---|---|---|
| **Fase 1 — Blokir produksi** | Gate `PayOrder`; hardening `.env` + `docker-compose`; guard `validate()`; limiter auth + lockout akun | 3.1, 3.2, 3.3, 4.1 | 0.5–1 hari |
| **Fase 2 — Lapisan HTTP** | Security header backend & front-end; trusted proxy; redaksi `/readyz`; turunkan TTL JWT | 4.2, 4.1, 4.4, 4.5 | 1 hari |
| **Fase 3 — Konten & jejak** | Signed URL media; audit log; deteksi reuse refresh token; validasi upload; batas input AI | 4.3, 4.6, 5.3, 5.4, 5.5 | 1–2 hari |
| **Fase 4 — Proses** | CI (`go vet`, `go test`, `govulncheck`, `npm audit`, `gitleaks`), Dependabot, CSP disesuaikan Snap | 5.1, 5.2, 5.7 | 0.5 hari |

**Kriteria selesai Fase 1 (gate rilis):**

1. `POST /orders/{id}/pay` tidak dapat dipanggil saat `MIDTRANS_SANDBOX_SIMULATOR=false`
   (dibuktikan unit test).
2. Aplikasi **menolak start** bila `APP_ENV=production` dengan secret contoh/pendek,
   `SEED_ON_START=true`, atau `MIDTRANS_SANDBOX_SIMULATOR=true`.
3. `docker-compose.yml` tidak memuat kredensial literal.
4. `/auth/login` dibatasi limiter khusus + lockout per akun.

---

## 9. Referensi berkas

| Berkas | Peran dalam audit |
|---|---|
| `internal/router/router.go` | Pemasangan rute, middleware dasar, grup otorisasi |
| `internal/middleware/auth.go`, `common.go` | Autentikasi, rate limit, kunci klien |
| `internal/auth/jwt.go` | Penerbitan & verifikasi JWT, refresh token |
| `internal/service/auth.go`, `billing.go` | Login/refresh, alur pesanan & pelunasan |
| `internal/handler/health.go`, `classroom.go`, `admin.go` | `/readyz`, media, upload import soal |
| `internal/config/config.go` | Nilai default, validasi, gate simulator |
| `internal/pkg/password`, `pkg/midtrans`, `pkg/validator` | Kriptografi & verifikasi |
| `docker-compose.yml`, `Dockerfile`, `.env.example` | Konfigurasi deployment |
| `bumn-academy/src/lib/auth/session.ts`, `src/proxy.ts` | Cookie sesi & gate area terproteksi |
| `bumn-academy/src/app/api/be/[...path]/route.ts` | Proxy same-origin ke backend |

Dokumen pendamping untuk sisi front-end: `bumn-academy/SECURITY.md`.

