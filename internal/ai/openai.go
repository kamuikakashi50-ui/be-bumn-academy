package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxUpstreamBody membatasi jumlah byte balasan yang dibaca (jaga memori).
const maxUpstreamBody = 1 << 20 // 1 MiB

// OpenAIProvider berbicara dengan endpoint kompatibel OpenAI Chat Completions.
//
// BytePlus Ark memakai format yang sama (POST {base}/chat/completions) sehingga
// satu implementasi ini cukup untuk Ark, OpenAI, DeepSeek, Ollama, dan vLLM.
type OpenAIProvider struct {
	name        string
	baseURL     string
	apiKey      string
	model       string
	temperature float64
	maxTokens   int
	client      *http.Client
}

// OpenAIOptions mengatur pembuatan provider.
type OpenAIOptions struct {
	Name        string
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	Timeout     time.Duration
}

// NewOpenAI membentuk provider kompatibel OpenAI.
func NewOpenAI(opts OpenAIOptions) *OpenAIProvider {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	return &OpenAIProvider{
		name:        opts.Name,
		baseURL:     strings.TrimRight(opts.BaseURL, "/"),
		apiKey:      opts.APIKey,
		model:       opts.Model,
		temperature: opts.Temperature,
		maxTokens:   opts.MaxTokens,
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

// Name mengembalikan nama penyedia.
func (p *OpenAIProvider) Name() string { return p.name }

// Model mengembalikan id model.
func (p *OpenAIProvider) Model() string { return p.model }

/* ------------------------------ bentuk payload ----------------------------- */

type chatPayload struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatCompletion struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

/* ---------------------------------- chat ---------------------------------- */

// Chat mengirim permintaan dan mengembalikan balasan lengkap.
func (p *OpenAIProvider) Chat(ctx context.Context, req Request) (*Response, error) {
	started := time.Now()

	payload := p.buildPayload(req, false)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &UpstreamError{Provider: p.name, Message: "gagal menyusun permintaan", Err: err}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, &UpstreamError{Provider: p.name, Message: "gagal membuat request", Err: err}
	}
	p.decorate(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &UpstreamError{Provider: p.name, Message: "gagal menghubungi penyedia model", Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBody))
	if err != nil {
		return nil, &UpstreamError{Provider: p.name, StatusCode: resp.StatusCode, Message: "gagal membaca balasan", Err: err}
	}

	var parsed chatCompletion
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: resp.StatusCode,
			Message:    "balasan penyedia bukan JSON yang dikenali",
			Err:        err,
		}
	}

	if parsed.Error != nil {
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: resp.StatusCode,
			Code:       parsed.Error.Code,
			Message:    parsed.Error.Message,
		}
	}
	if resp.StatusCode >= 400 {
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: resp.StatusCode,
			Message:    truncateForError(string(raw)),
		}
	}
	if len(parsed.Choices) == 0 {
		return nil, &UpstreamError{Provider: p.name, StatusCode: resp.StatusCode, Message: "penyedia mengembalikan balasan kosong"}
	}

	return &Response{
		Content: strings.TrimSpace(parsed.Choices[0].Message.Content),
		Usage: Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		},
		Provider: p.name,
		Model:    p.model,
		Latency:  time.Since(started),
	}, nil
}

/* --------------------------------- stream --------------------------------- */

// Stream mengirim permintaan dengan mode streaming dan mengembalikan potongan
// balasan lewat channel. Channel selalu ditutup.
func (p *OpenAIProvider) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	payload := p.buildPayload(req, true)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &UpstreamError{Provider: p.name, Message: "gagal menyusun permintaan", Err: err}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, &UpstreamError{Provider: p.name, Message: "gagal membuat request", Err: err}
	}
	p.decorate(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &UpstreamError{Provider: p.name, Message: "gagal menghubungi penyedia model", Err: err}
	}

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBody))
		_ = resp.Body.Close()

		var parsed chatCompletion
		_ = json.Unmarshal(raw, &parsed)
		upstreamErr := &UpstreamError{Provider: p.name, StatusCode: resp.StatusCode, Message: truncateForError(string(raw))}
		if parsed.Error != nil {
			upstreamErr.Code = parsed.Error.Code
			upstreamErr.Message = parsed.Error.Message
		}
		return nil, upstreamErr
	}

	chunks := make(chan Chunk, 16)
	go p.readStream(ctx, resp.Body, chunks)
	return chunks, nil
}

// readStream mengurai event SSE dari penyedia menjadi potongan teks.
func (p *OpenAIProvider) readStream(ctx context.Context, body io.ReadCloser, chunks chan<- Chunk) {
	defer close(chunks)
	defer func() { _ = body.Close() }()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxUpstreamBody)

	var usage Usage
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			chunks <- Chunk{Done: true, Usage: usage}
			return
		}

		var event chatCompletion
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event.Error != nil {
			chunks <- Chunk{Err: &UpstreamError{Provider: p.name, Code: event.Error.Code, Message: event.Error.Message}}
			return
		}
		if event.Usage.PromptTokens > 0 || event.Usage.CompletionTokens > 0 {
			usage = Usage{
				PromptTokens:     event.Usage.PromptTokens,
				CompletionTokens: event.Usage.CompletionTokens,
				TotalTokens:      event.Usage.TotalTokens,
			}
		}
		if len(event.Choices) == 0 {
			continue
		}
		if delta := event.Choices[0].Delta.Content; delta != "" {
			select {
			case chunks <- Chunk{Delta: delta}:
			case <-ctx.Done():
				chunks <- Chunk{Err: ctx.Err()}
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		chunks <- Chunk{Err: &UpstreamError{Provider: p.name, Message: "koneksi streaming terputus", Err: err}}
		return
	}
	chunks <- Chunk{Done: true, Usage: usage}
}

/* --------------------------------- helper --------------------------------- */

func (p *OpenAIProvider) buildPayload(req Request, stream bool) chatPayload {
	temperature := req.Temperature
	if temperature == 0 {
		temperature = p.temperature
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	payload := chatPayload{
		Model:       p.model,
		Messages:    req.Messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      stream,
	}
	if req.JSONMode {
		payload.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	return payload
}

func (p *OpenAIProvider) decorate(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
}

func truncateForError(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) <= 300 {
		return raw
	}
	return raw[:300] + "…"
}
