// Package service berisi aturan bisnis aplikasi.
//
// Service menjadi satu-satunya tempat yang menggabungkan repository, cache, dan
// konfigurasi; handler hanya menangani HTTP.
package service

import (
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// formatTime membentuk waktu ISO-8601 (RFC3339) untuk klien; string kosong
// bila waktu nol.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// formatTimePtr membentuk waktu ISO-8601 dari pointer.
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func strPtrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// emptyIfNil memastikan slice JSON selalu terkirim sebagai array (bukan null)
// agar front-end tidak perlu menangani dua bentuk.
func emptyIfNil[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

/* --------------------------------- user ---------------------------------- */

func toUserView(user models.User) dto.UserView {
	return dto.UserView{
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		Role:         string(user.Role),
		Program:      user.Program,
		Initials:     user.Initials(),
		ReferralCode: strPtrOrEmpty(user.ReferralCode),
	}
}

/* -------------------------------- katalog -------------------------------- */

func toProgramView(program models.Program) dto.ProgramView {
	return dto.ProgramView{
		Slug:        program.Slug,
		Name:        program.Name,
		Tagline:     program.Tagline,
		Audience:    program.Audience,
		Format:      program.Format,
		Duration:    program.Duration,
		Features:    emptyIfNil(program.Features),
		Highlighted: program.Highlighted,
	}
}

func toMateriView(category models.MateriCategory) dto.MateriCategoryView {
	return dto.MateriCategoryView{
		ID:          category.ID,
		Label:       category.Label,
		Description: category.Description,
		Topics:      emptyIfNil(category.Topics),
	}
}

func toProductView(product models.Product, owned bool) dto.ProductView {
	return dto.ProductView{
		ID:          product.ID,
		Slug:        product.Slug,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		Type:        string(product.Type),
		Includes:    emptyIfNil(product.Includes),
		Badge:       product.Badge,
		Owned:       owned,
	}
}

func toPaymentMethodView(method models.PaymentMethod) dto.PaymentMethodView {
	return dto.PaymentMethodView{
		ID:      method.ID,
		Label:   method.Label,
		Channel: method.Channel,
		Fee:     method.Fee,
	}
}

func toVoucherView(voucher models.Voucher) dto.VoucherView {
	return dto.VoucherView{
		Code:        voucher.Code,
		Label:       voucher.Label,
		Type:        string(voucher.Type),
		Value:       voucher.Value,
		MinPurchase: voucher.MinPurchase,
		MaxUses:     voucher.MaxUses,
		UsedCount:   voucher.UsedCount,
		Active:      voucher.Active,
	}
}

/* -------------------------------- billing -------------------------------- */

func toOrderView(order models.Order) dto.OrderView {
	return dto.OrderView{
		ID:               order.ID,
		InvoiceNo:        order.InvoiceNo,
		ProductSlug:      order.ProductSlug,
		ProductName:      order.ProductName,
		Price:            order.Price,
		Discount:         order.Discount,
		Total:            order.Total,
		VoucherCode:      strPtrOrEmpty(order.VoucherCode),
		ReferralCode:     strPtrOrEmpty(order.ReferralCode),
		Method:           order.Method,
		Status:           string(order.Status),
		CreatedAt:        formatTime(order.CreatedAt),
		PaidAt:           formatTimePtr(order.PaidAt),
		ExpiresAt:        formatTimePtr(order.ExpiresAt),
		PaymentReference: strPtrOrEmpty(order.PaymentReference),
	}
}

func toEnrollmentView(enrollment models.Enrollment) dto.EnrollmentView {
	return dto.EnrollmentView{
		ProductSlug: enrollment.ProductSlug,
		Source:      enrollment.Source,
		CreatedAt:   formatTime(enrollment.CreatedAt),
	}
}

/* -------------------------------- tryout --------------------------------- */

func toSubtestView(subtest models.Subtest, questionCount int) dto.SubtestView {
	return dto.SubtestView{
		Name:          subtest.Name,
		Banks:         emptyIfNil(subtest.Banks),
		Minutes:       subtest.Minutes,
		QuestionCount: questionCount,
	}
}

func toAttemptView(attempt models.Attempt) dto.AttemptView {
	categories := make([]dto.CategoryScoreView, 0, len(attempt.Categories))
	for _, category := range attempt.Categories {
		categories = append(categories, dto.CategoryScoreView{
			Name:    category.Name,
			Score:   category.Score,
			Correct: category.Correct,
			Total:   category.Total,
		})
	}
	return dto.AttemptView{
		ID:              attempt.ID,
		TryoutID:        attempt.TryoutID,
		Title:           attempt.Title,
		TakenAt:         formatTime(attempt.TakenAt),
		Score:           attempt.Score,
		Correct:         attempt.Correct,
		Total:           attempt.Total,
		Status:          string(attempt.Status),
		DurationSeconds: attempt.DurationSeconds,
		Categories:      categories,
	}
}

func toQuestionView(question models.Question) dto.QuestionView {
	return dto.QuestionView{
		ID:          question.ID,
		BankKey:     question.BankKey,
		BankLabel:   bank.Label(question.BankKey),
		Text:        question.Text,
		Options:     emptyIfNil(question.Options),
		AnswerIndex: question.AnswerIndex,
		Explanation: question.Explanation,
		Difficulty:  string(question.Difficulty),
		Active:      question.Active,
	}
}

/* ------------------------------- learning -------------------------------- */

func toLiveSessionView(session models.LiveSession, reminded bool) dto.LiveSessionView {
	return dto.LiveSessionView{
		ID:              session.ID,
		Title:           session.Title,
		Topic:           session.Topic,
		Mentor:          session.Mentor,
		MentorRole:      session.MentorRole,
		StartsAt:        formatTime(session.StartsAt),
		DurationMinutes: session.DurationMinutes,
		Status:          string(session.Status),
		Platform:        session.Platform,
		Note:            strPtrOrEmpty(session.Note),
		MeetingURL:      strPtrOrEmpty(session.MeetingURL),
		Reminded:        reminded,
	}
}

func toRecordingView(recording models.Recording) dto.RecordingView {
	return dto.RecordingView{
		ID:              recording.ID,
		Title:           recording.Title,
		Description:     recording.Description,
		Topic:           recording.Topic,
		Category:        recording.Category,
		Mentor:          recording.Mentor,
		DurationMinutes: recording.DurationMinutes,
		PublishedAt:     formatTime(recording.PublishedAt),
		Accent:          recording.Accent,
		VideoURL:        strPtrOrEmpty(recording.VideoURL),
	}
}

// bankLabelOf mengubah kunci bank soal menjadi label yang dibaca peserta.
func bankLabelOf(bankKey string) string {
	return bank.Label(bankKey)
}

func toLeaderboardEntry(row repository.LeaderboardRow, meID string) dto.LeaderboardEntry {
	return dto.LeaderboardEntry{
		Rank:            row.Rank,
		UserID:          row.UserID,
		Name:            row.Name,
		Initials:        initialsOf(row.Name),
		Score:           row.Score,
		Correct:         row.Correct,
		Total:           row.Total,
		DurationSeconds: row.DurationSeconds,
		TakenAt:         formatTime(row.TakenAt),
		IsMe:            row.UserID == meID,
	}
}

// initialsOf meniru logika inisial pada front-end (maksimal 2 huruf pertama).
func initialsOf(name string) string {
	parts := strings.Fields(name)
	out := make([]string, 0, 2)
	for _, part := range parts {
		if len(out) == 2 {
			break
		}
		if part == "" {
			continue
		}
		out = append(out, strings.ToUpper(string([]rune(part)[0])))
	}
	return strings.Join(out, "")
}
