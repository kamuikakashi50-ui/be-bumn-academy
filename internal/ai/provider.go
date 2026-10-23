// Package ai menyediakan abstraksi penyedia model bahasa untuk mentor AI
// ("MinSmart") yang dipakai pada ulasan hasil tryout.
//
// Tiga implementasi tersedia:
//
//   - openai  : endpoint apa pun yang kompatibel OpenAI Chat Completions
//     (BytePlus Ark, OpenAI, DeepSeek, Ollama, vLLM, OpenRouter),
//   - stub    : mentor heuristik lokal yang bekerja tanpa jaringan — dipakai
//     sebagai default pengembangan, CI, dan fallback saat provider utama gagal,
//   - fallback: pembungkus yang mencoba provider utama lalu jatuh ke stub.
//
// Service tidak pernah memanggil HTTP secara langsung; ia hanya bergantung pada
// interface Provider sehingga penggantian vendor tidak menyentuh aturan bisnis.
package ai

import (
	"context"
	"fmt"
	"time"
)

// Role adalah peran pengirim pesan.
type Role string

const (
	// RoleSystem instruksi sistem (persona mentor).
	RoleSystem Role = "system"
	// RoleUser pesan peserta.
	RoleUser Role = "user"
	// RoleAssistant balasan model.
	RoleAssistant Role = "assistant"
)

// Message adalah satu pesan pada permintaan model.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Request adalah permintaan penyelesaian teks.
type Request struct {
	Messages    []Message
	Temperature float64
	MaxTokens   int
	// JSONMode meminta model membalas JSON valid (dipakai untuk analisis
	// terstruktur).
	JSONMode bool
}

// Usage mencatat pemakaian token.
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// Response adalah balasan model.
type Response struct {
	Content  string
	Usage    Usage
	Provider string
	Model    string
	Latency  time.Duration
}

// Chunk adalah potongan balasan pada mode streaming.
type Chunk struct {
	Delta string
	Usage Usage
	Done  bool
	Err   error
}

// Provider adalah kontrak penyedia model bahasa.
type Provider interface {
	// Name mengembalikan nama penyedia (mis. "ark", "stub").
	Name() string
	// Model mengembalikan id model yang dipakai.
	Model() string
	// Chat mengirim permintaan dan menunggu balasan lengkap.
	Chat(ctx context.Context, req Request) (*Response, error)
	// Stream mengirim permintaan dan mengembalikan potongan balasan.
	// Channel selalu ditutup; kesalahan dikirim sebagai Chunk{Err}.
	Stream(ctx context.Context, req Request) (<-chan Chunk, error)
}

// UpstreamError membungkus kegagalan dari penyedia model.
//
// `Code` meneruskan kode dari penyedia (mis. `ModelNotOpen`,
// `InvalidEndpointOrModel.NotFound`) supaya operator bisa langsung tahu
// tindakan yang diperlukan tanpa membaca log server.
type UpstreamError struct {
	Provider   string
	StatusCode int
	Code       string
	Message    string
	Err        error
}

// Error memenuhi interface error.
func (e *UpstreamError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("ai/%s: %s (%s)", e.Provider, e.Message, e.Code)
	}
	return fmt.Sprintf("ai/%s: %s", e.Provider, e.Message)
}

// Unwrap memungkinkan errors.Is/As menembus ke error asli.
func (e *UpstreamError) Unwrap() error { return e.Err }

// IsRetryable menandakan kegagalan yang layak dicoba ulang atau dialihkan ke
// provider fallback (mis. rate limit, gangguan sementara).
func (e *UpstreamError) IsRetryable() bool {
	switch e.StatusCode {
	case 408, 409, 425, 429, 500, 502, 503, 504:
		return true
	}
	return false
}

// IsConfiguration menandakan kegagalan konfigurasi yang tidak akan berhasil
// bila diulang (mis. model belum diaktifkan atau API key salah).
func (e *UpstreamError) IsConfiguration() bool {
	switch e.Code {
	case "ModelNotOpen", "InvalidEndpointOrModel.NotFound", "AuthenticationError", "Unauthorized":
		return true
	}
	return e.StatusCode == 401 || e.StatusCode == 403
}
