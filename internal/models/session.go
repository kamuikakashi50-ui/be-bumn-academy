package models

import (
	"time"

	"gorm.io/gorm"
)

// SessionStatus adalah status sesi tryout.
type SessionStatus string

const (
	// SessionActive sesi sedang berjalan.
	SessionActive SessionStatus = "active"
	// SessionSubmitted sesi sudah dikumpulkan.
	SessionSubmitted SessionStatus = "submitted"
	// SessionExpired sesi kedaluwarsa tanpa dikumpulkan.
	SessionExpired SessionStatus = "expired"
)

// AttemptStatus adalah status hasil dibanding ambang batas.
type AttemptStatus string

const (
	// AttemptPassed skor di atas ambang.
	AttemptPassed AttemptStatus = "di atas ambang"
	// AttemptFailed skor perlu perbaikan.
	AttemptFailed AttemptStatus = "perlu perbaikan"
)

// TryoutSession adalah satu percobaan tryout milik peserta.
//
// Seluruh penentuan waktu berada di server: `SubtestDeadlines` berisi tenggat
// tiap subtes dan `ExpiresAt` adalah tenggat keras seluruh sesi. Klien hanya
// menampilkan hitungan mundur dari data ini, sehingga peserta tidak bisa
// menambah waktu secara lokal.
//
// `QuestionOrder` menyimpan snapshot id soal per subtes agar perubahan bank
// soal di tengah sesi tidak mengacaukan penilaian.
type TryoutSession struct {
	Base
	UserID           string        `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	TryoutID         string        `gorm:"column:tryout_id;not null" json:"tryoutId"`
	Status           SessionStatus `gorm:"column:status;not null;default:active" json:"status"`
	CurrentSubtest   int           `gorm:"column:current_subtest;not null;default:0" json:"currentSubtest"`
	QuestionOrder    IDList        `gorm:"column:question_order;type:jsonb;not null;default:'[]'" json:"-"`
	SubtestDeadlines TimeList      `gorm:"column:subtest_deadlines;type:jsonb;not null;default:'[]'" json:"subtestDeadlines"`
	StartedAt        time.Time     `gorm:"column:started_at;not null;autoCreateTime" json:"startedAt"`
	ExpiresAt        time.Time     `gorm:"column:expires_at;not null" json:"expiresAt"`
	SubmittedAt      *time.Time    `gorm:"column:submitted_at" json:"submittedAt,omitempty"`
	AttemptID        *string       `gorm:"column:attempt_id;type:uuid" json:"attemptId,omitempty"`
}

// TableName menetapkan nama tabel.
func (TryoutSession) TableName() string { return "tryout_sessions" }

// IsActive menandakan sesi masih bisa diisi.
func (s TryoutSession) IsActive() bool { return s.Status == SessionActive }

// RemainingSeconds menghitung sisa waktu sebuah subtes berdasarkan tenggat
// yang tersimpan di server. Nilai negatif dipotong menjadi 0.
func (s TryoutSession) RemainingSeconds(subtestIndex int, now time.Time) int {
	if subtestIndex < 0 || subtestIndex >= len(s.SubtestDeadlines) {
		return 0
	}
	remaining := s.SubtestDeadlines[subtestIndex].Sub(now)
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Seconds())
}

// TryoutAnswer adalah jawaban peserta untuk satu soal pada satu sesi.
//
// Primary key gabungan (session_id, question_id) memungkinkan autosave memakai
// satu statement INSERT ... ON CONFLICT DO UPDATE, sehingga aman saat peserta
// mengirim jawaban berulang dengan cepat.
type TryoutAnswer struct {
	SessionID     string    `gorm:"column:session_id;type:uuid;primaryKey" json:"sessionId"`
	QuestionID    string    `gorm:"column:question_id;primaryKey" json:"questionId"`
	SubtestIndex  int       `gorm:"column:subtest_index;not null;default:0" json:"subtestIndex"`
	SelectedIndex *int      `gorm:"column:selected_index" json:"selectedIndex"`
	Flagged       bool      `gorm:"column:flagged;not null;default:false" json:"flagged"`
	AnsweredAt    time.Time `gorm:"column:answered_at;not null;autoCreateTime" json:"answeredAt"`
}

// TableName menetapkan nama tabel.
func (TryoutAnswer) TableName() string { return "tryout_answers" }

// Attempt adalah hasil akhir satu sesi tryout yang tersimpan pada riwayat.
//
// Tidak memakai `Base` karena tabel `attempts` tidak memiliki `updated_at`:
// hasil tryout bersifat final dan tidak pernah diubah setelah tersimpan.
type Attempt struct {
	ID              string        `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	UserID          string        `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	TryoutID        string        `gorm:"column:tryout_id;not null" json:"tryoutId"`
	Title           string        `gorm:"column:title;not null;default:''" json:"title"`
	Score           int           `gorm:"column:score;not null;default:0" json:"score"`
	Correct         int           `gorm:"column:correct;not null;default:0" json:"correct"`
	Total           int           `gorm:"column:total;not null;default:0" json:"total"`
	Status          AttemptStatus `gorm:"column:status;not null;default:perlu perbaikan" json:"status"`
	DurationSeconds int           `gorm:"column:duration_seconds;not null;default:0" json:"durationSeconds"`
	TakenAt         time.Time     `gorm:"column:taken_at;not null;autoCreateTime" json:"takenAt"`
	CreatedAt       time.Time     `gorm:"column:created_at;autoCreateTime" json:"createdAt"`

	Categories []AttemptCategory `gorm:"foreignKey:AttemptID;references:ID" json:"categories"`
}

// TableName menetapkan nama tabel.
func (Attempt) TableName() string { return "attempts" }

// AttemptCategory adalah rincian skor per subtes pada satu attempt.
type AttemptCategory struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AttemptID string `gorm:"column:attempt_id;type:uuid;not null" json:"attemptId"`
	Name      string `gorm:"column:name;not null" json:"name"`
	Score     int    `gorm:"column:score;not null;default:0" json:"score"`
	Correct   int    `gorm:"column:correct;not null;default:0" json:"correct"`
	Total     int    `gorm:"column:total;not null;default:0" json:"total"`
	SortOrder int    `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
}

// TableName menetapkan nama tabel.
func (AttemptCategory) TableName() string { return "attempt_categories" }

// BeforeCreate memastikan ID attempt terisi saat dibuat langsung di Go.
func (a *Attempt) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = NewID()
	}
	return nil
}
