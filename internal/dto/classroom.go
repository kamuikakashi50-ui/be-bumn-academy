package dto

/* ----------------------------- ruang kelas ------------------------------ */

// LessonProgressView adalah progres seorang peserta pada satu pelajaran.
type LessonProgressView struct {
	PositionSeconds int    `json:"positionSeconds"`
	ProgressPercent int    `json:"progressPercent"`
	Completed       bool   `json:"completed"`
	LastViewedAt    string `json:"lastViewedAt,omitempty"`
}

// LessonRefView adalah rujukan ringkas sebuah pelajaran (untuk navigasi).
type LessonRefView struct {
	LessonID        string `json:"lessonId"`
	ModuleID        string `json:"moduleId"`
	ModuleTitle     string `json:"moduleTitle"`
	ProductSlug     string `json:"productSlug"`
	Title           string `json:"title"`
	Kind            string `json:"kind"`
	DurationSeconds int    `json:"durationSeconds"`
	PositionSeconds int    `json:"positionSeconds"`
	ProgressPercent int    `json:"progressPercent"`
	Completed       bool   `json:"completed"`
}

// LessonView adalah satu materi belajar pada ruang kelas.
type LessonView struct {
	ID              string             `json:"id"`
	ModuleID        string             `json:"moduleId"`
	ModuleTitle     string             `json:"moduleTitle"`
	ProductSlug     string             `json:"productSlug"`
	Title           string             `json:"title"`
	Summary         string             `json:"summary"`
	Kind            string             `json:"kind"`
	VideoURL        string             `json:"videoUrl,omitempty"`
	EbookURL        string             `json:"ebookUrl,omitempty"`
	PosterURL       string             `json:"posterUrl,omitempty"`
	EbookAvailable  bool               `json:"ebookAvailable"`
	Content         string             `json:"content,omitempty"`
	DurationSeconds int                `json:"durationSeconds"`
	SortOrder       int                `json:"sortOrder"`
	Active          bool               `json:"active"`
	Progress        LessonProgressView `json:"progress"`
}

// ModuleView adalah modul beserta pelajaran dan ringkasan progresnya.
type ModuleView struct {
	ID               string       `json:"id"`
	ProductSlug      string       `json:"productSlug"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	SortOrder        int          `json:"sortOrder"`
	Active           bool         `json:"active"`
	Lessons          []LessonView `json:"lessons"`
	TotalLessons     int          `json:"totalLessons"`
	CompletedLessons int          `json:"completedLessons"`
	ProgressPercent  int          `json:"progressPercent"`
}

// CourseView adalah ringkasan ruang kelas satu produk.
type CourseView struct {
	ProductSlug          string         `json:"productSlug"`
	ProductName          string         `json:"productName"`
	Description          string         `json:"description,omitempty"`
	Badge                string         `json:"badge,omitempty"`
	TotalLessons         int            `json:"totalLessons"`
	CompletedLessons     int            `json:"completedLessons"`
	ProgressPercent      int            `json:"progressPercent"`
	TotalDurationSeconds int            `json:"totalDurationSeconds"`
	LastLesson           *LessonRefView `json:"lastLesson,omitempty"`
}

// ClassroomView adalah isi lengkap satu produk di ruang kelas.
type ClassroomView struct {
	Course   CourseView      `json:"course"`
	Modules  []ModuleView    `json:"modules"`
	Sequence []LessonRefView `json:"sequence"`
}

// LessonDetailView adalah detail satu pelajaran beserta navigasi.
type LessonDetailView struct {
	Lesson   LessonView     `json:"lesson"`
	Course   CourseView     `json:"course"`
	Previous *LessonRefView `json:"previous,omitempty"`
	Next     *LessonRefView `json:"next,omitempty"`
}

// SaveProgressRequest adalah payload penyimpanan progres menonton/membaca.
//
// Persentase dihitung ulang di server dari durasi pelajaran, sehingga klien tidak
// bisa memalsukan progres penyelesaian.
type SaveProgressRequest struct {
	PositionSeconds *int  `json:"positionSeconds" validate:"omitempty,gte=0,lte=86400"`
	ProgressPercent *int  `json:"progressPercent" validate:"omitempty,gte=0,lte=100"`
	Completed       *bool `json:"completed"`
}

/* ------------------------- ruang kelas (admin) -------------------------- */

// ModuleInput adalah payload tambah modul ruang kelas.
type ModuleInput struct {
	ID          string `json:"id" validate:"omitempty,max=64"`
	ProductSlug string `json:"productSlug" validate:"required,max=80"`
	Title       string `json:"title" validate:"required,min=3,max=180"`
	Description string `json:"description" validate:"omitempty,max=1000"`
	SortOrder   int    `json:"sortOrder" validate:"gte=0,lte=999"`
	Active      *bool  `json:"active"`
}

// ModulePatch adalah payload ubah sebagian modul.
type ModulePatch struct {
	ProductSlug *string `json:"productSlug" validate:"omitempty,max=80"`
	Title       *string `json:"title" validate:"omitempty,min=3,max=180"`
	Description *string `json:"description" validate:"omitempty,max=1000"`
	SortOrder   *int    `json:"sortOrder" validate:"omitempty,gte=0,lte=999"`
	Active      *bool   `json:"active"`
}

// LessonInput adalah payload tambah pelajaran.
type LessonInput struct {
	ID              string `json:"id" validate:"omitempty,max=64"`
	ModuleID        string `json:"moduleId" validate:"required,max=64"`
	Title           string `json:"title" validate:"required,min=3,max=180"`
	Summary         string `json:"summary" validate:"omitempty,max=1000"`
	Kind            string `json:"kind" validate:"required,oneof=video ebook"`
	VideoURL        string `json:"videoUrl" validate:"omitempty,max=500"`
	EbookURL        string `json:"ebookUrl" validate:"omitempty,max=500"`
	PosterURL       string `json:"posterUrl" validate:"omitempty,max=500"`
	Content         string `json:"content" validate:"omitempty,max=20000"`
	DurationSeconds int    `json:"durationSeconds" validate:"gte=0,lte=86400"`
	SortOrder       int    `json:"sortOrder" validate:"gte=0,lte=999"`
	Active          *bool  `json:"active"`
}

// LessonPatch adalah payload ubah sebagian pelajaran.
type LessonPatch struct {
	ModuleID        *string `json:"moduleId" validate:"omitempty,max=64"`
	Title           *string `json:"title" validate:"omitempty,min=3,max=180"`
	Summary         *string `json:"summary" validate:"omitempty,max=1000"`
	Kind            *string `json:"kind" validate:"omitempty,oneof=video ebook"`
	VideoURL        *string `json:"videoUrl" validate:"omitempty,max=500"`
	EbookURL        *string `json:"ebookUrl" validate:"omitempty,max=500"`
	PosterURL       *string `json:"posterUrl" validate:"omitempty,max=500"`
	Content         *string `json:"content" validate:"omitempty,max=20000"`
	DurationSeconds *int    `json:"durationSeconds" validate:"omitempty,gte=0,lte=86400"`
	SortOrder       *int    `json:"sortOrder" validate:"omitempty,gte=0,lte=999"`
	Active          *bool   `json:"active"`
}
