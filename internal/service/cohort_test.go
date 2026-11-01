package service

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/ebook"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// Uji logika kohort yang murni perhitungan: kode undangan, kelayakan gabung kelas,
// validasi kuota, pembentukan slug/kode kelas, dan pembacaan berkas anggota massal.
// Semuanya tanpa database supaya aturan bisnis §4.4 bisa diuji cepat.

func TestGenerateInviteCodeMengikutiFormat(t *testing.T) {
	// Sumber acak disuntikkan agar hasilnya bisa diprediksi.
	source := bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	code, err := generateInviteCode(source)
	if err != nil {
		t.Fatalf("generateInviteCode error: %v", err)
	}
	if len(code) != inviteCodeLength {
		t.Fatalf("panjang kode = %d, harap %d", len(code), inviteCodeLength)
	}
	if code != inviteAlphabet[:8] {
		t.Errorf("kode = %q, harap 8 karakter pertama alfabet %q", code, inviteAlphabet[:8])
	}
	for _, char := range code {
		if strings.ContainsRune("01OI", char) {
			t.Errorf("kode %q memuat karakter ambigu %q", code, char)
		}
	}

	// Sumber acak yang tidak cukup harus menghasilkan error, bukan kode pendek.
	if _, err := generateInviteCode(bytes.NewReader([]byte{1, 2})); err == nil {
		t.Error("sumber acak yang kurang harus menghasilkan error")
	}
}

func TestNewInviteCodeSelaluDelapanKarakterTanpaAmbiguitas(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 50; i++ {
		code, err := newInviteCode()
		if err != nil {
			t.Fatalf("newInviteCode error: %v", err)
		}
		if err := validateInviteCodeFormat(code); err != nil {
			t.Fatalf("kode yang dibuat server harus lolos validasi: %v (%s)", err, code)
		}
		seen[code] = struct{}{}
	}
	if len(seen) < 40 {
		t.Errorf("kode yang dihasilkan terlalu sering sama: %d unik dari 50", len(seen))
	}
}

