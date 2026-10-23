package ai

import (
	"fmt"
	"strings"
)

/* ---------------------------------- util ---------------------------------- */

func estimateUsage(req Request, content string) Usage {
	promptChars := 0
	for _, message := range req.Messages {
		promptChars += len(message.Content)
	}
	// Perkiraan kasar 4 karakter per token; cukup untuk kuota internal.
	promptTokens := promptChars / 4
	completionTokens := len(content) / 4
	return Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}

func splitForStreaming(content string) []string {
	lines := strings.SplitAfter(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 && content != "" {
		out = append(out, content)
	}
	return out
}

func optionText(question QuestionContext, index int) string {
	if index < 0 || index >= len(question.Options) {
		return "(opsi tidak tersedia)"
	}
	return question.Options[index]
}

func firstLine(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return strings.TrimSpace(text[:idx])
	}
	return strings.TrimSpace(text)
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func humanDuration(seconds int) string {
	minutes := seconds / 60
	rest := seconds % 60
	if minutes == 0 {
		return fmt.Sprintf("%d detik", rest)
	}
	return fmt.Sprintf("%d menit %d detik", minutes, rest)
}

// difficultyNote menjelaskan mengapa sebuah soal cenderung menyita waktu.
func difficultyNote(question QuestionContext) string {
	switch question.Difficulty {
	case "sulit":
		return "termasuk soal sulit — kalau lebih dari 2 menit belum ketemu polanya, tandai lalu lanjut"
	case "mudah":
		return "sebenarnya soal mudah, jadi sayang kalau dilewatkan"
	default:
		return "tergolong sedang, cukup 60–90 detik untuk memutuskan"
	}
}
