package models

import (
	"time"

	"gorm.io/gorm"
)

// LiveStatus adalah status sesi live.
type LiveStatus string

const (
	// LiveOngoing sesi sedang berlangsung.
	LiveOngoing LiveStatus = "live"
	// LiveUpcoming sesi akan datang.
	LiveUpcoming LiveStatus = "upcoming"
	// LiveEnded sesi sudah selesai.
	LiveEnded LiveStatus = "ended"
)

// LiveSession adalah jadwal kelas live.
type LiveSession struct {
	ID              string     `gorm:"column:id;primaryKey" json:"id"`
	Title           string     `gorm:"column:title;not null" json:"title"`
	Topic           string     `gorm:"column:topic;not null;default:''" json:"topic"`
	Mentor          string     `gorm:"column:mentor;not null;default:''" json:"mentor"`
	MentorRole      string     `gorm:"column:mentor_role;not null;default:''" json:"mentorRole"`
	StartsAt        time.Time  `gorm:"column:starts_at;not null" json:"startsAt"`
	DurationMinutes int        `gorm:"column:duration_minutes;not null;default:60" json:"durationMinutes"`
	Status          LiveStatus `gorm:"column:status;not null;default:upcoming" json:"status"`
	Platform        string     `gorm:"column:platform;not null;default:Ruang live BUMN Academy" json:"platform"`
	Note            *string    `gorm:"column:note" json:"note,omitempty"`
	MeetingURL      *string    `gorm:"column:meeting_url" json:"meetingUrl,omitempty"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (LiveSession) TableName() string { return "live_sessions" }

// Recording adalah rekaman kelas yang bisa ditonton ulang.
type Recording struct {
	ID              string    `gorm:"column:id;primaryKey" json:"id"`
	Title           string    `gorm:"column:title;not null" json:"title"`
	Description     string    `gorm:"column:description;not null;default:''" json:"description"`
	Topic           string    `gorm:"column:topic;not null;default:''" json:"topic"`
	Category        string    `gorm:"column:category;not null;default:TKD" json:"category"`
	Mentor          string    `gorm:"column:mentor;not null;default:''" json:"mentor"`
	DurationMinutes int       `gorm:"column:duration_minutes;not null;default:0" json:"durationMinutes"`
	PublishedAt     time.Time `gorm:"column:published_at;not null;autoCreateTime" json:"publishedAt"`
	Accent          string    `gorm:"column:accent;not null;default:from-brand/80 to-navy" json:"accent"`
	VideoURL        *string   `gorm:"column:video_url" json:"videoUrl,omitempty"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (Recording) TableName() string { return "recordings" }

// SessionReminder adalah pengingat peserta untuk sebuah sesi live.
type SessionReminder struct {
	UserID        string    `gorm:"column:user_id;type:uuid;primaryKey" json:"userId"`
	LiveSessionID string    `gorm:"column:live_session_id;primaryKey" json:"liveSessionId"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (SessionReminder) TableName() string { return "session_reminders" }

// BroadcastLog mencatat broadcast WhatsApp dari dashboard admin.
type BroadcastLog struct {
	ID         string     `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	CreatedBy  *string    `gorm:"column:created_by;type:uuid" json:"createdBy,omitempty"`
	Audience   string     `gorm:"column:audience;not null;default:all" json:"audience"`
	Message    string     `gorm:"column:message;not null" json:"message"`
	Recipients int        `gorm:"column:recipients;not null;default:0" json:"recipients"`
	Status     string     `gorm:"column:status;not null;default:queued" json:"status"`
	CreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	SentAt     *time.Time `gorm:"column:sent_at" json:"sentAt,omitempty"`
}

// TableName menetapkan nama tabel.
func (BroadcastLog) TableName() string { return "broadcast_logs" }

// BeforeCreate memastikan ID terisi.
func (b *BroadcastLog) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = NewID()
	}
	return nil
}
