package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Seed KPI kelulusan (Fase 2 langkah 1): referensi tahapan seleksi BUMN, laporan
// hasil seleksi demo, dan target KPI contoh.
//
// Tujuannya agar dasbor `/admin/kpi` langsung bisa dibaca pada instalasi demo:
// ada peserta yang lolos tahap akhir, ada yang berhenti di tengah, dan ada laporan
// yang **belum** diverifikasi — sehingga pemisahan `verified` vs `reported`
// terlihat nyata, bukan sekadar teori.

// selectionStageDefs adalah tahapan seleksi BUMN (mengikuti Rekrutmen Bersama
// BUMN). Nilainya sama dengan seed di migrasi `000017`; seed di sini menjaga
// instalasi yang migrasinya sudah lebih dulu berjalan tetap lengkap.
var selectionStageDefs = []models.SelectionStage{
	{Key: "administrasi", Label: "Seleksi Administrasi", Phase: 1, SortOrder: 10, Active: true},
	{Key: "tahap1_tkd", Label: "Tes Kemampuan Dasar (TKD)", Phase: 2, SortOrder: 20, Active: true},
	{Key: "tahap1_akhlaq", Label: "Tes AKHLAK", Phase: 2, SortOrder: 30, Active: true},
	{Key: "tahap1_twk", Label: "Tes Wawasan Kebangsaan (TWK)", Phase: 2, SortOrder: 40, Active: true},
	{Key: "tahap2_bing", Label: "Tes Bahasa Inggris", Phase: 3, SortOrder: 50, Active: true},
	{Key: "tahap2_learning_agility", Label: "Tes Learning Agility", Phase: 3, SortOrder: 60, Active: true},
	{Key: "tkb", Label: "Tes Kompetensi Bidang (TKB)", Phase: 4, SortOrder: 70, Active: true},
	{Key: "wawancara_user", Label: "Wawancara User", Phase: 5, SortOrder: 80, Active: true},
	{Key: "wawancara_hc", Label: "Wawancara HC", Phase: 6, SortOrder: 90, Active: true},
	{Key: "mcu", Label: "Medical Check-Up (MCU)", Phase: 7, SortOrder: 100, Active: true},
	{Key: "offering", Label: "Offering / Penetapan", Phase: 8, IsFinal: true, SortOrder: 110, Active: true},
}

// companyProfiles memetakan instansi demo → posisi & keluarga jabatan, supaya
// definisi laporan demo tidak mengulang kolom yang sama di setiap baris.
var companyProfiles = map[string]struct{ Position, Family string }{
	"Bank Nusantara":           {Position: "Analis Kredit", Family: "keuangan"},
	"Energi Prima":             {Position: "Analis Energi", Family: "operasional"},
	"Telekomunikasi Nusantara": {Position: "Analis Data", Family: "teknologi"},
	"Logistik Andalan":         {Position: "Staf Operasional", Family: "operasional"},
}

// outcomeDemoDef adalah satu laporan hasil seleksi demo.
//
// `AnnouncedDay` memakai format `MM-DD` dan digabung dengan **tahun berjalan** saat
// seed dijalankan, sehingga tanggal pengumuman selalu jatuh setelah riwayat latihan
// demo (median waktu tempuh `timeToPass` jadi bermakna, bukan negatif).
type outcomeDemoDef struct {
	Email        string
	StageKey     string
	Company      string
	Sector       string
	Status       models.OutcomeStatus
	Source       models.OutcomeSource
	AnnouncedDay string
	Note         string
	// InDemoCohort menandai laporan milik anggota kelas khusus demo, sehingga
	// KPI per kelas (`cohortPassRate`) terisi.
	InDemoCohort bool
}

