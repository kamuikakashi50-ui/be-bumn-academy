package dto

// LiveSessionView adalah jadwal kelas live.
type LiveSessionView struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Topic           string `json:"topic"`
	Mentor          string `json:"mentor"`
	MentorRole      string `json:"mentorRole"`
	StartsAt        string `json:"startsAt"`
	DurationMinutes int    `json:"durationMinutes"`
	Status          string `json:"status"`
	Platform        string `json:"platform"`
	Note            string `json:"note,omitempty"`
	MeetingURL      string `json:"meetingUrl,omitempty"`
	Reminded        bool   `json:"reminded"`
}

// RecordingView adalah rekaman kelas.
type RecordingView struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Topic           string `json:"topic"`
	Category        string `json:"category"`
	Mentor          string `json:"mentor"`
	DurationMinutes int    `json:"durationMinutes"`
	PublishedAt     string `json:"publishedAt"`
	Accent          string `json:"accent"`
	VideoURL        string `json:"videoUrl,omitempty"`
}

// ReminderResponse adalah balasan toggle pengingat sesi live.
type ReminderResponse struct {
	LiveSessionID string `json:"liveSessionId"`
	Reminded      bool   `json:"reminded"`
}

/* --------------------------- dashboard peserta --------------------------- */

// DashboardSummary adalah ringkasan halaman utama dashboard peserta.
type DashboardSummary struct {
	TotalAttempts    int               `json:"totalAttempts"`
	AverageScore     int               `json:"averageScore"`
	BestScore        int               `json:"bestScore"`
	LastScore        int               `json:"lastScore"`
	PassedCount      int               `json:"passedCount"`
	PassingThreshold int               `json:"passingThreshold"`
	StudyStreakDays  int               `json:"studyStreakDays"`
	EnrolledSlugs    []string          `json:"enrolledSlugs"`
	ActiveTryouts    []TryoutSummary   `json:"activeTryouts"`
	RecentAttempts   []AttemptView     `json:"recentAttempts"`
	UpcomingLive     []LiveSessionView `json:"upcomingLive"`
	ScoreTrend       []ScoreTrendPoint `json:"scoreTrend"`
}

// ScoreTrendPoint adalah satu titik tren skor.
type ScoreTrendPoint struct {
	Label   string `json:"label"`
	Score   int    `json:"score"`
	TakenAt string `json:"takenAt"`
}
