package dto

// CategoryScoreView adalah skor per subtes pada hasil.
type CategoryScoreView struct {
	Name    string `json:"name"`
	Score   int    `json:"score"`
	Correct int    `json:"correct"`
	Total   int    `json:"total"`
}

// AttemptView adalah satu baris riwayat tryout (bentuknya sama dengan tipe
// `Attempt` pada front-end).
type AttemptView struct {
	ID              string              `json:"id"`
	TryoutID        string              `json:"tryoutId"`
	Title           string              `json:"title"`
	TakenAt         string              `json:"takenAt"`
	Score           int                 `json:"score"`
	Correct         int                 `json:"correct"`
	Total           int                 `json:"total"`
	Status          string              `json:"status"`
	DurationSeconds int                 `json:"durationSeconds"`
	Categories      []CategoryScoreView `json:"categories"`
}

// ReviewQuestionView adalah pembahasan satu soal setelah sesi dikumpulkan.
type ReviewQuestionView struct {
	ID            string   `json:"id"`
	Number        int      `json:"number"`
	Text          string   `json:"text"`
	Options       []string `json:"options"`
	AnswerIndex   int      `json:"answerIndex"`
	SelectedIndex *int     `json:"selectedIndex"`
	Correct       bool     `json:"correct"`
	Flagged       bool     `json:"flagged"`
	Explanation   string   `json:"explanation"`
}

// ReviewSubtestView adalah pembahasan satu subtes.
type ReviewSubtestView struct {
	Name      string               `json:"name"`
	Score     int                  `json:"score"`
	Correct   int                  `json:"correct"`
	Total     int                  `json:"total"`
	Questions []ReviewQuestionView `json:"questions"`
}

// AttemptResultView adalah hasil lengkap satu sesi tryout.
type AttemptResultView struct {
	AttemptView
	PassingThreshold int                 `json:"passingThreshold"`
	Review           []ReviewSubtestView `json:"review,omitempty"`
}

// AttemptDetailView adalah detail riwayat lengkap dengan pembahasan.
type AttemptDetailView struct {
	AttemptResultView
	TryoutTitle string `json:"tryoutTitle"`
}

// LeaderboardEntry adalah satu baris papan peringkat.
type LeaderboardEntry struct {
	Rank            int    `json:"rank"`
	UserID          string `json:"userId"`
	Name            string `json:"name"`
	Initials        string `json:"initials"`
	Score           int    `json:"score"`
	Correct         int    `json:"correct"`
	Total           int    `json:"total"`
	DurationSeconds int    `json:"durationSeconds"`
	TakenAt         string `json:"takenAt"`
	IsMe            bool   `json:"isMe"`
}

// LeaderboardResponse adalah balasan papan peringkat.
type LeaderboardResponse struct {
	TryoutID string             `json:"tryoutId"`
	Entries  []LeaderboardEntry `json:"entries"`
	MyRank   *LeaderboardEntry  `json:"myRank,omitempty"`
}
