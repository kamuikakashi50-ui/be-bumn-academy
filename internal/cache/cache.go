package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Cache menyediakan pola "remember" (read-through) dengan proteksi
// cache stampede memakai singleflight: saat cache kosong dan ribuan request
// datang bersamaan, hanya SATU request yang memuat data dari database.
type Cache struct {
	client *Client
	group  singleflight.Group
}

// NewCache membentuk Cache dari Client.
func NewCache(client *Client) *Cache {
	return &Cache{client: client}
}

// Client mengembalikan klien Redis di balik cache.
func (c *Cache) Client() *Client { return c.client }

// Remember mencoba membaca `dest` dari cache; bila tidak ada, memanggil loader
// lalu menyimpannya. Nilai cache yang rusak diabaikan (dianggap tidak ada).
func Remember[T any](
	ctx context.Context,
	c *Cache,
	key string,
	ttl time.Duration,
	loader func(context.Context) (T, error),
) (T, error) {
	var zero T
	if c == nil || !c.client.Enabled() {
		return loader(ctx)
	}

	// 1) Coba baca dari cache.
	if value, ok := getJSON[T](ctx, c.client, key); ok {
		return value, nil
	}

	// 2) Cache miss → dedupe dengan singleflight.
	raw, err, _ := c.group.Do(key, func() (any, error) {
		// Cek ulang: mungkin sudah diisi oleh request lain.
		if value, ok := getJSON[T](ctx, c.client, key); ok {
			return value, nil
		}
		value, loadErr := loader(ctx)
		if loadErr != nil {
			return zero, loadErr
		}
		setJSON(ctx, c.client, key, value, ttl)
		return value, nil
	})
	if err != nil {
		return zero, err
	}

	value, ok := raw.(T)
	if !ok {
		return zero, errors.New("cache: tipe nilai cache tidak cocok")
	}
	return value, nil
}

// Forget menghapus satu kunci cache.
func (c *Cache) Forget(ctx context.Context, keys ...string) {
	if c == nil {
		return
	}
	_ = c.client.Del(ctx, keys...)
}

// ForgetPrefix menghapus seluruh kunci dengan awalan tertentu.
func (c *Cache) ForgetPrefix(ctx context.Context, prefix string) {
	if c == nil {
		return
	}
	_ = c.client.DelPrefix(ctx, prefix)
}

func getJSON[T any](ctx context.Context, client *Client, key string) (T, bool) {
	var zero T
	raw, err := client.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			slog.Debug("cache: gagal membaca", "key", key, "error", err)
		}
		return zero, false
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		slog.Debug("cache: isi tidak valid, diabaikan", "key", key, "error", err)
		return zero, false
	}
	return out, true
}

func setJSON(ctx context.Context, client *Client, key string, value any, ttl time.Duration) {
	raw, err := json.Marshal(value)
	if err != nil {
		slog.Debug("cache: gagal marshal", "key", key, "error", err)
		return
	}
	if err := client.rdb.Set(ctx, key, raw, ttl).Err(); err != nil {
		slog.Debug("cache: gagal menulis", "key", key, "error", err)
	}
}

/* ------------------------------ kunci cache ------------------------------ */

// Key builders — dikumpulkan di satu tempat agar mudah di-invalidate.

// KeyCatalogPrograms adalah kunci daftar program.
func KeyCatalogPrograms() string { return "catalog:programs" }

// KeyCatalogMateri adalah kunci kategori materi.
func KeyCatalogMateri() string { return "catalog:materi" }

// KeyCatalogProducts adalah kunci daftar produk.
func KeyCatalogProducts() string { return "catalog:products" }

// KeyCatalogPaymentMethods adalah kunci metode pembayaran.
func KeyCatalogPaymentMethods() string { return "catalog:payment-methods" }

// KeyTryoutList adalah kunci daftar paket tryout.
func KeyTryoutList() string { return "tryout:list" }

// KeyTryoutDetail adalah kunci detail paket tryout.
func KeyTryoutDetail(id string) string { return "tryout:detail:" + id }

// KeyQuestionBank adalah kunci bank soal satu tryout per kategori.
//
// Soal dimiliki per paket tryout, sehingga kuncinya memuat tryoutID — dua tryout
// dengan kategori sama tidak boleh berbagi cache.
func KeyQuestionBank(tryoutID, bankKey string) string {
	return "bank:" + tryoutID + ":" + bankKey
}

// KeyLeaderboard adalah kunci cache leaderboard per tryout.
func KeyLeaderboard(tryoutID string) string { return "leaderboard:" + tryoutID }

// KeyLiveSessions adalah kunci daftar sesi live.
func KeyLiveSessions() string { return "learning:live-sessions" }

// KeyRecordings adalah kunci daftar rekaman.
func KeyRecordings() string { return "learning:recordings" }

// PrefixCatalog adalah awalan seluruh kunci katalog (untuk invalidasi massal).
func PrefixCatalog() string { return "catalog:" }

// PrefixTryout adalah awalan seluruh kunci tryout.
func PrefixTryout() string { return "tryout:" }

// PrefixQuestionBank adalah awalan seluruh kunci bank soal.
func PrefixQuestionBank() string { return "bank:" }

// KeyCorporateInsights adalah kunci ringkasan agregat halaman korporasi (B2B).
func KeyCorporateInsights() string { return "insights:corporate" }
