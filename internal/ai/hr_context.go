package ai

import (
	"encoding/json"
	"strings"
)

// Marker pembatas blok data analitik HR pada prompt.
//
// Sama seperti konteks hasil tryout, blok ini dibaca oleh model bahasa sungguhan
// maupun oleh asisten heuristik lokal sehingga keduanya berbicara dari angka yang
// sama — tidak ada jawaban yang dikarang.
const (
	hrContextOpen  = "<DATA_ANALITIK_HR>"
	hrContextClose = "</DATA_ANALITIK_HR>"
)

// HRBucket adalah hitungan pada satu rentang/kelompok.
type HRBucket struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// HRSkill adalah rata-rata skor satu kategori/subtes pada kohort.
type HRSkill struct {
	Name         string  `json:"name"`
	AverageScore float64 `json:"averageScore"`
	Attempts     int64   `json:"attempts"`
}

// HRMonth adalah aktivitas pada satu bulan.
type HRMonth struct {
	Label        string  `json:"label"`
	Attempts     int64   `json:"attempts"`
	AverageScore float64 `json:"averageScore"`
}

// HRProgram adalah capaian satu segmen program peserta.
type HRProgram struct {
	Program      string  `json:"program"`
	Peserta      int64   `json:"peserta"`
	Attempts     int64   `json:"attempts"`
	AverageScore float64 `json:"averageScore"`
}

// HRLearner adalah satu peserta pada daftar berprestasi atau perlu pendampingan.
type HRLearner struct {
	Name         string `json:"name"`
	Program      string `json:"program"`
	AttemptCount int64  `json:"attemptCount"`
	// Score adalah skor terbaik (daftar berprestasi) atau skor terakhir (daftar
	// pendampingan). HasScore=false berarti peserta belum punya nilai sama sekali.
	Score    int    `json:"score"`
	HasScore bool   `json:"hasScore"`
	Activity string `json:"lastActivity,omitempty"`
	Note     string `json:"note"`
}

// HRContext adalah seluruh fakta agregat analitik HR yang boleh dipakai asisten.
//
// Sengaja agregat: tidak ada email, id, atau jawaban per soal peserta — asisten
// hanya perlu angka kohort dan nama peserta pada dua daftar yang memang sudah
// tampil di dasbor admin.
type HRContext struct {
	GeneratedAt string `json:"generatedAt"`

	TotalPeserta      int64   `json:"totalPeserta"`
	ActivePeserta     int64   `json:"activePeserta"`
	InactivePeserta   int64   `json:"inactivePeserta"`
	ParticipationRate float64 `json:"participationRate"`
	ActiveWindowDays  int     `json:"activeWindowDays"`

	TotalAttempts    int64   `json:"totalAttempts"`
	AverageScore     float64 `json:"averageScore"`
	TopScore         int     `json:"topScore"`
	PassingThreshold int     `json:"passingThreshold"`
	PassingRate      float64 `json:"passingRate"`
	PracticeHours    float64 `json:"practiceHours"`
	DistinctTryouts  int64   `json:"distinctTryouts"`

	TotalLessons     int64   `json:"totalLessons"`
	CompletedLessons int64   `json:"completedLessons"`
	ActiveLearners   int64   `json:"activeLearners"`
	AverageProgress  float64 `json:"averageProgress"`
	CompletionRate   float64 `json:"completionRate"`

	ScoreDistribution []HRBucket  `json:"scoreDistribution"`
	SkillProfile      []HRSkill   `json:"skillProfile"`
	MonthlyTrend      []HRMonth   `json:"monthlyTrend"`
	ProgramBreakdown  []HRProgram `json:"programBreakdown"`
	TopLearners       []HRLearner `json:"topLearners"`
	AtRiskLearners    []HRLearner `json:"atRiskLearners"`
}