// outcomeDemoDefs adalah data demo KPI untuk tahun berjalan.
//
// Sengaja memuat beberapa profil sekaligus: peserta yang lolos sampai offering,
// peserta yang berhenti di tengah (tahap 1/2), peserta yang gagal di administrasi,
// dan peserta dengan laporan yang masih `self_reported` supaya lencana "belum
// terverifikasi" bisa diuji.
var outcomeDemoDefs = []outcomeDemoDef{
	// Lolos sampai offering (terverifikasi; anggota kelas khusus demo).
	{Email: "peserta@bumnacademy.id", StageKey: "administrasi", Company: "Bank Nusantara",
		Sector: "jasa_keuangan", Status: models.OutcomeLolos, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "08-05", InDemoCohort: true},
	{Email: "peserta@bumnacademy.id", StageKey: "tahap1_tkd", Company: "Bank Nusantara",
		Sector: "jasa_keuangan", Status: models.OutcomeLolos, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "08-20", InDemoCohort: true},
	{Email: "peserta@bumnacademy.id", StageKey: "tahap2_bing", Company: "Bank Nusantara",
		Sector: "jasa_keuangan", Status: models.OutcomeLolos, Source: models.OutcomeDocumentVerified,
		AnnouncedDay: "09-10", InDemoCohort: true},
	{Email: "peserta@bumnacademy.id", StageKey: "offering", Company: "Bank Nusantara",
		Sector: "jasa_keuangan", Status: models.OutcomeLolos, Source: models.OutcomeDocumentVerified,
		AnnouncedDay: "10-10", Note: "Bukti pengumuman offering (PDF) sudah diverifikasi.",
		InDemoCohort: true},

	// Berhenti di tahap 1 (terverifikasi; anggota kelas khusus demo).
	{Email: "dewi.lestari@bumnacademy.id", StageKey: "administrasi", Company: "Bank Nusantara",
		Sector: "jasa_keuangan", Status: models.OutcomeLolos, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "08-05", InDemoCohort: true},
	{Email: "dewi.lestari@bumnacademy.id", StageKey: "tahap1_tkd", Company: "Bank Nusantara",
		Sector: "jasa_keuangan", Status: models.OutcomeGagal, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "08-20", InDemoCohort: true},

	// Berhenti di tahap 2 (terverifikasi).
	{Email: "siti.nurhaliza@bumnacademy.id", StageKey: "administrasi", Company: "Energi Prima",
		Sector: "energi", Status: models.OutcomeLolos, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "08-05"},
	{Email: "siti.nurhaliza@bumnacademy.id", StageKey: "tahap1_tkd", Company: "Energi Prima",
		Sector: "energi", Status: models.OutcomeLolos, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "08-20"},
	{Email: "siti.nurhaliza@bumnacademy.id", StageKey: "tahap2_bing", Company: "Energi Prima",
		Sector: "energi", Status: models.OutcomeGagal, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "09-10"},

	// Gagal di seleksi administrasi (terverifikasi).
	{Email: "bayu.saputra@bumnacademy.id", StageKey: "administrasi", Company: "Telekomunikasi Nusantara",
		Sector: "telekomunikasi", Status: models.OutcomeGagal, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "08-06"},

	// Lolos sampai offering (terverifikasi; klaster energi).
	{Email: "intan.permata@bumnacademy.id", StageKey: "administrasi", Company: "Energi Prima",
		Sector: "energi", Status: models.OutcomeLolos, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "08-05"},
	{Email: "intan.permata@bumnacademy.id", StageKey: "tahap1_tkd", Company: "Energi Prima",
		Sector: "energi", Status: models.OutcomeLolos, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "08-20"},
	{Email: "intan.permata@bumnacademy.id", StageKey: "tahap2_bing", Company: "Energi Prima",
		Sector: "energi", Status: models.OutcomeLolos, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "09-10"},
	{Email: "intan.permata@bumnacademy.id", StageKey: "offering", Company: "Energi Prima",
		Sector: "energi", Status: models.OutcomeLolos, Source: models.OutcomeAdminVerified,
		AnnouncedDay: "10-12"},

	// Masih menunggu pengumuman (belum terverifikasi) — calon penerima pengingat.
	{Email: "fajar.ramadhan@bumnacademy.id", StageKey: "administrasi", Company: "Logistik Andalan",
		Sector: "logistik", Status: models.OutcomeLolos, Source: models.OutcomeSelfReported,
		AnnouncedDay: "08-07"},
	{Email: "fajar.ramadhan@bumnacademy.id", StageKey: "tahap1_tkd", Company: "Logistik Andalan",
		Sector: "logistik", Status: models.OutcomeLolos, Source: models.OutcomeSelfReported,
		AnnouncedDay: "08-21"},
	{Email: "fajar.ramadhan@bumnacademy.id", StageKey: "offering", Company: "Logistik Andalan",
		Sector: "logistik", Status: models.OutcomeMenunggu, Source: models.OutcomeSelfReported,
		Note: "Menunggu pengumuman offering; belum dihitung sebagai kelulusan."},

	// Lolos offering dengan laporan yang belum diverifikasi (kualitas data mentah).
	{Email: "nadia.safira@bumnacademy.id", StageKey: "administrasi", Company: "Telekomunikasi Nusantara",
		Sector: "telekomunikasi", Status: models.OutcomeLolos, Source: models.OutcomeSelfReported,
		AnnouncedDay: "08-06"},
	{Email: "nadia.safira@bumnacademy.id", StageKey: "tahap1_tkd", Company: "Telekomunikasi Nusantara",
		Sector: "telekomunikasi", Status: models.OutcomeLolos, Source: models.OutcomeSelfReported,
		AnnouncedDay: "08-21"},
	{Email: "nadia.safira@bumnacademy.id", StageKey: "offering", Company: "Telekomunikasi Nusantara",
		Sector: "telekomunikasi", Status: models.OutcomeLolos, Source: models.OutcomeSelfReported,
		AnnouncedDay: "10-14", Note: "Belum unggah bukti pengumuman."},
}

