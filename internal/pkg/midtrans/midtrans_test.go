package midtrans

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Signature dihitung dari gabungan order_id + status_code + gross_amount + kunci.
func TestSignatureMatchesMidtransFormula(t *testing.T) {
	orderID := "BA-2026-00001"
	statusCode := "200"
	grossAmount := "319200.00"
	serverKey := "SB-Mid-server-EXAMPLE"

	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	want := hex.EncodeToString(sum[:])

	if got := Signature(orderID, statusCode, grossAmount, serverKey); got != want {
		t.Fatalf("Signature() = %s, harap %s", got, want)
	}
}

func TestVerifySignature(t *testing.T) {
	client := New(Config{ServerKey: "SB-Mid-server-EXAMPLE"})

	valid := Signature("BA-2026-00002", "200", "150000.00", "SB-Mid-server-EXAMPLE")
	if !client.VerifySignature("BA-2026-00002", "200", "150000.00", valid) {
		t.Fatal("signature valid seharusnya diterima")
	}

	cases := []struct {
		name   string
		order  string
		status string
		amount string
		sig    string
	}{
		{"signature kosong", "BA-2026-00002", "200", "150000.00", ""},
		{"nominal diubah", "BA-2026-00002", "200", "15000.00", valid},
		{"order lain", "BA-2026-00003", "200", "150000.00", valid},
		{"signature palsu", "BA-2026-00002", "200", "150000.00", strings.Repeat("a", 128)},
	}
	for _, tc := range cases {
		if client.VerifySignature(tc.order, tc.status, tc.amount, tc.sig) {
			t.Errorf("%s: signature tidak valid seharusnya ditolak", tc.name)
		}
	}

	// Kunci demo dipakai saat kredensial belum diisi, sehingga simulator lokal
	// tetap melewati verifikasi yang sama.
	demo := New(Config{})
	demoSignature := Signature("BA-2026-00004", "200", "99000.00", SandboxDemoServerKey)
	if !demo.VerifySignature("BA-2026-00004", "200", "99000.00", demoSignature) {
		t.Fatal("mode simulator seharusnya memakai kunci demo")
	}
}

func TestChannelsFor(t *testing.T) {
	if got := ChannelsFor("qris"); len(got) != 1 || got[0] != "qris" {
		t.Fatalf("ChannelsFor(qris) = %v", got)
	}
	if got := ChannelsFor("VA-BCA"); len(got) != 1 || got[0] != "bca_va" {
		t.Fatalf("ChannelsFor(VA-BCA) = %v", got)
	}
	if got := ChannelsFor("metode-tidak-dikenal"); got != nil {
		t.Fatalf("metode tak dikenal seharusnya nil, dapat %v", got)
	}
}

func TestCreateSnapTransaction(t *testing.T) {
	var (
		gotAuth string
		gotPath string
		gotBody SnapRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"snap-token-123","redirect_url":"https://app.sandbox.midtrans.com/snap/v4/redirection/snap-token-123"}`))
	}))
	defer server.Close()

	client := New(Config{ServerKey: "SB-Mid-server-EXAMPLE", SnapBaseURL: server.URL})

	payload := SnapRequest{
		TransactionDetails: SnapTransactionDetails{OrderID: "BA-2026-00005", GrossAmount: 319200},
		ItemDetails: []SnapItem{
			{ID: "kelas-intensif", Price: 399000, Quantity: 1, Name: "Kelas Intensif"},
			{ID: "DISCOUNT", Price: -79800, Quantity: 1, Name: "Diskon MULAI20"},
		},
		EnabledPayments: ChannelsFor("qris"),
	}

	out, err := client.CreateSnapTransaction(context.Background(), payload)
	if err != nil {
		t.Fatalf("CreateSnapTransaction() error: %v", err)
	}
	if out.Token != "snap-token-123" || out.RedirectURL == "" {
		t.Fatalf("balasan tidak sesuai: %+v", out)
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("SB-Mid-server-EXAMPLE:"))
	if gotAuth != wantAuth {
		t.Errorf("Authorization = %q, harap %q", gotAuth, wantAuth)
	}
	if gotPath != "/snap/v1/transactions" {
		t.Errorf("path = %q, harap /snap/v1/transactions", gotPath)
	}
	if gotBody.TransactionDetails.OrderID != "BA-2026-00005" || gotBody.TransactionDetails.GrossAmount != 319200 {
		t.Errorf("transaction_details tidak terkirim: %+v", gotBody.TransactionDetails)
	}
	if len(gotBody.ItemDetails) != 2 {
		t.Errorf("item_details = %+v", gotBody.ItemDetails)
	}
}

func TestCreateSnapTransactionRejectsWithoutKey(t *testing.T) {
	client := New(Config{})
	if _, err := client.CreateSnapTransaction(context.Background(), SnapRequest{}); err == nil {
		t.Fatal("tanpa server key seharusnya mengembalikan galat")
	}
}

func TestStatusTranslatesMidtransError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status_code":"404","status_message":"Transaction doesn't exist."}`))
	}))
	defer server.Close()

	client := New(Config{ServerKey: "SB-Mid-server-EXAMPLE", CoreBaseURL: server.URL})
	_, err := client.Status(context.Background(), "BA-2026-00099")
	if err == nil || !strings.Contains(err.Error(), "Transaction doesn't exist.") {
		t.Fatalf("galat Midtrans tidak diteruskan: %v", err)
	}
}

func TestSnapScriptURLPerEnvironment(t *testing.T) {
	sandbox := New(Config{})
	if got := sandbox.SnapScriptURL(); got != "https://app.sandbox.midtrans.com/snap/snap.js" {
		t.Errorf("snap.js sandbox = %q", got)
	}
	production := New(Config{IsProduction: true})
	if got := production.SnapScriptURL(); got != "https://app.midtrans.com/snap/snap.js" {
		t.Errorf("snap.js produksi = %q", got)
	}
}
