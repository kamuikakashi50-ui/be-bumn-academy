package models

import (
	"time"

	"gorm.io/gorm"
)

// LessonKind adalah jenis materi pada satu pelajaran.
type LessonKind string

const (
	// LessonVideo adalah materi video (MP4/HLS) yang ditonton di player.
	LessonVideo LessonKind = "video"
	// LessonEbook adalah e-book PDF yang dibaca langsung di browser.
	LessonEbook LessonKind = "ebook"
)

// Valid memeriksa apakah jenis pelajaran dikenal.
func (k LessonKind) Valid() bool { return k == LessonVideo || k == LessonEbook }

// CourseModule adalah satu bab/modul pada ruang kelas sebuah produk.
type CourseModule struct {
	ID          string `gorm:"column:id;primaryKey" json:"id"`
	ProductSlug string `gorm:"column:product_slug;not null" json:"productSlug"`
	Title       string `gorm:"column:title;not null" json:"title"`
	Description string `gorm:"column:description;not null;default:''" json:"description"`
	SortOrder   int    `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	Active      bool   `gorm:"column:active;not null;default:true" json:"active"`
	// Lessons diisi lewat Preload saat memuat modul.
	Lessons []Lesson `gorm:"foreignKey:ModuleID" json:"lessons,omitempty"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (CourseModule) TableName() string { return "course_modules" }

// Lesson adalah satu materi belajar (video atau e-book) di dalam modul.
//
// `ProductSlug` diduplikasi dari modul agar pemeriksaan kepemilikan produk dan
// daftar "lanjutkan belajar" bisa dijalankan tanpa join tambahan.
type Lesson struct {
	ID              string     `gorm:"column:id;primaryKey" json:"id"`
	ModuleID        string     `gorm:"column:module_id;not null" json:"moduleId"`
	ProductSlug     string     `gorm:"column:product_slug;not null" json:"productSlug"`
	Title           string     `gorm:"column:title;not null" json:"title"`
	Summary         string     `gorm:"column:summary;not null;default:''" json:"summary"`
	Kind            LessonKind `gorm:"column:kind;not null;default:video" json:"kind"`
	VideoURL        *string    `gorm:"column:video_url" json:"videoUrl,omitempty"`
	EbookURL        *string    `gorm:"column:ebook_url" json:"ebookUrl,omitempty"`
	PosterURL       *string    `gorm:"column:poster_url" json:"posterUrl,omitempty"`
	Content         string     `gorm:"column:content;not null;default:''" json:"content"`
	DurationSeconds int        `gorm:"column:duration_seconds;not null;default:0" json:"durationSeconds"`
	SortOrder       int        `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	Active          bool       `gorm:"column:active;not null;default:true" json:"active"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (Lesson) TableName() string { return "lessons" }

// LessonProgress mencatat posisi menonton/baca seorang peserta pada satu pelajaran.
//
// Memakai kunci gabungan (user_id, lesson_id) sehingga satu peserta hanya punya
// satu baris progres per pelajaran.
type LessonProgress struct {
	UserID          string    `gorm:"column:user_id;type:uuid;primaryKey" json:"userId"`
	LessonID        string    `gorm:"column:lesson_id;primaryKey" json:"lessonId"`
	PositionSeconds int       `gorm:"column:position_seconds;not null;default:0" json:"positionSeconds"`
	ProgressPercent int       `gorm:"column:progress_percent;not null;default:0" json:"progressPercent"`
	Completed       bool      `gorm:"column:completed;not null;default:false" json:"completed"`
	LastViewedAt    time.Time `gorm:"column:last_viewed_at;not null;default:now()" json:"lastViewedAt"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

// TableName menetapkan nama tabel.
func (LessonProgress) TableName() string { return "lesson_progress" }

// BeforeCreate memastikan waktu tayang terisi.
func (p *LessonProgress) BeforeCreate(*gorm.DB) error {
	if p.LastViewedAt.IsZero() {
		p.LastViewedAt = time.Now().UTC()
	}
	return nil
}
