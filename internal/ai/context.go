package ai

import (
	"encoding/json"
	"strings"
)

// Marker pembatas blok konteks pada prompt.
//
// Blok ini dibaca oleh model bahasa sungguhan maupun oleh mentor heuristik
// lokal, sehingga keduanya menerima fakta yang sama tentang hasil tryout.
const (
	contextOpen  = "<KONTEKS_HASIL_TRYOUT>"
	contextClose = "</KONTEKS_HASIL_TRYOUT>"
)

// SubtestContext adalah ringkasan satu subtes pada hasil tryout.
type SubtestContext struct {
	Name    string `json:"name"`
	Score   int    `json:"score"`
	Correct int    `json:"correct"`
	Total   int    `json:"total"`
	Minutes int    `json:"minutes,omitempty"`
}

// QuestionContext adalah satu soal pada pembahasan hasil tryout.
type QuestionContext struct {
	ID            string   `json:"id"`
	Number        int      `json:"number"`
	Subtest       string   `json:"subtest"`
	BankLabel     string   `json:"bankLabel,omitempty"`
	Text          string   `json:"text"`
	Options       []string `json:"options"`
	CorrectIndex  int      `json:"correctIndex"`
	SelectedIndex *int     `json:"selectedIndex"`
	IsCorrect     bool     `json:"isCorrect"`
	IsUnanswered  bool     `json:"isUnanswered"`
	Flagged       bool     `json:"flagged"`
	Difficulty    string   `json:"difficulty,omitempty"`
	Explanation   string   `json:"explanation"`
}

// AttemptContext adalah seluruh fakta hasil tryout yang boleh diketahui mentor.
//
// SENGAJA hanya berisi data milik peserta yang bersangkutan: tidak ada nama
// peserta lain, tidak ada peringkat nasional, dan tidak ada data soal di luar
// paket tryout yang dikerjakan.
type AttemptContext struct {
	Title            string            `json:"title"`
	TryoutID         string            `json:"tryoutId"`
	TakenAt          string            `json:"takenAt"`
	Score            int               `json:"score"`
	Correct          int               `json:"correct"`
	Total            int               `json:"total"`
	PassingThreshold int               `json:"passingThreshold"`
	Status           string            `json:"status"`
	DurationSeconds  int               `json:"durationSeconds"`
	AttemptNumber    int               `json:"attemptNumber,omitempty"`
	Subtests         []SubtestContext  `json:"subtests"`
	Questions        []QuestionContext `json:"questions"`
	// Truncated menandakan sebagian soal tidak disertakan karena batas konteks.
	Truncated bool `json:"truncated,omitempty"`
}

// WrongQuestions mengembalikan soal yang salah atau tidak dijawab.
func (c AttemptContext) WrongQuestions() []QuestionContext {
	out := make([]QuestionContext, 0, len(c.Questions))
	for _, question := range c.Questions {
		if !question.IsCorrect {
			out = append(out, question)
		}
	}
	return out
}

// QuestionByID mencari satu soal pada konteks.
func (c AttemptContext) QuestionByID(id string) (QuestionContext, bool) {
	for _, question := range c.Questions {
		if question.ID == id {
			return question, true
		}
	}
	return QuestionContext{}, false
}

// WeakestSubtests mengembalikan subtes dengan skor terendah (maksimal n).
func (c AttemptContext) WeakestSubtests(n int) []SubtestContext {
	return sortSubtestsByScore(c.Subtests, false, n)
}

// StrongestSubtests mengembalikan subtes dengan skor tertinggi (maksimal n).
func (c AttemptContext) StrongestSubtests(n int) []SubtestContext {
	return sortSubtestsByScore(c.Subtests, true, n)
}

// sortSubtestsByScore mengurutkan subtes berdasarkan skor.
//
// desc=true → tertinggi ke terendah (kekuatan), desc=false → terendah ke
// tertinggi (yang perlu diperbaiki lebih dulu).
func sortSubtestsByScore(subtests []SubtestContext, desc bool, limit int) []SubtestContext {
	sorted := make([]SubtestContext, len(subtests))
	copy(sorted, subtests)

	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			// Naik (ascending) bila desc=false: tukar ketika elemen kiri lebih besar.
			swap := sorted[j-1].Score > sorted[j].Score
			if desc {
				swap = sorted[j-1].Score < sorted[j].Score
			}
			if !swap {
				break
			}
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}

	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

// ThresholdOrDefault mengembalikan ambang batas efektif konteks.
func (c AttemptContext) ThresholdOrDefault() int {
	if c.PassingThreshold <= 0 {
		return 70
	}
	return c.PassingThreshold
}

// StrongSubtests mengembalikan subtes yang sudah mencapai ambang batas.
// Bila tidak ada yang mencapai ambang, dikembalikan subtes dengan skor tertinggi
// sebagai kekuatan relatif.
func (c AttemptContext) StrongSubtests(limit int) []SubtestContext {
	threshold := c.ThresholdOrDefault()

	out := make([]SubtestContext, 0, limit)
	for _, subtest := range c.StrongestSubtests(len(c.Subtests)) {
		if subtest.Score >= threshold {
			out = append(out, subtest)
		}
	}
	if len(out) == 0 {
		best := c.StrongestSubtests(1)
		if len(best) > 0 {
			out = append(out, best[0])
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// WeakSubtests mengembalikan subtes yang belum mencapai ambang batas,
// diurutkan dari yang paling lemah.
func (c AttemptContext) WeakSubtests(limit int) []SubtestContext {
	threshold := c.ThresholdOrDefault()

	out := make([]SubtestContext, 0, limit)
	for _, subtest := range c.WeakestSubtests(len(c.Subtests)) {
		if subtest.Score < threshold {
			out = append(out, subtest)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// RenderContext membentuk blok konteks JSON yang disisipkan ke prompt.
func RenderContext(ctx AttemptContext) string {
	encoded, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return contextOpen + "\n{}\n" + contextClose
	}
	return contextOpen + "\n" + string(encoded) + "\n" + contextClose
}

// ParseContext mengekstrak blok konteks dari sebuah prompt.
//
// Dipakai mentor heuristik lokal; mengembalikan ok=false bila blok tidak ada.
func ParseContext(prompt string) (AttemptContext, bool) {
	start := strings.Index(prompt, contextOpen)
	end := strings.Index(prompt, contextClose)
	if start < 0 || end < 0 || end <= start {
		return AttemptContext{}, false
	}

	raw := strings.TrimSpace(prompt[start+len(contextOpen) : end])
	var ctx AttemptContext
	if err := json.Unmarshal([]byte(raw), &ctx); err != nil {
		return AttemptContext{}, false
	}
	return ctx, true
}

// ParseContextFromMessages mencari blok konteks pada daftar pesan (dari pesan
// terakhir ke pesan pertama, karena konteks selalu berada di pesan terbaru).
func ParseContextFromMessages(messages []Message) (AttemptContext, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if ctx, ok := ParseContext(messages[i].Content); ok {
			return ctx, true
		}
	}
	return AttemptContext{}, false
}
