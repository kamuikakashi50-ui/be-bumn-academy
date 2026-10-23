// Package cache membungkus Redis untuk caching katalog, bank soal, leaderboard,
// dan rate limiting. Bila Redis tidak tersedia, seluruh operasi menjadi no-op
// sehingga aplikasi tetap berjalan (hanya kehilangan cache).
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/redis/go-redis/v9"
)

// Client adalah pembungkus klien Redis.
type Client struct {
	rdb     *redis.Client
	enabled bool
}

// New membuka koneksi Redis. Bila REDIS_ENABLED=false atau koneksi gagal,
// dikembalikan client non-aktif (bukan error) agar aplikasi tetap hidup.
func New(ctx context.Context, cfg *config.Config) *Client {
	if !cfg.Redis.Enabled {
		slog.Warn("redis dimatikan lewat konfigurasi — cache dilewati")
		return &Client{enabled: false}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.PoolSize / 4,
		// Timeout pendek: cache tidak boleh memperlambat request.
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolTimeout:  3 * time.Second,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		slog.Warn("redis tidak tersedia — cache dilewati", "addr", cfg.Redis.Addr, "error", err)
		_ = rdb.Close()
		return &Client{enabled: false}
	}

	slog.Info("redis terhubung", "addr", cfg.Redis.Addr, "pool_size", cfg.Redis.PoolSize)
	return &Client{rdb: rdb, enabled: true}
}

// Enabled menandakan Redis aktif dan bisa dipakai.
func (c *Client) Enabled() bool { return c != nil && c.enabled && c.rdb != nil }

// Redis mengembalikan klien mentah (bisa nil bila tidak aktif).
func (c *Client) Redis() *redis.Client {
	if !c.Enabled() {
		return nil
	}
	return c.rdb
}

// Ping memeriksa koneksi Redis (dipakai endpoint health).
func (c *Client) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return fmt.Errorf("redis tidak aktif")
	}
	return c.rdb.Ping(ctx).Err()
}

// Close menutup koneksi Redis.
func (c *Client) Close() error {
	if !c.Enabled() {
		return nil
	}
	return c.rdb.Close()
}

// Del menghapus kunci tertentu.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if !c.Enabled() || len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// DelPrefix menghapus seluruh kunci dengan awalan tertentu memakai SCAN
// (aman untuk dataset besar, tidak memblokir Redis seperti KEYS).
func (c *Client) DelPrefix(ctx context.Context, prefix string) error {
	if !c.Enabled() {
		return nil
	}
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, prefix+"*", 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
