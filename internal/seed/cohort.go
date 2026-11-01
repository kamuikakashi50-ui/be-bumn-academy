package seed

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// cohortCurriculumDef adalah satu modul ruang kelas yang diikutkan ke kelas demo.
type cohortCurriculumDef struct {
	ModuleID string
	// Required false menandai modul opsional (tidak menghalangi sertifikat).
	Required bool
}

// cohortDemoDef adalah satu kelas khusus demo beserta instansinya.
type cohortDemoDef struct {
	OrgSlug    string
	OrgName    string
	SectorKey  string
	PICName    string
	PICEmail   string
	CohortID   string
	CohortCode string
	CohortName string
	Notes      string
	Quota      int
	// MentorEmail diisi akun demo yang menjadi mentor kelas.
	MentorEmail string
	// MemberEmails adalah akun demo yang menjadi anggota kelas (peran peserta).
	MemberEmails []string
	Curriculum   []cohortCurriculumDef
	InviteCode   string
}

// cohortDemoDefs adalah kelas khusus demo yang memakai modul produk
// `kelas-intensif` sehingga bisa dicoba end-to-end bersama jalur belajar demo.
var cohortDemoDefs = []cohortDemoDef{
	{
		OrgSlug:     "instansi-demo-nusantara",
		OrgName:     "Instansi Demo Nusantara",
		SectorKey:   "jasa_keuangan",
		PICName:     "PIC Demo",
		PICEmail:    "pic@instansi-demo.id",
		CohortID:    "kohort-demo-1",
		CohortCode:  "DEMO-BATCH-1",
		CohortName:  "Kelas Khusus TKD Batch 1 — Instansi Demo Nusantara",
		Notes:       "Kelas contoh untuk mencoba alur kelas khusus: undangan, kurikulum, dan laporan.",
		Quota:       40,
		MentorEmail: "admin@bumnacademy.id",
		MemberEmails: []string{
			"peserta@bumnacademy.id",
			"dewi.lestari@bumnacademy.id",
			"rangga.pratama@bumnacademy.id",
		},
		Curriculum: []cohortCurriculumDef{
			{ModuleID: "mod-intensif-numerik", Required: true},
			{ModuleID: "mod-intensif-verbal", Required: true},
			{ModuleID: "mod-intensif-analitis", Required: false},
		},
		// 8 karakter huruf besar + angka, tanpa karakter ambigu (0/O, 1/I).
		InviteCode: "BATCH247",
	},
}

// seedCohorts menyiapkan instansi, kelas khusus, kurikulum, anggota, dan kode
// undangan demo (idempoten).
//
// Periode kelas dihitung relatif terhadap waktu seed dijalankan supaya kelas demo
// selalu berstatus berjalan dan bisa dicoba kapan pun; kurikulum & anggota diganti
// seluruhnya agar susunan yang tersimpan sama dengan yang ditulis di seed.
// `used_count` kode undangan sengaja tidak direset: pemakaian nyata saat demo
// tidak boleh hilang karena seed dijalankan ulang.
func seedCohorts(ctx context.Context, db *gorm.DB, classroom *ClassroomDataset, userIDs map[string]string) error {
	now := time.Now().UTC()
	moduleIDs := map[string]struct{}{}
	for _, module := range classroom.Modules {
		moduleIDs[module.ID] = struct{}{}
	}

	for _, def := range cohortDemoDefs {
		orgID := stableID("organization/" + def.OrgSlug)
		cohortID := stableID("cohort/" + def.CohortID)

		org := models.Organization{
			ID:        orgID,
			Name:      def.OrgName,
			Slug:      def.OrgSlug,
			SectorKey: def.SectorKey,
			PICName:   def.PICName,
			PICEmail:  def.PICEmail,
			Active:    true,
		}
		if err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "slug", "sector_key", "pic_name", "pic_email", "active",
			}),
		}).Create(&org).Error; err != nil {
			return fmt.Errorf("seed: gagal menyimpan instansi demo %s: %w", def.OrgSlug, err)
		}

		mentorID, ok := userIDs[def.MentorEmail]
		if !ok {
			return fmt.Errorf("seed: akun mentor kelas %s belum tersedia", def.MentorEmail)
		}

		starts := now.AddDate(0, 0, -7)
		ends := now.AddDate(0, 0, 60)
		accessUntil := now.AddDate(0, 0, 90)

		cohort := models.Cohort{
			ID:           cohortID,
			OrgID:        &orgID,
			Name:         def.CohortName,
			Code:         def.CohortCode,
			SectorKey:    def.SectorKey,
			Status:       models.CohortAktif,
			StartsAt:     &starts,
			EndsAt:       &ends,
			Quota:        def.Quota,
			AccessUntil:  &accessUntil,
			MentorUserID: &mentorID,
			Notes:        def.Notes,
		}
		if err := db.WithContext(ctx).
			Omit("Organization", "Curriculum").
			Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"org_id", "name", "code", "sector_key", "status",
					"starts_at", "ends_at", "quota", "access_until",
					"mentor_user_id", "notes",
				}),
			}).Create(&cohort).Error; err != nil {
			return fmt.Errorf("seed: gagal menyimpan kelas khusus %s: %w", def.CohortID, err)
		}

		if err := seedCohortCurriculum(ctx, db, cohortID, def, moduleIDs); err != nil {
			return err
		}
		if err := seedCohortMembers(ctx, db, cohortID, def, userIDs, now); err != nil {
			return err
		}
		if def.InviteCode != "" {
			invite := models.CohortInvite{
				Code:     def.InviteCode,
				CohortID: cohortID,
				Active:   true,
			}
			if err := db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "code"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"cohort_id", "max_uses", "expires_at", "active",
				}),
			}).Create(&invite).Error; err != nil {
				return fmt.Errorf("seed: gagal menyimpan kode undangan %s: %w", def.InviteCode, err)
			}
		}
	}
	return nil
}

