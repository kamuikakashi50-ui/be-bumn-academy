package ai

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

// FallbackProvider mencoba provider utama lalu jatuh ke provider cadangan.
//
// Tujuannya menjaga fitur ulasan hasil tryout tetap hidup ketika penyedia model
// bermasalah (kuota habis, model belum diaktifkan, timeout, gangguan jaringan).
// Kegagalan dicatat agar operator bisa menindaklanjuti, sementara peserta tetap
// menerima ulasan dari mentor heuristik lokal.
type FallbackProvider struct {
	primary  Provider
	fallback Provider
	logger   *slog.Logger

	degraded atomic.Bool
	lastErr  atomic.Value // string
	mu       sync.Mutex
}

// NewFallback membungkus provider utama dengan provider cadangan.
func NewFallback(primary, fallback Provider, logger *slog.Logger) *FallbackProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &FallbackProvider{primary: primary, fallback: fallback, logger: logger}
}

// Name mengembalikan nama provider utama (nama yang dilihat klien).
func (p *FallbackProvider) Name() string {
	if p.primary != nil {
		return p.primary.Name()
	}
	return p.fallback.Name()
}

// Model mengembalikan id model provider utama.
func (p *FallbackProvider) Model() string {
	if p.primary != nil {
		return p.primary.Model()
	}
	return p.fallback.Model()
}

// Primary mengembalikan provider utama.
func (p *FallbackProvider) Primary() Provider { return p.primary }

// Fallback mengembalikan provider cadangan.
func (p *FallbackProvider) Fallback() Provider { return p.fallback }

// Degraded menandakan permintaan terakhir dilayani provider cadangan.
func (p *FallbackProvider) Degraded() bool { return p.degraded.Load() }

// LastError mengembalikan pesan kegagalan provider utama yang terakhir.
func (p *FallbackProvider) LastError() string {
	if value, ok := p.lastErr.Load().(string); ok {
		return value
	}
	return ""
}

// Chat mencoba provider utama lalu provider cadangan.
func (p *FallbackProvider) Chat(ctx context.Context, req Request) (*Response, error) {
	if p.primary == nil {
		return p.fallback.Chat(ctx, req)
	}

	response, err := p.primary.Chat(ctx, req)
	if err == nil {
		p.markHealthy()
		return response, nil
	}

	p.markDegraded(err)
	if p.fallback == nil {
		return nil, err
	}

	fallbackResponse, fallbackErr := p.fallback.Chat(ctx, req)
	if fallbackErr != nil {
		return nil, err
	}
	// Tandai bahwa balasan berasal dari mentor lokal.
	fallbackResponse.Provider = p.fallback.Name()
	return fallbackResponse, nil
}

// Stream mencoba provider utama lalu provider cadangan.
func (p *FallbackProvider) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	if p.primary == nil {
		return p.fallback.Stream(ctx, req)
	}

	chunks, err := p.primary.Stream(ctx, req)
	if err == nil {
		p.markHealthy()
		return p.pipeWithFallback(ctx, chunks, req), nil
	}

	p.markDegraded(err)
	if p.fallback == nil {
		return nil, err
	}
	return p.fallback.Stream(ctx, req)
}

// pipeWithFallback meneruskan potongan provider utama; bila aliran terputus
// sebelum ada isi, balasan diambil dari provider cadangan.
func (p *FallbackProvider) pipeWithFallback(ctx context.Context, source <-chan Chunk, req Request) <-chan Chunk {
	out := make(chan Chunk, 16)
	go func() {
		defer close(out)

		emitted := false
		for chunk := range source {
			if chunk.Err != nil {
				p.markDegraded(chunk.Err)
				if emitted || p.fallback == nil {
					out <- chunk
					return
				}
				p.streamFallback(ctx, req, out)
				return
			}
			if chunk.Delta != "" {
				emitted = true
			}
			out <- chunk
			if chunk.Done {
				return
			}
		}
	}()
	return out
}

func (p *FallbackProvider) streamFallback(ctx context.Context, req Request, out chan<- Chunk) {
	fallbackChunks, err := p.fallback.Stream(ctx, req)
	if err != nil {
		out <- Chunk{Err: err}
		return
	}
	for chunk := range fallbackChunks {
		out <- chunk
	}
}

func (p *FallbackProvider) markDegraded(err error) {
	p.degraded.Store(true)
	message := ""
	if err != nil {
		message = err.Error()
	}
	p.lastErr.Store(message)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.logger.Warn("mentor AI dialihkan ke provider cadangan", "error", message, "provider", p.Name())
}

func (p *FallbackProvider) markHealthy() {
	if p.degraded.Swap(false) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.logger.Info("mentor AI kembali memakai provider utama", "provider", p.Name())
	}
}
