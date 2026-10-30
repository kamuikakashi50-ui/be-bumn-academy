package models

import (
	"fmt"
	"time"
)

/* ------------------------------ jalur belajar ------------------------------ */

// LearningPath adalah urutan modul untuk satu kategori/kelas sasaran.
//
// Jalur tidak memiliki materi sendiri: langkah-langkahnya menunjuk modul ruang
// kelas yang sudah ada (`course_modules`), sehingga perbaikan materi otomatis
// berlaku untuk semua jalur yang memakainya.
type LearningPath struct {
	ID          string `gorm:"column:id;primaryKey" json:"id"`
	Slug        string `gorm:"column:slug;not null" json:"slug"`
	Title       string `gorm:"column:title;not null" json:"title"`
	Description string `gorm:"column:description;not null;default:''" json:"description"`
	// SectorKey & TestType memakai kunci taksonomi Fase 0.
	SectorKey   string `gorm:"column:sector_key;not null;default:umum" json:"sectorKey"`
	TestType    string `gorm:"column:test_type;not null;default:tkd" json:"testType"`
	ProductSlug string `gorm:"column:product_slug;not null" json:"productSlug"`
	// Certificate menandai jalur ini menerbitkan sertifikat saat selesai.
	//
	// Sengaja tanpa tag `default:` — GORM akan menimpa nilai nol (`false`) dengan
	// nilai tag saat INSERT, sehingga jalur tanpa sertifikat tidak bisa disimpan.
	Certificate bool `gorm:"column:certificate;not null" json:"certificate"`
	SortOrder   int  `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	Active      bool `gorm:"column:active;not null" json:"active"`
	Timestamps

	Steps []LearningPathStep `gorm:"foreignKey:PathID" json:"steps,omitempty"`
}

// TableName menetapkan nama tabel.
func (LearningPath) TableName() string { return "learning_paths" }

// LearningPathStep adalah satu langkah (modul) di dalam jalur belajar.
type LearningPathStep struct {
	Base
	PathID   string `gorm:"column:path_id;not null" json:"pathId"`
	ModuleID string `gorm:"column:module_id;not null" json:"moduleId"`
	// Required menandai modul wajib; modul opsional tidak menghalangi sertifikat.
	//
	// Tanpa tag `default:` (lihat catatan pada LearningPath.Certificate) supaya
	// modul opsional (`false`) benar-benar tersimpan.
	Required  bool `gorm:"column:required;not null" json:"required"`
	SortOrder int  `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
}

// TableName menetapkan nama tabel.
func (LearningPathStep) TableName() string { return "learning_path_steps" }

// RequiredModuleIDs mengembalikan id modul wajib pada jalur (urutan tetap).
//
// Dipakai pemeriksaan kelayakan sertifikat: hanya langkah wajib yang menentukan.
func (p LearningPath) RequiredModuleIDs() []string {
	out := make([]string, 0, len(p.Steps))
	for _, step := range p.Steps {
		if step.Required {
			out = append(out, step.ModuleID)
		}
	}
	return out
}

/* --------------------------- kuis per pelajaran ---------------------------- */

// LessonQuiz adalah kuis pendek di akhir sebuah pelajaran.
//
// Soalnya disimpan sebagai snapshot (`lesson_quiz_questions`) sehingga seluruh
// peserta satu kelas mengerjakan susunan yang sama dan hasilnya dapat diaudit.
type LessonQuiz struct {
	Base
	LessonID string `gorm:"column:lesson_id;not null" json:"lessonId"`
	Title    string `gorm:"column:title;not null" json:"title"`
	// PassScore adalah ambang lulus dalam persen (0-100).
	//
	// Tanpa tag `default:`: nilai 0 sah (peserta selalu lulus) dan default 70
	// ditetapkan service saat admin tidak mengirimkannya.
	PassScore int `gorm:"column:pass_score;not null" json:"passScore"`
	// MaxAttempts 0 berarti tanpa batas percobaan, jadi tidak boleh ditimpa
	// default 1 oleh tag GORM; default 1 ditetapkan service.
	MaxAttempts int `gorm:"column:max_attempts;not null" json:"maxAttempts"`
	// TimeLimit dalam menit; 0 berarti tanpa batas waktu.
	TimeLimit int  `gorm:"column:time_limit;not null;default:0" json:"timeLimit"`
	Active    bool `gorm:"column:active;not null" json:"active"`

	Questions []LessonQuizQuestion `gorm:"foreignKey:QuizID" json:"questions,omitempty"`
}

// TableName menetapkan nama tabel.
func (LessonQuiz) TableName() string { return "lesson_quizzes" }