func TestValidateInviteCodeFormat(t *testing.T) {
	cases := []struct {
		code  string
		valid bool
	}{
		{"BATCH247", true},
		{"KL7X2QAB", true},
		{"batch247", false},  // huruf kecil
		{"BATCH24", false},   // kurang satu karakter
		{"BATCH2478", false}, // lebih satu karakter
		{"BATCH0O7", false},  // memuat karakter ambigu 0/O
		{"BATCH1I7", false},  // memuat karakter ambigu 1/I
		{"BATCH-27", false},  // memuat tanda hubung
		{"", false},          // kosong
	}

	for _, tc := range cases {
		err := validateInviteCodeFormat(tc.code)
		if tc.valid && err != nil {
			t.Errorf("kode %q harus valid, dapat %v", tc.code, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("kode %q harus ditolak", tc.code)
		}
	}
}

func TestJoinabilityErrorMemeriksaStatusPeriodeDanKuota(t *testing.T) {
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	cases := []struct {
		name     string
		cohort   models.Cohort
		active   int
		wantCode apperror.Code
	}{
		{"kelas berjalan tanpa kuota", models.Cohort{Status: models.CohortAktif}, 100, ""},
		{"kelas berjalan masih ada jatah", models.Cohort{Status: models.CohortAktif, Quota: 5}, 4, ""},
		{"kuota kelas penuh", models.Cohort{Status: models.CohortAktif, Quota: 5}, 5, apperror.CodeConflict},
		{"kelas draf", models.Cohort{Status: models.CohortDraft}, 0, apperror.CodeValidation},
		{"kelas selesai", models.Cohort{Status: models.CohortSelesai}, 0, apperror.CodeValidation},
		{"kelas arsip", models.Cohort{Status: models.CohortArsip}, 0, apperror.CodeValidation},
		{"kelas belum dimulai", models.Cohort{Status: models.CohortAktif, StartsAt: &future}, 0, apperror.CodeValidation},
		{"kelas sudah berakhir", models.Cohort{Status: models.CohortAktif, EndsAt: &past}, 0, apperror.CodeValidation},
	}

	for _, tc := range cases {
		err := joinabilityError(tc.cohort, tc.active, now)
		if tc.wantCode == "" {
			if err != nil {
				t.Errorf("%s: harus boleh diikuti, dapat %v", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: harus ditolak", tc.name)
			continue
		}
		if got := apperror.From(err).Code; got != tc.wantCode {
			t.Errorf("%s: kode galat = %v, harap %v", tc.name, got, tc.wantCode)
		}
	}
}

func TestSlugifyDanKodeKelas(t *testing.T) {
	slugCases := []struct {
		name string
		want string
	}{
		{"Instansi Demo Nusantara", "instansi-demo-nusantara"},
		{"  Bank   Nusantara  ", "bank-nusantara"},
		{"PT. Telkom (Persero)", "pt-telkom-persero"},
		{"---", ""},
	}
	for _, tc := range slugCases {
		if got := slugify(tc.name); got != tc.want {
			t.Errorf("slugify(%q) = %q, harap %q", tc.name, got, tc.want)
		}
	}

	codeCases := []struct {
		name string
		want string
	}{
		{"Kelas Khusus TKD Batch 1", "KELAS-KHUSUS-TKD-BATCH-1"},
		{"Batch 2/2026", "BATCH-2-2026"},
		{"", "KELAS"},
	}
	for _, tc := range codeCases {
		if got := cohortCodeFromName(tc.name); got != tc.want {
			t.Errorf("cohortCodeFromName(%q) = %q, harap %q", tc.name, got, tc.want)
		}
	}
	if got := cohortCodeFromName(strings.Repeat("A", 60)); len(got) != 40 {
		t.Errorf("kode kelas harus dipotong 40 karakter, dapat %d (%q)", len(got), got)
	}
}

func TestParseMemberRowsDenganDanTanpaHeader(t *testing.T) {
	withHeader := "email,nama\npeserta@bumnacademy.id,Peserta Demo\nbukan-email,Nama Salah\n, Tanpa Email\n"
	emails, failures, err := parseMemberRows("peserta.csv", []byte(withHeader))
	if err != nil {
		t.Fatalf("parseMemberRows error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "peserta@bumnacademy.id" {
		t.Errorf("email terbaca = %v, harap hanya peserta@bumnacademy.id", emails)
	}
	if len(failures) != 2 {
		t.Fatalf("jumlah baris gagal = %d, harap 2 (%v)", len(failures), failures)
	}
	// Header adalah baris 1, sehingga baris gagal ada di 3 dan 4.
	if failures[0].Row != 3 || failures[1].Row != 4 {
		t.Errorf("nomor baris gagal = %d,%d harap 3,4", failures[0].Row, failures[1].Row)
	}

	withoutHeader := "satu@bumnacademy.id\ndua@bumnacademy.id\n\n"
	emails, failures, err = parseMemberRows("peserta.csv", []byte(withoutHeader))
	if err != nil {
		t.Fatalf("parseMemberRows error: %v", err)
	}
	if len(emails) != 2 {
		t.Errorf("email tanpa header = %v, harap 2", emails)
	}
	if len(failures) != 0 {
		t.Errorf("tanpa header seharusnya tidak ada kegagalan, dapat %v", failures)
	}

	// Kolom email tetap dikenali walau posisinya bukan yang pertama.
	reordered := "nama,email\nPeserta Demo,peserta@bumnacademy.id\n"
	emails, _, err = parseMemberRows("peserta.csv", []byte(reordered))
	if err != nil {
		t.Fatalf("parseMemberRows error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "peserta@bumnacademy.id" {
		t.Errorf("email dari kolom kedua = %v, harap peserta@bumnacademy.id", emails)
	}
}

func TestNormalizeEmailsDanValidasiEmail(t *testing.T) {
	got := normalizeEmails([]string{" Peserta@BumnAcademy.id ", "peserta@bumnacademy.id", "", "dua@bumnacademy.id"})
	if len(got) != 2 {
		t.Fatalf("email unik = %v, harap 2", got)
	}
	if got[0] != "peserta@bumnacademy.id" || got[1] != "dua@bumnacademy.id" {
		t.Errorf("urutan email harus dipertahankan: %v", got)
	}

	for _, valid := range []string{"a@b.id", "nama.peserta@bumnacademy.co.id"} {
		if !validEmail(valid) {
			t.Errorf("email %q harus dianggap sah", valid)
		}
	}
	for _, invalid := range []string{"", "bukan-email", "a@b", "@b.id", "a b@c.id"} {
		if validEmail(invalid) {
			t.Errorf("email %q harus dianggap tidak sah", invalid)
		}
	}
}

func TestInviteRejectErrorMemetakanKodeGalat(t *testing.T) {
	if err := inviteRejectError(""); err != nil {
		t.Errorf("alasan kosong berarti kode sah, dapat %v", err)
	}

	kuota := inviteRejectError("Kuota kode undangan sudah habis.")
	if apperror.From(kuota).Code != apperror.CodeConflict {
		t.Errorf("kuota kode habis harus CONFLICT, dapat %v", apperror.From(kuota).Code)
	}

	for _, reason := range []string{"Kode undangan sudah dicabut.", "Kode undangan sudah kedaluwarsa."} {
		err := inviteRejectError(reason)
		if apperror.From(err).Code != apperror.CodeValidation {
			t.Errorf("%q harus VALIDATION_ERROR, dapat %v", reason, apperror.From(err).Code)
		}
	}
}

func TestPemetaanInputStatusDanPeran(t *testing.T) {
	if role, err := memberRoleFromInput(""); err != nil || role != models.CohortRolePeserta {
		t.Errorf("peran bawaan harus peserta, dapat %v (%v)", role, err)
	}
	if role, err := memberRoleFromInput("MENTOR"); err != nil || role != models.CohortRoleMentor {
		t.Errorf("peran mentor tidak terbaca: %v (%v)", role, err)
	}
	if _, err := memberRoleFromInput("pengawas"); err == nil {
		t.Error("peran tak dikenal harus ditolak")
	}

	if status, err := memberStatusFromInput("Lulus"); err != nil || status != models.CohortMemberLulus {
		t.Errorf("status lulus tidak terbaca: %v (%v)", status, err)
	}
	if _, err := memberStatusFromInput("ditangguhkan"); err == nil {
		t.Error("status tak dikenal harus ditolak")
	}

	if status, err := cohortStatusFromInput("Aktif"); err != nil || status != models.CohortAktif {
		t.Errorf("status kelas aktif tidak terbaca: %v (%v)", status, err)
	}
	if _, err := cohortStatusFromInput("berjalan"); err == nil {
		t.Error("status kelas tak dikenal harus ditolak")
	}
}

func TestValidateCohortPeriod(t *testing.T) {
	service := &CohortService{}
	start := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	if err := service.validateCohortPeriod(&models.Cohort{StartsAt: &start, EndsAt: &end}, nil); err != nil {
		t.Errorf("periode wajar harus lolos, dapat %v", err)
	}
	if err := service.validateCohortPeriod(&models.Cohort{StartsAt: &end, EndsAt: &start}, nil); err == nil {
		t.Error("tanggal berakhir sebelum mulai harus ditolak")
	}
	if err := service.validateCohortPeriod(&models.Cohort{StartsAt: &end, AccessUntil: &start}, nil); err == nil {
		t.Error("akhir masa akses sebelum mulai harus ditolak")
	}

	// PATCH sebagian: `ends_at` baru dibandingkan dengan `starts_at` yang tersimpan.
	if err := service.validateCohortPeriod(
		&models.Cohort{StartsAt: &start},
		map[string]any{"ends_at": &start},
	); err != nil {
		t.Errorf("tanggal berakhir sama dengan mulai masih boleh, dapat %v", err)
	}
	if err := service.validateCohortPeriod(
		&models.Cohort{StartsAt: &start},
		map[string]any{"ends_at": start.AddDate(0, 0, -1)},
	); err == nil {
		t.Error("PATCH dengan tanggal berakhir sebelum mulai harus ditolak")
	}
}

func TestCurriculumItemsMenolakInputTidakLengkap(t *testing.T) {
	// Kurikulum divalidasi sebelum menyentuh database: id modul kosong, modul ganda,
	// dan jumlah melebihi batas harus ditolak lebih dulu.
	service := &CohortService{}
	ctx := context.Background()

	if _, err := service.curriculumItems(ctx, "cohort-1", []dto.CohortCurriculumInput{{ModuleID: ""}}); err == nil {
		t.Error("modul tanpa id harus ditolak")
	}
	if _, err := service.curriculumItems(ctx, "cohort-1", []dto.CohortCurriculumInput{
		{ModuleID: "mod-a"}, {ModuleID: "mod-a"},
	}); err == nil {
		t.Error("modul ganda harus ditolak")
	}

	tooMany := make([]dto.CohortCurriculumInput, cohortMaxModules+1)
	for index := range tooMany {
		tooMany[index] = dto.CohortCurriculumInput{ModuleID: "mod-" + string(rune('a'+index%26))}
	}
	if _, err := service.curriculumItems(ctx, "cohort-1", tooMany); err == nil {
		t.Error("kurikulum melebihi batas harus ditolak")
	}
}

// Uji laporan kelas: klasifikasi capaian anggota, urutan daftar prestasi &
// pendampingan, serta isi dokumen PDF. Semuanya murni (tanpa database), karena
// angka laporan berasal dari query yang sudah diuji terpisah.
func TestCohortMemberNoteMengklasifikasiCapaian(t *testing.T) {
	score := func(value int) *int { return &value }

	cases := []struct {
		name string
		row  repository.CohortReportMemberRow
		want string
	}{
		{"belum ada percobaan", repository.CohortReportMemberRow{}, "Belum mulai"},
		{"skor terakhir di bawah ambang", repository.CohortReportMemberRow{
			AttemptCount: 2, BestScore: 80, AverageScore: 70, LastScore: score(50),
		}, "Perlu pendampingan"},
		{"skor terakhir & rata-rata di atas ambang", repository.CohortReportMemberRow{
			AttemptCount: 3, BestScore: 90, AverageScore: 75, LastScore: score(80),
		}, "Prestasi"},
		{"skor terakhir lulus tapi rata-rata belum", repository.CohortReportMemberRow{
			AttemptCount: 3, BestScore: 85, AverageScore: 60, LastScore: score(85),
		}, "Lancar"},
		{"tanpa skor terakhir memakai skor terbaik", repository.CohortReportMemberRow{
			AttemptCount: 1, BestScore: 90, AverageScore: 90,
		}, "Prestasi"},
	}

	for _, tc := range cases {
		if got := cohortMemberNote(tc.row, 70); got != tc.want {
			t.Errorf("%s: cohortMemberNote = %q, harap %q", tc.name, got, tc.want)
		}
	}
}

func TestCohortReportTopDanAtRisk(t *testing.T) {
	score := func(value int) *int { return &value }
	members := []dto.CohortReportMemberView{
		{Name: "Belum Mulai", AttemptCount: 0, Note: "Belum mulai"},
		{Name: "Rendah", AttemptCount: 2, BestScore: 40, LastScore: score(30), Note: "Perlu pendampingan"},
		{Name: "Sedang", AttemptCount: 2, BestScore: 75, LastScore: score(75), Note: "Lancar"},
		{Name: "Tinggi", AttemptCount: 4, BestScore: 95, AverageScore: 88, LastScore: score(90), Note: "Prestasi"},
		{Name: "Kedua", AttemptCount: 3, BestScore: 95, AverageScore: 80, LastScore: score(80), Note: "Prestasi"},
	}

	top := cohortReportTopLearners(members, 2)
	if len(top) != 2 {
		t.Fatalf("jumlah peserta berprestasi = %d, harap 2", len(top))
	}
	if top[0].Name != "Tinggi" || top[1].Name != "Kedua" {
		t.Errorf("urutan prestasi harus skor terbaik lalu rata-rata: %v", []string{top[0].Name, top[1].Name})
	}

	atRisk := cohortReportAtRisk(members, 10)
	if len(atRisk) != 2 {
		t.Fatalf("jumlah peserta pendampingan = %d, harap 2", len(atRisk))
	}
	if atRisk[0].Name != "Belum Mulai" {
		t.Errorf("peserta tanpa percobaan harus didahulukan, dapat %q", atRisk[0].Name)
	}
	if atRisk[1].Name != "Rendah" {
		t.Errorf("peserta skor terendah berikutnya harus 'Rendah', dapat %q", atRisk[1].Name)
	}

	if got := cohortReportTopLearners(members, 0); len(got) != 4 {
		t.Errorf("batas 0 berarti tanpa batas (semua anggota yang berlatih), dapat %d", len(got))
	}
	if got := cohortReportAtRisk([]dto.CohortReportMemberView{members[2]}, 5); len(got) != 0 {
		t.Errorf("peserta lancar tidak boleh masuk daftar pendampingan, dapat %v", got)
	}
}

func TestCohortReportDocumentMemuatAngkaDanCatatan(t *testing.T) {
	report := &dto.CohortReportView{
		Cohort: dto.CohortSummaryView{
			ID: "c-1", Name: "Kelas Uji", Code: "KELAS-1", OrgName: "Instansi Uji",
			MentorName: "Admin Akademik", StatusLabel: "Aktif",
		},
		Period: dto.CohortReportPeriod{StartsAt: "2026-11-01T00:00:00Z", EndsAt: "2026-12-01T00:00:00Z"},
		Summary: dto.CohortReportSummary{
			TotalMembers: 3, ActiveMembers: 2, CompletedMembers: 1,
			ReportingMembers: 2, ParticipationRate: 66.7, ActiveLast30Days: 2, ActiveWindowDays: 30,
			TotalAttempts: 5, InClassAttempts: 3, AverageScore: 71.5, TopScore: 95,
			PassingThreshold: 70, PassingRate: 60, PracticeHours: 1.5,
			TotalLessons: 5, CompletedLessons: 3, CompletionRate: 60, Certificates: 1,
		},
		Modules: []dto.CohortReportModuleView{
			{ModuleTitle: "Modul Wajib", Required: true, LessonCount: 3, CompletedLessons: 2, CompletionRate: 66.7},
			{ModuleTitle: "Modul Opsional", Required: false, LessonCount: 2, CompletedLessons: 1, CompletionRate: 50},
		},
		Skills: []dto.CohortReportSkillView{{Name: "Penalaran Numerik", AverageScore: 71.5, Attempts: 5}},
		Members: []dto.CohortReportMemberView{
			{Name: "Peserta Satu", Status: "aktif", AttemptCount: 3, AverageScore: 80, BestScore: 95, LessonsCompleted: 3, LessonsTotal: 5, Certificates: 1, Note: "Prestasi"},
		},
		TopLearners: []dto.CohortReportMemberView{{Name: "Peserta Satu", Program: "Reguler", BestScore: 95, AverageScore: 80, AttemptCount: 3}},
		AtRisk:      []dto.CohortReportMemberView{{Name: "Peserta Dua", Program: "Reguler", ProgressPercent: 20}},
		GeneratedAt: "2026-11-20T10:00:00Z",
		Notes:       []string{"Bukan prediksi kelulusan."},
	}

	document := cohortReportDocument(report)
	if document.Title != "Laporan Kelas Khusus" {
		t.Errorf("judul dokumen = %q", document.Title)
	}
	if !strings.Contains(document.Subtitle, "Instansi Uji") {
		t.Errorf("subjudul harus memuat instansi: %q", document.Subtitle)
	}

	text := cohortReportText(document)
	for _, want := range []string{
		"Kelas: Kelas Uji (KELAS-1)",
		"Instansi: Instansi Uji",
		"Mentor: Admin Akademik",
		"Anggota: 3 (aktif 2, lulus 1, keluar 0)",
		"Partisipasi latihan: 2 dari 3 anggota (66.7%)",
		"tercatat milik kelas ini: 3",
		"Modul Wajib (wajib): 2/3 materi selesai",
		"Modul Opsional (opsional): 1/2 materi selesai",
		"Penalaran Numerik: rata-rata 71.5 dari 5 percobaan",
		"Peserta Satu (Reguler): skor terbaik 95",
		"Peserta Dua (Reguler): belum ada percobaan, progres materi 20%",
		"Peserta Satu · aktif · 3 percobaan",
		"Bukan prediksi kelulusan.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("dokumen laporan tidak memuat %q:\n%s", want, text)
		}
	}

	// Kelas tanpa kurikulum/percobaan tetap menghasilkan laporan yang terbaca.
	emptyText := cohortReportText(cohortReportDocument(&dto.CohortReportView{}))
	for _, want := range []string{
		"Kurikulum kelas belum diisi.",
		"Belum ada peserta berprestasi",
		"Tidak ada peserta yang perlu pendampingan.",
	} {
		if !strings.Contains(emptyText, want) {
			t.Errorf("laporan kelas kosong tidak memuat %q", want)
		}
	}
}

// cohortReportText menggabungkan seluruh teks blok dokumen (pembantu uji).
func cohortReportText(document ebook.Document) string {
	var builder strings.Builder
	for _, block := range document.Blocks {
		builder.WriteString(block.Text)
		builder.WriteString("\n")
	}
	return builder.String()
}

func TestCohortFileNameDanPercent(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"SMOKE-1793442522", "SMOKE-1793442522"},
		{"Kelas/Uji 2026", "Kelas-Uji-2026"},
		{"", "kelas"},
	}
	for _, tc := range cases {
		if got := cohortFileName(tc.code); got != tc.want {
			t.Errorf("cohortFileName(%q) = %q, harap %q", tc.code, got, tc.want)
		}
	}

	if got := cohortPercentFloat(3, 4); got != 75 {
		t.Errorf("cohortPercentFloat(3,4) = %v, harap 75", got)
	}
	if got := cohortPercentFloat(1, 3); got != 33.3 {
		t.Errorf("cohortPercentFloat(1,3) = %v, harap 33.3", got)
	}
	if got := cohortPercentFloat(0, 0); got != 0 {
		t.Errorf("pembagi nol harus menghasilkan 0, dapat %v", got)
	}
	if got := cohortPercentFloat(5, 2); got != 100 {
		t.Errorf("persentase di atas 100 harus dibatasi, dapat %v", got)
	}

	if got := periodText(dto.CohortReportPeriod{}); got != "tanpa periode tetap" {
		t.Errorf("periode kosong = %q", got)
	}
	if got := periodText(dto.CohortReportPeriod{StartsAt: "2026-11-01", EndsAt: "2026-12-01"}); got != "2026-11-01 s.d. 2026-12-01" {
		t.Errorf("periode = %q", got)
	}
}

func TestCohortActionNoteDanUrutanCurriculumView(t *testing.T) {
	if got := cohortActionNote(models.CohortActionInviteUsed); got != "Kode undangan dipakai" {
		t.Errorf("catatan aksi invite_used = %q", got)
	}
	if got := cohortActionNote("aksi-lain"); got != "aksi-lain" {
		t.Errorf("aksi tak dikenal harus dikembalikan apa adanya, dapat %q", got)
	}

	service := &CohortService{}
	items := []models.CohortCurriculumItem{
		{CohortID: "c-1", ModuleID: "mod-a", Required: true, SortOrder: 0},
		{CohortID: "c-1", ModuleID: "mod-b", Required: false, SortOrder: 1},
	}
	views := service.curriculumViews(items)
	if len(views) != 2 {
		t.Fatalf("jumlah modul tampilan = %d, harap 2", len(views))
	}
	if !views[0].Required || views[1].Required {
		t.Error("penanda modul wajib/opsional harus terbawa ke tampilan")
	}
}
