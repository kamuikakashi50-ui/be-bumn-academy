package cache

import (
	"context"
	"testing"
	"time"
)

func TestLimiterTanpaRedisMemakaiJaringLokal(t *testing.T) {
	limiter := NewLimiter(nil, 5, 3)
	ctx := context.Background()

	allowed := 0
	for i := 0; i < 5; i++ {
		if limiter.Allow(ctx, "ip:127.0.0.1") {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("permintaan diizinkan = %d, harap 3 (sesuai burst)", allowed)
	}
}

func TestLimiterTerpisahPerKunci(t *testing.T) {
	limiter := NewLimiter(nil, 5, 2)
	ctx := context.Background()

	limiter.Allow(ctx, "ip:a")
	limiter.Allow(ctx, "ip:a")

	if limiter.Allow(ctx, "ip:a") {
		t.Fatal("kunci a seharusnya sudah habis")
	}
	if !limiter.Allow(ctx, "ip:b") {
		t.Fatal("kunci b harus punya jatah sendiri")
	}
}

func TestLimiterNilAman(t *testing.T) {
	var limiter *Limiter
	if !limiter.Allow(context.Background(), "ip:x") {
		t.Fatal("limiter nil harus selalu mengizinkan (tanpa rate limit)")
	}
}

func TestLimiterMengisiTokenKembali(t *testing.T) {
	// rps sangat tinggi agar token cepat terisi kembali.
	limiter := NewLimiter(nil, 1000, 1)
	ctx := context.Background()

	if !limiter.Allow(ctx, "ip:refill") {
		t.Fatal("permintaan pertama harus diizinkan")
	}
	if limiter.Allow(ctx, "ip:refill") {
		t.Fatal("permintaan kedua harus ditolak (burst 1)")
	}

	time.Sleep(20 * time.Millisecond)
	if !limiter.Allow(ctx, "ip:refill") {
		t.Fatal("token harus terisi kembali setelah jeda")
	}
}

func TestItoa(t *testing.T) {
	cases := map[int64]string{0: "0", 1: "1", 42: "42", 1234567890: "1234567890"}
	for input, want := range cases {
		if got := itoa(input); got != want {
			t.Errorf("itoa(%d) = %q, harap %q", input, got, want)
		}
	}
}

func TestKeyBuilders(t *testing.T) {
	if got := KeyQuestionBank("numerik"); got != "bank:numerik" {
		t.Errorf("KeyQuestionBank = %q", got)
	}
	if got := KeyLeaderboard("to-tkd-01"); got != "leaderboard:to-tkd-01" {
		t.Errorf("KeyLeaderboard = %q", got)
	}
	if got := KeyTryoutDetail("to-tkd-01"); got != "tryout:detail:to-tkd-01" {
		t.Errorf("KeyTryoutDetail = %q", got)
	}
	if got := KeyCatalogProducts(); got != "catalog:products" {
		t.Errorf("KeyCatalogProducts = %q", got)
	}
}

func TestRememberTanpaRedisMemanggilLoader(t *testing.T) {
	// Client non-aktif → loader harus tetap dipanggil (cache dilewati).
	client := &Client{enabled: false}
	layer := NewCache(client)

	calls := 0
	got, err := Remember(context.Background(), layer, "kunci", time.Minute, func(context.Context) (int, error) {
		calls++
		return 7, nil
	})
	if err != nil {
		t.Fatalf("Remember error: %v", err)
	}
	if got != 7 || calls != 1 {
		t.Fatalf("got=%d calls=%d, harap 7/1", got, calls)
	}
}

func TestForgetTanpaRedisAman(t *testing.T) {
	client := &Client{enabled: false}
	layer := NewCache(client)
	layer.Forget(context.Background(), "a", "b")
	layer.ForgetPrefix(context.Background(), "catalog:")

	var nilLayer *Cache
	nilLayer.Forget(context.Background(), "a")
	nilLayer.ForgetPrefix(context.Background(), "a")
}
