// Command loadtest menjalankan simulasi tryout serentak untuk mengukur
// ketahanan backend saat banyak peserta mengerjakan latihan bersamaan.
//
// Alurnya meniru perilaku front-end:
//  1. daftar akun peserta baru,
//  2. mulai sesi tryout,
//  3. autosave jawaban (beberapa kali, seperti peserta berpindah soal),
//  4. kumpulkan sesi dan verifikasi skor yang dikembalikan server.
//
// Pemakaian:
//
//	go run ./scripts/loadtest.go -users 200 -concurrency 40
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type options struct {
	base        string
	users       int
	concurrency int
	tryoutID    string
	timeout     time.Duration
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type authData struct {
	AccessToken string `json:"accessToken"`
}

type sessionData struct {
	ID             string `json:"id"`
	TotalQuestions int    `json:"totalQuestions"`
	Subtests       []struct {
		Name      string `json:"name"`
		Questions []struct {
			ID string `json:"id"`
		} `json:"questions"`
	} `json:"subtests"`
}

type submitData struct {
	ID      string `json:"id"`
	Score   int    `json:"score"`
	Correct int    `json:"correct"`
	Total   int    `json:"total"`
	Status  string `json:"status"`
}

// metrics mengumpulkan statistik pengukuran secara aman untuk konkurensi.
type metrics struct {
	mu         sync.Mutex
	latencies  []time.Duration
	errors     atomic.Int64
	usersOK    atomic.Int64
	attemptsOK atomic.Int64
	badScores  atomic.Int64
	requestErr atomic.Int64
}

func (m *metrics) record(d time.Duration) {
	m.mu.Lock()
	m.latencies = append(m.latencies, d)
	m.mu.Unlock()
}

func (m *metrics) summary() (p50, p95, p99, max time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.latencies) == 0 {
		return 0, 0, 0, 0
	}
	sorted := make([]time.Duration, len(m.latencies))
	copy(sorted, m.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	pick := func(percentile float64) time.Duration {
		return sorted[int(float64(len(sorted)-1)*percentile)]
	}
	return pick(0.50), pick(0.95), pick(0.99), sorted[len(sorted)-1]
}

func main() {
	opts := options{}
	flag.StringVar(&opts.base, "base", "http://127.0.0.1:8080/api/v1", "base URL API")
	flag.IntVar(&opts.users, "users", 100, "jumlah peserta simulasi")
	flag.IntVar(&opts.concurrency, "concurrency", 25, "jumlah pekerja paralel")
	flag.StringVar(&opts.tryoutID, "tryout", "to-demo-nasional", "id paket tryout")
	flag.DurationVar(&opts.timeout, "timeout", 60*time.Second, "timeout per request")
	flag.Parse()

	if opts.concurrency > opts.users {
		opts.concurrency = opts.users
	}

	client := &http.Client{Timeout: opts.timeout}
	ctx := context.Background()

	fmt.Printf("Load test tryout serentak\n")
	fmt.Printf("  base        : %s\n", opts.base)
	fmt.Printf("  paket       : %s\n", opts.tryoutID)
	fmt.Printf("  peserta     : %d\n", opts.users)
	fmt.Printf("  paralel     : %d\n", opts.concurrency)
	fmt.Println(strings.Repeat("-", 58))

	// Fase 1 — siapkan akun peserta.
	// Pendaftaran sengaja dibatasi beberapa pekerja sekaligus dan mengulang
	// saat terkena rate limit, supaya fase pengukuran tidak tercemar 429.
	fmt.Println("Fase 1/2 — mendaftarkan akun peserta…")
	prepareStart := time.Now()
	tokens := prepareUsers(ctx, client, opts)
	fmt.Printf("  %d/%d akun siap dalam %s\n", len(tokens), opts.users, time.Since(prepareStart).Round(time.Millisecond))
	if len(tokens) == 0 {
		fmt.Println("\n❌ Tidak ada akun yang berhasil didaftarkan — periksa API dan rate limit.")
		os.Exit(1)
	}

	// Fase 2 — jalankan tryout serentak.
	fmt.Println("Fase 2/2 — menjalankan tryout serentak…")
	stats := &metrics{}
	jobs := make(chan string)
	var wg sync.WaitGroup

	start := time.Now()
	for worker := 0; worker < opts.concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for token := range jobs {
				runParticipant(ctx, client, opts, stats, token)
			}
		}()
	}

	for _, token := range tokens {
		jobs <- token
	}
	close(jobs)
	wg.Wait()

	elapsed := time.Since(start)
	p50, p95, p99, max := stats.summary()

	fmt.Println(strings.Repeat("-", 58))
	fmt.Printf("Total waktu tryout   : %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Peserta berhasil     : %d/%d\n", stats.usersOK.Load(), len(tokens))
	fmt.Printf("Sesi dikumpulkan     : %d\n", stats.attemptsOK.Load())
	fmt.Printf("Latensi p50/p95/p99  : %s / %s / %s\n",
		p50.Round(time.Millisecond), p95.Round(time.Millisecond), p99.Round(time.Millisecond))
	fmt.Printf("Latensi maksimum     : %s\n", max.Round(time.Millisecond))
	fmt.Printf("Throughput           : %.1f peserta/detik\n", float64(stats.usersOK.Load())/elapsed.Seconds())
	fmt.Printf("Error aplikasi       : %d\n", stats.errors.Load())
	fmt.Printf("Error jaringan       : %d\n", stats.requestErr.Load())
	fmt.Printf("Skor tidak valid     : %d\n", stats.badScores.Load())

	if stats.errors.Load() > 0 || stats.requestErr.Load() > 0 || stats.badScores.Load() > 0 {
		fmt.Println("\n❌ Load test GAGAL")
		os.Exit(1)
	}
	fmt.Println("\n✅ Load test LULUS — tidak ada error dan seluruh skor valid")
}

