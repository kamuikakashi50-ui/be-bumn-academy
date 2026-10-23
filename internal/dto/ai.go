package dto

// AINoteView adalah satu butir catatan mentor AI.
type AINoteView struct {
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Subtest    string `json:"subtest,omitempty"`
	QuestionID string `json:"questionId,omitempty"`
	Priority   int    `json:"priority,omitempty"`
}

// AIAnalysisView adalah analisis hasil tryout dari mentor AI.
type AIAnalysisView struct {
	AttemptID        string       `json:"attemptId"`
	Headline         string       `json:"headline"`
	Summary          string       `json:"summary"`
	Readiness        string       `json:"readiness"`
	ReadinessScore   int          `json:"readinessScore"`
	PassingThreshold int          `json:"passingThreshold"`
	Strengths        []AINoteView `json:"strengths"`
	Weaknesses       []AINoteView `json:"weaknesses"`
	FocusAreas       []AINoteView `json:"focusAreas"`
	StudyPlan        []AINoteView `json:"studyPlan"`
	QuestionNotes    []AINoteView `json:"questionNotes"`
	GeneratedAt      string       `json:"generatedAt"`
	Provider         string       `json:"provider"`
	Model            string       `json:"model"`
	// Cached menandakan analisis diambil dari hasil yang sudah pernah dibuat.
	Cached bool `json:"cached"`
	// Degraded menandakan analisis dibuat mentor lokal karena provider utama gagal.
	Degraded bool `json:"degraded"`
	// Suggestions adalah pertanyaan lanjutan yang bisa diklik peserta.
	Suggestions []string `json:"suggestions"`
}

// AIChatRequest adalah pertanyaan peserta pada panel mentor.
type AIChatRequest struct {
	Message string `json:"message" validate:"required,min=2,max=1500"`
	// QuestionID opsional: bila diisi, mentor membahas soal tersebut.
	QuestionID string `json:"questionId" validate:"omitempty,max=64"`
}

// AIMessageView adalah satu pesan pada percakapan mentor.
type AIMessageView struct {
	ID             string `json:"id"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	Scope          string `json:"scope"`
	QuestionID     string `json:"questionId,omitempty"`
	QuestionNumber *int   `json:"questionNumber,omitempty"`
	SubtestName    string `json:"subtestName,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	LatencyMS      int    `json:"latencyMs,omitempty"`
	CreatedAt      string `json:"createdAt"`
}

// AIUsageView adalah pemakaian kuota mentor AI peserta.
type AIUsageView struct {
	Date             string `json:"date"`
	Requests         int    `json:"requests"`
	DailyQuota       int    `json:"dailyQuota"`
	Remaining        int    `json:"remaining"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
}

// AIConversationView adalah riwayat percakapan mentor untuk satu hasil tryout.
type AIConversationView struct {
	AttemptID    string          `json:"attemptId"`
	Title        string          `json:"title"`
	Messages     []AIMessageView `json:"messages"`
	MessageCount int             `json:"messageCount"`
	Usage        AIUsageView     `json:"usage"`
	Suggestions  []string        `json:"suggestions"`
}

// AIChatResponse adalah balasan mentor untuk satu pertanyaan.
type AIChatResponse struct {
	Message     AIMessageView `json:"message"`
	Usage       AIUsageView   `json:"usage"`
	Degraded    bool          `json:"degraded"`
	Suggestions []string      `json:"suggestions"`
}

// AIStatusView merangkum kesiapan fitur mentor AI.
type AIStatusView struct {
	Enabled          bool   `json:"enabled"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	StreamingEnabled bool   `json:"streamingEnabled"`
	DailyQuota       int    `json:"dailyQuota"`
	Degraded         bool   `json:"degraded"`
	LastError        string `json:"lastError,omitempty"`
}

// AIQuestionContextView adalah pratinjau soal yang akan dibahas mentor.
type AIQuestionContextView struct {
	QuestionID    string   `json:"questionId"`
	Number        int      `json:"number"`
	SubtestName   string   `json:"subtestName"`
	Text          string   `json:"text"`
	Options       []string `json:"options"`
	CorrectIndex  int      `json:"correctIndex"`
	SelectedIndex *int     `json:"selectedIndex"`
	IsCorrect     bool     `json:"isCorrect"`
	IsUnanswered  bool     `json:"isUnanswered"`
	Explanation   string   `json:"explanation"`
}
