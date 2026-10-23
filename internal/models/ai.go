package models

import (
	"time"

	"gorm.io/gorm"
)

// AINote adalah satu butir catatan mentor AI (judul + penjelasan).
//
// Dipakai untuk daftar kekuatan, kelemahan, area fokus, dan langkah belajar
// pada analisis hasil tryout.
type AINote struct {
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Subtest    string `json:"subtest,omitempty"`
	QuestionID string `json:"questionId,omitempty"`
	Priority   int    `json:"priority,omitempty"`
}

// AIReadiness adalah kesiapan peserta berdasarkan hasil satu tryout.
type AIReadiness string

const (
	// AIReady skor jauh di atas ambang.
	AIReady AIReadiness = "siap"
	// AIAlmostReady skor sedikit di atas ambang.
	AIAlmostReady AIReadiness = "hampir siap"
	// AINeedsWork skor di bawah ambang.
	AINeedsWork AIReadiness = "perlu perbaikan"
)

// AttemptAnalysis adalah analisis mentor AI untuk satu hasil tryout.
//
// Satu attempt hanya punya satu analisis (unique index) sehingga pembuatan
// analisis bisa idempotent dan hasilnya di-cache.
type AttemptAnalysis struct {
	ID             string           `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	AttemptID      string           `gorm:"column:attempt_id;type:uuid;not null" json:"attemptId"`
	UserID         string           `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	Provider       string           `gorm:"column:provider;not null;default:stub" json:"provider"`
	Model          string           `gorm:"column:model;not null;default:''" json:"model"`
	Headline       string           `gorm:"column:headline;not null;default:''" json:"headline"`
	Summary        string           `gorm:"column:summary;not null;default:''" json:"summary"`
	Readiness      AIReadiness      `gorm:"column:readiness;not null;default:perlu perbaikan" json:"readiness"`
	ReadinessScore int              `gorm:"column:readiness_score;not null;default:0" json:"readinessScore"`
	Strengths      JSONList[AINote] `gorm:"column:strengths;type:jsonb;not null;default:'[]'" json:"strengths"`
	Weaknesses     JSONList[AINote] `gorm:"column:weaknesses;type:jsonb;not null;default:'[]'" json:"weaknesses"`
	FocusAreas     JSONList[AINote] `gorm:"column:focus_areas;type:jsonb;not null;default:'[]'" json:"focusAreas"`
	StudyPlan      JSONList[AINote] `gorm:"column:study_plan;type:jsonb;not null;default:'[]'" json:"studyPlan"`
	QuestionNotes  JSONList[AINote] `gorm:"column:question_notes;type:jsonb;not null;default:'[]'" json:"questionNotes"`
	GeneratedAt    time.Time        `gorm:"column:generated_at;not null;autoCreateTime" json:"generatedAt"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (AttemptAnalysis) TableName() string { return "attempt_analyses" }

// BeforeCreate memastikan ID terisi.
func (a *AttemptAnalysis) BeforeCreate(*gorm.DB) error {
	if a.ID == "" {
		a.ID = NewID()
	}
	return nil
}

// AIConversation adalah percakapan mentor untuk satu hasil tryout.
type AIConversation struct {
	ID            string     `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	AttemptID     string     `gorm:"column:attempt_id;type:uuid;not null" json:"attemptId"`
	UserID        string     `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	Title         string     `gorm:"column:title;not null;default:Ulasan Mentor AI" json:"title"`
	MessageCount  int        `gorm:"column:message_count;not null;default:0" json:"messageCount"`
	LastMessageAt *time.Time `gorm:"column:last_message_at" json:"lastMessageAt,omitempty"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (AIConversation) TableName() string { return "ai_conversations" }

// BeforeCreate memastikan ID terisi.
func (c *AIConversation) BeforeCreate(*gorm.DB) error {
	if c.ID == "" {
		c.ID = NewID()
	}
	return nil
}

// AIRole adalah peran pengirim pesan pada percakapan mentor.
type AIRole string

const (
	// AIRoleUser pesan dari peserta.
	AIRoleUser AIRole = "user"
	// AIRoleAssistant pesan dari mentor AI.
	AIRoleAssistant AIRole = "assistant"
)

// AIScope menandakan cakupan pembahasan sebuah pesan.
type AIScope string

const (
	// AIScopeAttempt membahas keseluruhan hasil tryout.
	AIScopeAttempt AIScope = "attempt"
	// AIScopeQuestion membahas satu soal tertentu.
	AIScopeQuestion AIScope = "question"
)

// AIMessage adalah satu pesan pada percakapan mentor.
type AIMessage struct {
	ID               string    `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	ConversationID   string    `gorm:"column:conversation_id;type:uuid;not null" json:"conversationId"`
	UserID           string    `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	Role             AIRole    `gorm:"column:role;not null" json:"role"`
	Content          string    `gorm:"column:content;not null" json:"content"`
	Scope            AIScope   `gorm:"column:scope;not null;default:attempt" json:"scope"`
	QuestionID       *string   `gorm:"column:question_id" json:"questionId,omitempty"`
	QuestionNumber   *int      `gorm:"column:question_number" json:"questionNumber,omitempty"`
	SubtestName      *string   `gorm:"column:subtest_name" json:"subtestName,omitempty"`
	Provider         string    `gorm:"column:provider;not null;default:''" json:"provider,omitempty"`
	Model            string    `gorm:"column:model;not null;default:''" json:"model,omitempty"`
	PromptTokens     int       `gorm:"column:prompt_tokens;not null;default:0" json:"promptTokens,omitempty"`
	CompletionTokens int       `gorm:"column:completion_tokens;not null;default:0" json:"completionTokens,omitempty"`
	LatencyMS        int       `gorm:"column:latency_ms;not null;default:0" json:"latencyMs,omitempty"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

// TableName menetapkan nama tabel.
func (AIMessage) TableName() string { return "ai_messages" }

// BeforeCreate memastikan ID terisi.
func (m *AIMessage) BeforeCreate(*gorm.DB) error {
	if m.ID == "" {
		m.ID = NewID()
	}
	return nil
}

// AIUsageCounter menghitung pemakaian AI harian per peserta.
type AIUsageCounter struct {
	UserID           string    `gorm:"column:user_id;type:uuid;primaryKey" json:"userId"`
	UsageDate        time.Time `gorm:"column:usage_date;type:date;primaryKey" json:"usageDate"`
	Requests         int       `gorm:"column:requests;not null;default:0" json:"requests"`
	PromptTokens     int       `gorm:"column:prompt_tokens;not null;default:0" json:"promptTokens"`
	CompletionTokens int       `gorm:"column:completion_tokens;not null;default:0" json:"completionTokens"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

// TableName menetapkan nama tabel.
func (AIUsageCounter) TableName() string { return "ai_usage_counters" }