// seedCohortCurriculum mengganti kurikulum kelas demo (delete-then-insert).
//
// Modul yang tidak ada di dataset ruang kelas dilewati dengan peringatan agar
// perubahan dataset tidak membuat seed gagal karena foreign key — masalahnya tetap
// terlihat di log.
func seedCohortCurriculum(
	ctx context.Context,
	db *gorm.DB,
	cohortID string,
	def cohortDemoDef,
	moduleIDs map[string]struct{},
) error {
	items := make([]models.CohortCurriculumItem, 0, len(def.Curriculum))
	for index, item := range def.Curriculum {
		if _, ok := moduleIDs[item.ModuleID]; !ok {
			slog.Warn("seed: modul kurikulum kelas dilewati karena tidak ada di dataset",
				"kelas", def.CohortID, "modul", item.ModuleID)
			continue
		}
		items = append(items, models.CohortCurriculumItem{
			CohortID:  cohortID,
			ModuleID:  item.ModuleID,
			Required:  item.Required,
			SortOrder: index,
		})
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("cohort_id = ?", cohortID).Delete(&models.CohortCurriculumItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan kurikulum kelas %s: %w", def.CohortID, err)
	}
	return nil
}

// seedCohortMembers menyisipkan anggota & mentor kelas demo.
//
// Idempoten lewat konflik (cohort_id, user_id): menjalankan seed berulang tidak
// menggandakan keanggotaan.
func seedCohortMembers(
	ctx context.Context,
	db *gorm.DB,
	cohortID string,
	def cohortDemoDef,
	userIDs map[string]string,
	now time.Time,
) error {
	members := make([]models.CohortMember, 0, len(def.MemberEmails)+1)
	for _, email := range def.MemberEmails {
		userID, ok := userIDs[email]
		if !ok {
			return fmt.Errorf("seed: akun anggota kelas %s belum tersedia", email)
		}
		members = append(members, models.CohortMember{
			CohortID: cohortID,
			UserID:   userID,
			Role:     models.CohortRolePeserta,
			Status:   models.CohortMemberAktif,
			JoinedAt: now,
		})
	}

	if mentorID, ok := userIDs[def.MentorEmail]; ok {
		members = append(members, models.CohortMember{
			CohortID: cohortID,
			UserID:   mentorID,
			Role:     models.CohortRoleMentor,
			Status:   models.CohortMemberAktif,
			JoinedAt: now,
		})
	}

	if len(members) == 0 {
		return nil
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cohort_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"role", "status"}),
	}).Create(&members).Error; err != nil {
		return fmt.Errorf("seed: gagal menyimpan anggota kelas %s: %w", def.CohortID, err)
	}
	return nil
}
