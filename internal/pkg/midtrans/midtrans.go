// Package midtrans membungkus integrasi pembayaran Midtrans (Snap).
//
// Yang disediakan paket ini:
//
//   - pembuatan transaksi Snap (token + redirect_url) memakai Server Key,
//   - pengecekan status transaksi via Core API (untuk rekonsiliasi saat
//     notifikasi tidak bisa menjangkau server lokal),
//   - verifikasi signature notifikasi: SHA512(order_id + status_code +
//     gross_amount + ServerKey),
//   - pemetaan metode pembayaran internal → kanal Midtrans.
//
// Paket ini tidak menyimpan state apa pun; seluruh keputusan bisnis tetap ada di
// lapisan service.
package midtrans

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
)

// Konstanta domain Midtrans.
const (
	sandboxSnapBase = "https://app.sandbox.midtrans.com"
	prodSnapBase    = "https://app.midtrans.com"
	sandboxCoreBase = "https://api.sandbox.midtrans.com"
	prodCoreBase    = "https://api.midtrans.com"

	snapTransactionPath = "/snap/v1/transactions"

	// SandboxDemoServerKey dipakai simulator lokal saat kredensial Midtrans belum
	// diisi, agar jalur verifikasi signature tetap teruji tanpa akun Midtrans.
	SandboxDemoServerKey = "SB-Mid-server-SANDBOX-SIMULATOR"
)

// Config berisi kredensial dan pengaturan jaringan Midtrans.
type Config struct {
	ServerKey    string
	ClientKey    string
	IsProduction bool
	Timeout      time.Duration
	// SnapBaseURL & CoreBaseURL hanya dipakai untuk pengujian/override domain
	// (mis. lewat proxy internal). Bila kosong, domain resmi Midtrans dipakai.
	SnapBaseURL string
	CoreBaseURL string
}

// Client adalah klien HTTP Midtrans.
type Client struct {
	cfg  Config
	http *http.Client
}

// New membentuk klien Midtrans.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: timeout},
	}
}

// Enabled menandakan kredensial Midtrans sudah diisi sehingga Snap bisa dipakai.
func (c *Client) Enabled() bool {
	return strings.TrimSpace(c.cfg.ServerKey) != ""
}

// ClientKey mengembalikan client key (dipakai snap.js di browser).
func (c *Client) ClientKey() string { return c.cfg.ClientKey }

// IsProduction menandakan mode produksi.
func (c *Client) IsProduction() bool { return c.cfg.IsProduction }

// SigningKey mengembalikan kunci yang dipakai menghitung signature notifikasi.
//
// Bila Server Key belum diisi (mode simulator lokal), kunci demo dipakai agar
// perhitungan signature tetap konsisten di kedua sisi.
func (c *Client) SigningKey() string {
	if key := strings.TrimSpace(c.cfg.ServerKey); key != "" {
		return key
	}
	return SandboxDemoServerKey
}

// SnapScriptURL adalah URL snap.js sesuai lingkungan.
func (c *Client) SnapScriptURL() string {
	return c.snapBase() + "/snap/snap.js"
}

func (c *Client) snapBase() string {
	if base := strings.TrimSpace(c.cfg.SnapBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	if c.cfg.IsProduction {
		return prodSnapBase
	}
	return sandboxSnapBase
}

func (c *Client) coreBase() string {
	if base := strings.TrimSpace(c.cfg.CoreBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	if c.cfg.IsProduction {
		return prodCoreBase
	}
	return sandboxCoreBase
}

/* ------------------------------- signature -------------------------------- */

// Signature menghitung signature_key Midtrans:
// SHA512(order_id + status_code + gross_amount + ServerKey).
func Signature(orderID, statusCode, grossAmount, serverKey string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(sum[:])
}

// VerifySignature membandingkan signature_key notifikasi secara constant-time.
func (c *Client) VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	expected := Signature(orderID, statusCode, grossAmount, c.SigningKey())
	if expected == "" || signatureKey == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(signatureKey))) == 1
}

/* --------------------------- kanal pembayaran ----------------------------- */

// channelMap memetakan id metode pembayaran internal ke kanal Snap.
var channelMap = map[string][]string{
	"qris":       {"qris"},
	"gopay":      {"gopay"},
	"shopeepay":  {"shopeepay"},
	"va-bca":     {"bca_va"},
	"va-bni":     {"bni_va"},
	"va-mandiri": {"echannel"},
	"va-permata": {"permata_va"},
	"card":       {"credit_card"},
}

// ChannelsFor mengembalikan daftar kanal Midtrans untuk sebuah metode internal.
// Bila metode tidak dikenal, `nil` dikembalikan (artinya: semua kanal aktif).
func ChannelsFor(methodID string) []string {
	return channelMap[strings.ToLower(strings.TrimSpace(methodID))]
}

/* ------------------------------- permintaan ------------------------------- */