// prepareUsers mendaftarkan akun peserta secara bertahap dan mengembalikan
// daftar access token-nya.
func prepareUsers(ctx context.Context, client *http.Client, opts options) []string {
	parallel := opts.concurrency
	if parallel > 8 {
		parallel = 8
	}

	tokens := make([]string, 0, opts.users)
	var mu sync.Mutex
	jobs := make(chan int)
	var wg sync.WaitGroup

	stamp := time.Now().UnixNano()
	for worker := 0; worker < parallel; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				email := fmt.Sprintf("loadtest-%d-%d@bumnacademy.id", stamp, index)
				token, err := registerWithRetry(ctx, client, opts, email)
				if err != nil {
					continue
				}
				mu.Lock()
				tokens = append(tokens, token)
				mu.Unlock()
			}
		}()
	}

	for index := 0; index < opts.users; index++ {
		jobs <- index
	}
	close(jobs)
	wg.Wait()

	return tokens
}

// registerWithRetry mengulang pendaftaran bila terkena rate limit.
func registerWithRetry(ctx context.Context, client *http.Client, opts options, email string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		token, err := register(ctx, client, opts, email)
		if err == nil {
			return token, nil
		}
		lastErr = err
		if !strings.Contains(err.Error(), "RATE_LIMITED") {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(150*(attempt+1)) * time.Millisecond):
		}
	}
	return "", lastErr
}

// runParticipant menjalankan satu alur tryout penuh untuk seorang peserta.
func runParticipant(ctx context.Context, client *http.Client, opts options, stats *metrics, token string) {
	session, err := startSession(ctx, client, opts, token, stats)
	if err != nil {
		stats.errors.Add(1)
		return
	}

	if err := saveAnswers(ctx, client, opts, token, session, stats); err != nil {
		stats.errors.Add(1)
		return
	}

	result, err := submit(ctx, client, opts, token, session.ID, stats)
	if err != nil {
		stats.errors.Add(1)
		return
	}

	// Verifikasi hasil dihitung server dengan benar.
	if result.Score < 0 || result.Score > 100 || result.Correct < 0 || result.Correct > result.Total {
		stats.badScores.Add(1)
		return
	}
	if result.Status != "di atas ambang" && result.Status != "perlu perbaikan" {
		stats.badScores.Add(1)
		return
	}

	stats.usersOK.Add(1)
	stats.attemptsOK.Add(1)
}

func register(ctx context.Context, client *http.Client, opts options, email string) (string, error) {
	body := map[string]any{
		"name":     "Peserta Load Test",
		"email":    email,
		"password": "rahasia123",
	}
	var out authData
	if err := call(ctx, client, opts, http.MethodPost, "/auth/register", "", body, &out); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

func startSession(ctx context.Context, client *http.Client, opts options, token string, stats *metrics) (*sessionData, error) {
	var out sessionData
	path := "/tryouts/" + opts.tryoutID + "/sessions"
	if err := call(ctx, client, opts, http.MethodPost, path, token, nil, &out, stats); err != nil {
		return nil, err
	}
	return &out, nil
}

// saveAnswers menulis jawaban dalam beberapa permintaan, meniru autosave
// berkala saat peserta berpindah soal.
func saveAnswers(
	ctx context.Context,
	client *http.Client,
	opts options,
	token string,
	session *sessionData,
	stats *metrics,
) error {
	for subtestIndex, subtest := range session.Subtests {
		if len(subtest.Questions) == 0 {
			continue
		}

		answers := make([]map[string]any, 0, len(subtest.Questions))
		for _, question := range subtest.Questions {
			selected := rand.Intn(4) // #nosec G404 — data simulasi, bukan kriptografi
			answers = append(answers, map[string]any{
				"questionId":    question.ID,
				"selectedIndex": selected,
				"flagged":       selected == 3,
			})
		}

		payload := map[string]any{"subtestIndex": subtestIndex, "answers": answers}
		path := "/tryout-sessions/" + session.ID + "/answers"
		if err := call(ctx, client, opts, http.MethodPatch, path, token, payload, nil, stats); err != nil {
			return err
		}
	}
	return nil
}

func submit(
	ctx context.Context,
	client *http.Client,
	opts options,
	token, sessionID string,
	stats *metrics,
) (*submitData, error) {
	var out submitData
	path := "/tryout-sessions/" + sessionID + "/submit"
	if err := call(ctx, client, opts, http.MethodPost, path, token, nil, &out, stats); err != nil {
		return nil, err
	}
	return &out, nil
}

// call mengirim satu permintaan HTTP dan mengurai envelope balasan.
func call(
	ctx context.Context,
	client *http.Client,
	opts options,
	method, path, token string,
	payload any,
	out any,
	optionalStats ...*metrics,
) error {
	var body *bytes.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, opts.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	started := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(started)

	var stats *metrics
	if len(optionalStats) > 0 {
		stats = optionalStats[0]
	}
	if stats != nil {
		stats.record(elapsed)
	}

	if err != nil {
		if stats != nil {
			stats.requestErr.Add(1)
		}
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		if stats != nil {
			stats.requestErr.Add(1)
		}
		return fmt.Errorf("balasan bukan JSON: %w", err)
	}

	if !env.Success {
		code := "UNKNOWN"
		message := ""
		if env.Error != nil {
			code = env.Error.Code
			message = env.Error.Message
		}
		return fmt.Errorf("%s %s → %s: %s", method, path, code, message)
	}

	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("gagal mengurai data: %w", err)
		}
	}
	return nil
}