// RenderHRContext menyusun blok data analitik HR untuk prompt.
func RenderHRContext(ctx HRContext) string {
	encoded, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return hrContextOpen + "\n{}\n" + hrContextClose
	}
	return hrContextOpen + "\n" + string(encoded) + "\n" + hrContextClose
}

// ParseHRContext mengekstrak blok data analitik HR dari sebuah prompt.
func ParseHRContext(prompt string) (HRContext, bool) {
	start := strings.Index(prompt, hrContextOpen)
	end := strings.Index(prompt, hrContextClose)
	if start < 0 || end < 0 || end <= start {
		return HRContext{}, false
	}

	raw := strings.TrimSpace(prompt[start+len(hrContextOpen) : end])
	var ctx HRContext
	if err := json.Unmarshal([]byte(raw), &ctx); err != nil {
		return HRContext{}, false
	}
	return ctx, true
}

// ParseHRContextFromMessages mencari blok data HR pada daftar pesan (dari pesan
// terakhir ke pesan pertama, karena konteks selalu berada di pesan terbaru).
func ParseHRContextFromMessages(messages []Message) (HRContext, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if ctx, ok := ParseHRContext(messages[i].Content); ok {
			return ctx, true
		}
	}
	return HRContext{}, false
}

/* --------------------------------- turunan --------------------------------- */

// WeakestSkills mengembalikan kategori dengan rata-rata skor terendah.
func (c HRContext) WeakestSkills(limit int) []HRSkill {
	return trimSkills(sortSkillsByScore(c.ratedSkills(), true), limit)
}

// StrongestSkills mengembalikan kategori dengan rata-rata skor tertinggi.
func (c HRContext) StrongestSkills(limit int) []HRSkill {
	return trimSkills(sortSkillsByScore(c.ratedSkills(), false), limit)
}

// SkillsBelowThreshold mengembalikan kategori yang rata-ratanya di bawah ambang.
func (c HRContext) SkillsBelowThreshold() []HRSkill {
	out := make([]HRSkill, 0, len(c.SkillProfile))
	for _, skill := range c.ratedSkills() {
		if skill.AverageScore < float64(c.PassingThreshold) {
			out = append(out, skill)
		}
	}
	return sortSkillsByScore(out, true)
}

// LargestProgram mengembalikan segmen program dengan peserta terbanyak.
func (c HRContext) LargestProgram() (HRProgram, bool) {
	if len(c.ProgramBreakdown) == 0 {
		return HRProgram{}, false
	}
	best := c.ProgramBreakdown[0]
	for _, program := range c.ProgramBreakdown[1:] {
		if program.Peserta > best.Peserta {
			best = program
		}
	}
	return best, true
}

// NeverPracticed menghitung peserta yang belum pernah mengerjakan tryout.
func (c HRContext) NeverPracticed() int {
	total := 0
	for _, learner := range c.AtRiskLearners {
		if learner.AttemptCount == 0 || !learner.HasScore {
			total++
		}
	}
	return total
}

// ratedSkills hanya mengembalikan kategori yang punya percobaan (bisa dinilai).
func (c HRContext) ratedSkills() []HRSkill {
	out := make([]HRSkill, 0, len(c.SkillProfile))
	for _, skill := range c.SkillProfile {
		if skill.Attempts > 0 {
			out = append(out, skill)
		}
	}
	return out
}

// sortSkillsByScore mengurutkan kategori berdasarkan rata-rata skor.
func sortSkillsByScore(skills []HRSkill, ascending bool) []HRSkill {
	for i := 1; i < len(skills); i++ {
		for j := i; j > 0; j-- {
			left, right := skills[j-1].AverageScore, skills[j].AverageScore
			swapped := left > right
			if !ascending {
				swapped = left < right
			}
			if !swapped {
				break
			}
			skills[j-1], skills[j] = skills[j], skills[j-1]
		}
	}
	return skills
}

func trimSkills(skills []HRSkill, limit int) []HRSkill {
	if limit > 0 && len(skills) > limit {
		return skills[:limit]
	}
	return skills
}