// ScoreFor menghitung nilai persen dari jumlah jawaban benar.
//
// Kuis tanpa soal (total 0) bernilai 0, bukan pembagian tak hingga.
func (q LessonQuiz) ScoreFor(correct, total int) int {
	if total <= 0 {
		return 0
	}
	score := int(float64(correct) / float64(total) * 100)
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// PassedWith menentukan lulus berdasarkan ambang `PassScore`.
func (q LessonQuiz) PassedWith(score int) bool { return score >= q.PassScore }

// AttemptsAllowed menentukan apakah peserta masih boleh mengerjakan kuis.
//
// `used` adalah jumlah percobaan yang sudah dikumpulkan; percobaan yang belum
// dikumpulkan tidak dihitung (lihat repository).
func (q LessonQuiz) AttemptsAllowed(used int) bool {
	return q.MaxAttempts <= 0 || used < q.MaxAttempts
}

// LessonQuizQuestion adalah satu soal di dalam kuis.
//
// `QuestionID` boleh NULL: soal yang dihapus dari bank soal membuat posisi ini
// kosong dan kuis ditandai "perlu ditinjau" di panel admin.
type LessonQuizQuestion struct {
	QuizID     string  `gorm:"column:quiz_id;primaryKey" json:"quizId"`
	QuestionID *string `gorm:"column:question_id" json:"questionId,omitempty"`
	Position   int     `gorm:"column:position;primaryKey" json:"position"`
}

// TableName menetapkan nama tabel.
func (LessonQuizQuestion) TableName() string { return "lesson_quiz_questions" }

// QuizAnswer adalah satu jawaban peserta pada kuis.
type QuizAnswer struct {
	QuestionID    string `json:"questionId"`
	SelectedIndex *int   `json:"selectedIndex,omitempty"`
}

// QuizAttempt adalah satu percobaan kuis oleh peserta.
//
// `FinishedAt` NULL berarti percobaan masih berjalan (belum dikumpulkan), dan
// percobaan seperti itu tidak dihitung terhadap `MaxAttempts`.
type QuizAttempt struct {
	// Memakai IDBase (bukan Base): tabel ini tidak punya created_at/updated_at,
	// waktu hidupnya diwakili `started_at`/`finished_at`.
	IDBase
	QuizID     string               `gorm:"column:quiz_id;not null" json:"quizId"`
	UserID     string               `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	Score      int                  `gorm:"column:score;not null;default:0" json:"score"`
	Correct    int                  `gorm:"column:correct;not null;default:0" json:"correct"`
	Total      int                  `gorm:"column:total;not null;default:0" json:"total"`
	Passed     bool                 `gorm:"column:passed;not null;default:false" json:"passed"`
	Answers    JSONList[QuizAnswer] `gorm:"column:answers;type:jsonb;not null;default:'[]'" json:"answers"`
	StartedAt  time.Time            `gorm:"column:started_at;not null;default:now()" json:"startedAt"`
	FinishedAt *time.Time           `gorm:"column:finished_at" json:"finishedAt,omitempty"`
}

// TableName menetapkan nama tabel.
func (QuizAttempt) TableName() string { return "quiz_attempts" }

// IsFinished menandai percobaan sudah dikumpulkan.
func (a QuizAttempt) IsFinished() bool { return a.FinishedAt != nil }

/* ------------------------------- sertifikat -------------------------------- */

// Certificate adalah bukti penyelesaian sebuah jalur belajar.
//
// Nomornya unik (`BA-<tahun>-<6 digit>`) dan dapat diverifikasi publik, sehingga
// keasliannya bisa dicek tanpa membuka data peserta lain.
type Certificate struct {
	// Memakai IDBase (bukan Base): tabel ini tidak punya created_at/updated_at,
	// waktu terbitnya diwakili `issued_at`.
	IDBase
	Number    string     `gorm:"column:number;not null" json:"number"`
	UserID    string     `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	PathID    *string    `gorm:"column:path_id" json:"pathId,omitempty"`
	CohortID  *string    `gorm:"column:cohort_id" json:"cohortId,omitempty"`
	IssuedAt  time.Time  `gorm:"column:issued_at;not null;default:now()" json:"issuedAt"`
	RevokedAt *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	// Metadata memuat nama peserta, instansi, dan periode (dipakai PDF & laporan).
	Metadata JSONMap `gorm:"column:metadata;type:jsonb;not null;default:'{}'" json:"metadata"`
}

// TableName menetapkan nama tabel.
func (Certificate) TableName() string { return "certificates" }

// IsRevoked menandai sertifikat sudah dicabut.
func (c Certificate) IsRevoked() bool { return c.RevokedAt != nil }

// CertificateCounter menyimpan nomor sertifikat terakhir pada satu tahun.
//
// Baris ini dikunci (SELECT ... FOR UPDATE) saat nomor berikutnya diambil agar
// dua peserta yang lulus bersamaan tidak mendapat nomor kembar.
type CertificateCounter struct {
	Year       int       `gorm:"column:year;primaryKey" json:"year"`
	LastNumber int       `gorm:"column:last_number;not null;default:0" json:"lastNumber"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
}

// TableName menetapkan nama tabel.
func (CertificateCounter) TableName() string { return "certificate_counters" }

// CertificateNumber menyusun nomor sertifikat dari tahun & urutannya.
//
//	CertificateNumber(2026, 123) → "BA-2026-000123"
func CertificateNumber(year, sequence int) string {
	return fmt.Sprintf("BA-%04d-%06d", year, sequence)
}