// kpiTargetDemoDef adalah target KPI contoh agar panel "target vs realisasi"
// langsung terisi pada instalasi demo (nilai target ditetapkan tim akademik).
type kpiTargetDemoDef struct {
	Metric   models.KPIMetric
	StageKey string
	Target   float64
	Note     string
}

var kpiTargetDemoDefs = []kpiTargetDemoDef{
	{Metric: models.KPIMetricFinalPassRate, Target: 40,
		Note: "Target contoh tingkat kelulusan akhir (lintas program)."},
	{Metric: models.KPIMetricParticipants, Target: 60,
		Note: "Target contoh jumlah peserta yang melaporkan hasil seleksi."},
	{Metric: models.KPIMetricStagePassRate, StageKey: "administrasi", Target: 85,
		Note: "Target contoh tingkat kelulusan seleksi administrasi."},
}

// seedSelectionStages menyimpan referensi tahapan seleksi (idempoten).
//
// Kolom `active` sengaja **tidak** ikut diperbarui: admin boleh menonaktifkan
// tahap yang tidak dipakai tahun ini, dan menjalankan seed ulang tidak boleh
// mengaktifkannya kembali.
func seedSelectionStages(ctx context.Context, db *gorm.DB) error {
	stages := make([]models.SelectionStage, len(selectionStageDefs))
	copy(stages, selectionStageDefs)

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"label", "phase", "is_final", "sort_order"}),
	}).Create(&stages).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan tahapan seleksi: %w", err)
	}
	return nil
}