func (c *Client) authorization() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(c.cfg.ServerKey+":"))
}

// CreateSnapTransaction membuat transaksi Snap dan mengembalikan token-nya.
func (c *Client) CreateSnapTransaction(ctx context.Context, payload SnapRequest) (*SnapResponse, error) {
	if !c.Enabled() {
		return nil, apperror.BadRequest("Midtrans belum dikonfigurasi (MIDTRANS_SERVER_KEY kosong).")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menyusun permintaan Midtrans.")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.snapBase()+snapTransactionPath, bytes.NewReader(body))
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menyiapkan permintaan Midtrans.")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", c.authorization())

	var out SnapResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Token) == "" {
		return nil, apperror.Internal(nil, "Midtrans tidak mengembalikan token pembayaran.")
	}
	return &out, nil
}

// Status mengambil status transaksi terbaru langsung dari Midtrans (Core API).
//
// Dipakai untuk rekonsiliasi: notifikasi webhook tidak bisa dikirim ke
// localhost, sehingga front-end bisa meminta backend memeriksa status ke Midtrans.
func (c *Client) Status(ctx context.Context, orderID string) (*StatusResponse, error) {
	if !c.Enabled() {
		return nil, apperror.BadRequest("Midtrans belum dikonfigurasi (MIDTRANS_SERVER_KEY kosong).")
	}

	url := c.coreBase() + "/v2/" + orderID + "/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, apperror.Internal(err, "Gagal menyiapkan permintaan status Midtrans.")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", c.authorization())

	var out StatusResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// do menjalankan permintaan HTTP dan menerjemahkan galat Midtrans.
func (c *Client) do(req *http.Request, dst any) error {
	res, err := c.http.Do(req)
	if err != nil {
		return apperror.Internal(err, "Tidak bisa menghubungi Midtrans.")
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return apperror.Internal(err, "Balasan Midtrans tidak bisa dibaca.")
	}

	if res.StatusCode >= 400 {
		return apperror.BadRequest("Midtrans menolak permintaan: " + midtransMessage(raw))
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return apperror.Internal(err, "Balasan Midtrans tidak dikenali.")
	}
	return nil
}

// midtransMessage mengambil pesan galat Midtrans (error_messages/status_message).
func midtransMessage(raw []byte) string {
	var payload struct {
		ErrorMessages []string `json:"error_messages"`
		StatusMessage string   `json:"status_message"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "permintaan tidak valid"
	}
	if len(payload.ErrorMessages) > 0 {
		return strings.Join(payload.ErrorMessages, "; ")
	}
	if payload.StatusMessage != "" {
		return payload.StatusMessage
	}
	return "permintaan tidak valid"
}

// Describe merangkum mode yang sedang aktif (dipakai log & pesan bantuan).
func (c *Client) Describe() string {
	mode := "sandbox"
	if c.cfg.IsProduction {
		mode = "production"
	}
	if !c.Enabled() {
		return fmt.Sprintf("midtrans(%s, belum dikonfigurasi)", mode)
	}
	return fmt.Sprintf("midtrans(%s)", mode)
}

// Jumlah harga seluruh item harus sama dengan `transaction_details.gross_amount`.
type SnapItem struct {
	ID       string `json:"id"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
	Name     string `json:"name"`
}

// SnapCustomer adalah identitas pembeli pada halaman Snap.
type SnapCustomer struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

// SnapCallbacks adalah URL pengalihan setelah pembayaran selesai.
type SnapCallbacks struct {
	Finish   string `json:"finish,omitempty"`
	Unfinish string `json:"unfinish,omitempty"`
	Error    string `json:"error,omitempty"`
}

// SnapExpiry mengatur masa berlaku halaman pembayaran.
type SnapExpiry struct {
	Unit     string `json:"unit"`
	Duration int    `json:"duration"`
}

// SnapTransactionDetails adalah nominal transaksi Snap.
type SnapTransactionDetails struct {
	OrderID     string `json:"order_id"`
	GrossAmount int64  `json:"gross_amount"`
}

// SnapRequest adalah payload pembuatan transaksi Snap.
type SnapRequest struct {
	TransactionDetails SnapTransactionDetails `json:"transaction_details"`
	ItemDetails        []SnapItem             `json:"item_details,omitempty"`
	CustomerDetails    SnapCustomer           `json:"customer_details,omitempty"`
	Callbacks          *SnapCallbacks         `json:"callbacks,omitempty"`
	Expiry             *SnapExpiry            `json:"expiry,omitempty"`
	EnabledPayments    []string               `json:"enabled_payments,omitempty"`
	CustomField1       string                 `json:"custom_field1,omitempty"`
}

// SnapResponse adalah balasan pembuatan transaksi Snap.
type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

// StatusResponse adalah subset balasan GET status Core API.
type StatusResponse struct {
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	PaymentType       string `json:"payment_type"`
}
