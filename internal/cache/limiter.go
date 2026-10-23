package cache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter membatasi jumlah permintaan per kunci (IP atau user id).
//
// Bila Redis aktif, penghitungan dilakukan terpusat memakai fixed window
// 1 detik sehingga batas tetap berlaku walau API berjalan di banyak replika.
// Bila Redis mati, dipakai token bucket in-memory per replika sebagai
// jaring pengaman (tetap membatasi, hanya tidak terkoordinasi antar replika).
type Limiter struct {
	client *Client
	rps    float64
	burst  int

	mu        sync.Mutex
	local     map[string]*localBucket
	lastSweep time.Time
}

type localBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewLimiter membentuk limiter.
func NewLimiter(client *Client, rps float64, burst int) *Limiter {
	if rps <= 0 {
		rps = 20
	}
	if burst <= 0 {
		burst = int(rps * 2)
	}
	return &Limiter{
		client: client,
		rps:    rps,
		burst:  burst,
		local:  make(map[string]*localBucket),
	}
}

// Allow memeriksa apakah permintaan dengan kunci tertentu boleh lanjut.
func (l *Limiter) Allow(ctx context.Context, key string) bool {
	if l == nil {
		return true
	}
	if l.client != nil && l.client.Enabled() {
		return l.allowRedis(ctx, key)
	}
	return l.allowLocal(key)
}

// allowRedis memakai INCR + EXPIRE pada jendela 1 detik.
func (l *Limiter) allowRedis(ctx context.Context, key string) bool {
	window := time.Now().Unix()
	cacheKey := "rl:" + key + ":" + itoa(window)

	pipe := l.client.rdb.TxPipeline()
	incr := pipe.Incr(ctx, cacheKey)
	pipe.Expire(ctx, cacheKey, 2*time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		// Redis bermasalah → jangan blokir pengguna, pakai jaring lokal.
		return l.allowLocal(key)
	}
	return incr.Val() <= int64(l.burst)
}

func (l *Limiter) allowLocal(key string) bool {
	now := time.Now()

	l.mu.Lock()
	bucket, ok := l.local[key]
	if !ok {
		bucket = &localBucket{limiter: rate.NewLimiter(rate.Limit(l.rps), l.burst)}
		l.local[key] = bucket
	}
	bucket.lastSeen = now

	// Bersihkan entri yang sudah lama tidak dipakai (maksimal sekali per menit).
	if now.Sub(l.lastSweep) > time.Minute {
		l.lastSweep = now
		for k, v := range l.local {
			if now.Sub(v.lastSeen) > 5*time.Minute {
				delete(l.local, k)
			}
		}
	}
	l.mu.Unlock()

	return bucket.limiter.Allow()
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