// seedSelectionOutcomes menyiapkan laporan hasil seleksi demo (idempoten).
//
// Semua laporan memakai tahun berjalan agar dasbor KPI langsung terisi tanpa
// mengubah filter apa pun. Laporan berstatus terverifikasi ikut mengisi
// `verified_by`/`verified_at` supaya lencana "Terverifikasi" punya dasar nyata.
func seedSelectionOutcomes(ctx context.Context, db *gorm.DB, userIDs map[string]string) error {
	now := time.Now().UTC()
	year := now.Year()
	demoCohortID := stableID("cohort/kohort-demo-1")

	verifierID, ok := userIDs["admin@bumnacademy.id"]
	if !ok {
		return fmt.Errorf("seed: akun admin@bumnacademy.id belum tersedia untuk verifikasi demo")
	}
	verifiedAt := now.AddDate(0, 0, -30)

	outcomes := make([]models.SelectionOutcome, 0, len(outcomeDemoDefs))
	for _, def := range outcomeDemoDefs {
		userID, ok := userIDs[def.Email]
		if !ok {
			return fmt.Errorf("seed: akun %s untuk laporan hasil seleksi belum tersedia", def.Email)
		}
		profile := companyProfiles[def.Company]

		// `AnnouncedDay` memakai format `MM-DD`; tahun berjalan digabungkan di sini
		// agar tanggal pengumuman selalu jatuh pada tahun laporan (tahun seed) —
		// median waktu tempuh `timeToPass` jadi bermakna, bukan negatif.
		var announcedAt *time.Time
		if def.AnnouncedDay != "" {
			parsed, err := time.Parse("01-02", def.AnnouncedDay)
			if err != nil {
				return fmt.Errorf("seed: tanggal pengumuman %q tidak valid: %w", def.AnnouncedDay, err)
			}
			announced := time.Date(year, parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
			announcedAt = &announced
		}

		outcome := models.SelectionOutcome{
			Base:             models.Base{ID: stableID("outcome/" + def.Email + "/" + def.StageKey + "/" + def.Company)},
			UserID:           userID,
			StageKey:         def.StageKey,
			CompanyName:      def.Company,
			CompanySector:    def.Sector,
			Position:         profile.Position,
			PositionFamily:   profile.Family,
			SelectionYear:    year,
			Status:           def.Status,
			AnnouncedAt:      announcedAt,
			Source:           def.Source,
			Note:             def.Note,
			AggregateConsent: true,
		}
		if def.InDemoCohort {
			cohortID := demoCohortID
			outcome.CohortID = &cohortID
		}
		if def.Source.Verified() {
			verified := verifierID
			at := verifiedAt
			outcome.VerifiedBy = &verified
			outcome.VerifiedAt = &at
		}
		outcomes = append(outcomes, outcome)
	}

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"}, {Name: "selection_year"}, {Name: "stage_key"}, {Name: "company_name"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"cohort_id", "company_sector", "position", "position_family",
			"status", "announced_at", "source", "note", "verified_by", "verified_at",
		}),
	}).Create(&outcomes).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan laporan hasil seleksi demo: %w", err)
	}
	return nil
}

// seedKPITargets menyiapkan target KPI contoh untuk tahun berjalan (idempoten).
//
// Target menempel pada id deterministik sehingga menjalankan seed berulang hanya
// memperbarui nilainya, bukan menambah baris (indeks uniknya berbasis ekspresi
// COALESCE, jadi konflik ditangani lewat primary key).
func seedKPITargets(ctx context.Context, db *gorm.DB) error {
	year := time.Now().UTC().Year()

	targets := make([]models.KPITarget, 0, len(kpiTargetDemoDefs))
	for _, def := range kpiTargetDemoDefs {
		target := models.KPITarget{
			Base:        models.Base{ID: stableID(fmt.Sprintf("kpi-target/%d/%s/%s", year, def.Metric, def.StageKey))},
			PeriodYear:  year,
			Metric:      def.Metric,
			TargetValue: def.Target,
			Note:        def.Note,
		}
		if def.StageKey != "" {
			stageKey := def.StageKey
			target.StageKey = &stageKey
		}
		targets = append(targets, target)
	}

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_value", "note", "stage_key"}),
	}).Create(&targets).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan target KPI demo: %w", err)
	}
	return nil
}
