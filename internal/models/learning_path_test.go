package models

import (
	"testing"
	"time"
)

// Uji logika model Fase 1 yang murni perhitungan: susunan langkah jalur, penilaian
// kuis, batas percobaan, dan penomoran sertifikat.

func TestRequiredModuleIDsHanyaMengambilLangkahWajib(t *testing.T) {
	path := LearningPath{
		Steps: []LearningPathStep{
			{ModuleID: "mod-a", Required: true, SortOrder: 0},
			{ModuleID: "mod-b", Required: false, SortOrder: 1},
			{ModuleID: "mod-c", Required: true, SortOrder: 2},
		},
	}

	got := path.RequiredModuleIDs()
	if len(got) != 2 {
		t.Fatalf("jumlah modul wajib = %d, harap 2 (%v)", len(got), got)
	}
	if got[0] != "mod-a" || got[1] != "mod-c" {
		t.Errorf("urutan modul wajib harus mengikuti langkah: %v", got)
	}

	if ids := (LearningPath{}).RequiredModuleIDs(); len(ids) != 0 {
		t.Errorf("jalur tanpa langkah harus menghasilkan daftar kosong, dapat %v", ids)
	}
}

func TestLessonQuizScoreFor(t *testing.T) {
	quiz := LessonQuiz{PassScore: 70}

	cases := []struct {
		correct, total, want int
	}{
		{0, 0, 0},     // kuis tanpa soal → 0, bukan pembagian tak hingga
		{0, 10, 0},    // tidak ada jawaban benar
		{7, 10, 70},   // tepat di ambang
		{8, 10, 80},   // di atas ambang
		{1, 3, 33},    // pembulatan ke bawah
		{2, 3, 66},    // pembulatan ke bawah
		{10, 10, 100}, // sempurna
	}

	for _, tc := range cases {
		if got := quiz.ScoreFor(tc.correct, tc.total); got != tc.want {
			t.Errorf("ScoreFor(%d, %d) = %d, harap %d", tc.correct, tc.total, got, tc.want)
		}
	}
}

func TestLessonQuizPassedWithDanAttempts(t *testing.T) {
	quiz := LessonQuiz{PassScore: 70, MaxAttempts: 2}

	if quiz.PassedWith(69) {
		t.Error("skor di bawah ambang tidak boleh lulus")
	}
	if !quiz.PassedWith(70) {
		t.Error("skor tepat di ambang harus lulus")
	}

	if !quiz.AttemptsAllowed(0) {
		t.Error("percobaan pertama harus diizinkan")
	}
	if !quiz.AttemptsAllowed(1) {
		t.Error("percobaan kedua (max 2) harus diizinkan")
	}
	if quiz.AttemptsAllowed(2) {
		t.Error("percobaan melebihi max_attempts harus ditolak")
	}

	unlimited := LessonQuiz{MaxAttempts: 0}
	if !unlimited.AttemptsAllowed(99) {
		t.Error("max_attempts = 0 berarti tanpa batas")
	}
}

func TestQuizAttemptIsFinished(t *testing.T) {
	attempt := QuizAttempt{}
	if attempt.IsFinished() {
		t.Error("percobaan tanpa finished_at belum selesai")
	}

	now := time.Now().UTC()
	attempt.FinishedAt = &now
	if !attempt.IsFinished() {
		t.Error("percobaan dengan finished_at harus dianggap selesai")
	}
}

func TestCertificateIsRevoked(t *testing.T) {
	certificate := Certificate{Number: "BA-2026-000001"}
	if certificate.IsRevoked() {
		t.Error("sertifikat baru belum dicabut")
	}

	now := time.Now().UTC()
	certificate.RevokedAt = &now
	if !certificate.IsRevoked() {
		t.Error("sertifikat dengan revoked_at harus dianggap dicabut")
	}
}

func TestCertificateNumber(t *testing.T) {
	cases := map[[2]int]string{
		{2026, 1}:      "BA-2026-000001",
		{2026, 123}:    "BA-2026-000123",
		{2026, 999999}: "BA-2026-999999",
		{2030, 42}:     "BA-2030-000042",
	}
	for input, want := range cases {
		if got := CertificateNumber(input[0], input[1]); got != want {
			t.Errorf("CertificateNumber(%d, %d) = %q, harap %q", input[0], input[1], got, want)
		}
	}
}
