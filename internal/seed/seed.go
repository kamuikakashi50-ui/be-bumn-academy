package seed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/password"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Run mengisi seluruh data demo (idempoten).
//
// Dipanggil dari cmd/api saat SEED_ON_START=true dan dari cmd/seed. Semua
// penulisan memakai upsert berbasis kunci unik sehingga aman dijalankan
// berulang, termasuk dari beberapa replika API yang start bersamaan.
func Run(ctx context.Context, db *gorm.DB, cfg *config.Config) error {
	data, err := loadDataset()
	if err != nil {
		return err
	}
	classroom, err := loadClassroom()
	if err != nil {
		return err
	}

	userIDs, err := seedUsers(ctx, db, data.Users)
	if err != nil {
		return err
	}

	// Kohort demo analitik HR: peserta tambahan lintas segmen program. Dibuat
	// lewat jalur yang sama dengan akun inti agar kata sandinya ikut di-hash.
	cohortIDs, err := seedUsers(ctx, db, hrCohortUsers)
	if err != nil {
		return err
	}
	for email, id := range cohortIDs {
		userIDs[email] = id
	}

	if err := seedPrograms(ctx, db, data.Programs); err != nil {
		return err
	}
	if err := seedMateriCategories(ctx, db, data.MateriCategories); err != nil {
		return err
	}
	if err := seedProducts(ctx, db, data.Products); err != nil {
		return err
	}
	if err := seedPaymentMethods(ctx, db, data.PaymentMethods); err != nil {
		return err
	}
	if err := seedVouchers(ctx, db, data.Vouchers); err != nil {
		return err
	}
	// Paket tryout dibuat lebih dulu: soal menautkan dirinya ke paket lewat
	// `tryout_id` (foreign key), jadi paketnya harus sudah ada.
	if err := seedTryouts(ctx, db, data.Tryouts); err != nil {
		return err
	}
	if err := seedQuestions(ctx, db, data.Questions, data.Tryouts); err != nil {
		return err
	}
	if err := seedClassroom(ctx, db, classroom); err != nil {
		return err
	}
	if err := seedLiveSessions(ctx, db); err != nil {
		return err
	}
	if err := seedRecordings(ctx, db); err != nil {
		return err
	}
	if err := seedEnrollments(ctx, db, data.Enrollments, userIDs); err != nil {
		return err
	}
	// Riwayat latihan & progres materi kohort agar dasbor Analitik HR
	// (/admin/hr) benar-benar terisi data.
	if err := seedHRActivity(ctx, db, data, classroom, userIDs, cfg.Business.PassingThreshold); err != nil {
		return err
	}

	slog.Info("seed: data demo siap",
		"env", cfg.App.Env,
		"programs", len(data.Programs),
		"produk", len(data.Products),
		"voucher", len(data.Vouchers),
		"soal", len(data.Questions),
		"tryout", len(data.Tryouts),
		"modul", len(classroom.Modules),
		"kohort_hr", len(hrCohortUsers),
		"percobaan_hr", len(hrAttemptDefs),
	)
	return nil
}

// stableID menghasilkan UUID deterministik dari sebuah kunci sehingga menjalankan
// seed berulang tidak membuat baris baru (dan id pada JSON tetap bisa dirujuk).
func stableID(key string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("be-bumn-academy/seed/"+key)).String()
}

/* --------------------------------- akun demo -------------------------------- */

// seedUsers menyisipkan/memperbarui akun demo dan mengembalikan peta email → id.
//
// Kata sandi di-hash ulang setiap kali seed dijalankan agar kredensial yang
// didokumentasikan di README (peserta@bumnacademy.id / admin@bumnacademy.id)
// selalu bisa dipakai pada instalasi demo.
func seedUsers(ctx context.Context, db *gorm.DB, defs []UserDef) (map[string]string, error) {
	out := make(map[string]string, len(defs))
	for _, def := range defs {
		hashed, err := password.Hash(def.Password)
		if err != nil {
			return nil, err
		}

		var existing models.User
		err = db.WithContext(ctx).Where("lower(email) = lower(?)", def.Email).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			user := models.User{
				Base:         models.Base{ID: stableID("user/" + def.Email)},
				Name:         def.Name,
				Email:        def.Email,
				PasswordHash: hashed,
				Role:         models.Role(def.Role),
				Program:      def.Program,
				ReferralCode: stringPtr(def.ReferralCode),
			}
			if err := db.WithContext(ctx).Create(&user).Error; err != nil {
				return nil, fmt.Errorf("seed: gagal membuat akun %s: %w", def.Email, err)
			}
			out[def.Email] = user.ID
		case err != nil:
			return nil, fmt.Errorf("seed: gagal mencari akun %s: %w", def.Email, err)
		default:
			updates := map[string]any{
				"name":          def.Name,
				"password_hash": hashed,
				"role":          models.Role(def.Role),
				"program":       def.Program,
				"referral_code": stringPtr(def.ReferralCode),
			}
			if err := db.WithContext(ctx).Model(&models.User{}).
				Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
				return nil, fmt.Errorf("seed: gagal memperbarui akun %s: %w", def.Email, err)
			}
			out[def.Email] = existing.ID
		}
	}
	return out, nil
}

// seedEnrollments memberi akses produk ke akun demo tanpa menimpa kepemilikan
// yang sudah ada (mis. hasil pembelian asli).
func seedEnrollments(ctx context.Context, db *gorm.DB, defs []EnrollmentDef, userIDs map[string]string) error {
	for _, def := range defs {
		userID, ok := userIDs[def.Email]
		if !ok {
			return fmt.Errorf("seed: enrollment menunjuk email %s yang tidak dikenal", def.Email)
		}
		enrollment := models.Enrollment{
			ID:          stableID("enrollment/" + def.Email + "/" + def.ProductSlug),
			UserID:      userID,
			ProductSlug: def.ProductSlug,
			Source:      def.Source,
		}
		if err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "product_slug"}},
			DoNothing: true,
		}).Create(&enrollment).Error; err != nil {
			return fmt.Errorf("seed: gagal memberi akses %s ke %s: %w", def.ProductSlug, def.Email, err)
		}
	}
	return nil
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

/* ------------------------------ bank soal & tryout -------------------------- */

// seedQuestions menyisipkan/memperbarui bank soal.
//
// Memakai ON CONFLICT DO UPDATE dalam batch sehingga puluhan soal tertulis dalam
// beberapa statement saja.
// seedQuestions menyisipkan/memperbarui bank soal.
//
// Sejak soal dimiliki per paket tryout (kolom `tryout_id`), satu definisi soal
// yang bank-nya dipakai beberapa tryout disalin ke tiap tryout tersebut: baris
// pertama memakai id asli dari data.json, salinannya memakai UUID deterministik
// sehingga seed berulang tidak menambah baris baru.
//
// Seed menguasai seluruh soal pada paket tryout demo: baris di luar himpunan
// kanonik (mis. hasil penggandaan migrasi 000011, atau soal yang dibuat admin
// pada tryout demo) dihapus agar hasilnya tetap sama setiap kali dijalankan.
func seedQuestions(ctx context.Context, db *gorm.DB, defs []QuestionDef, tryouts []TryoutDef) error {
	if len(defs) == 0 {
		return nil
	}

	// Peta bank → tryout yang memakainya (lewat `subtests.banks`).
	bankOwners := make(map[string]map[string]bool, len(defs))
	for _, tryout := range tryouts {
		for _, subtest := range tryout.Subtests {
			for _, bankKey := range subtest.Banks {
				if bankOwners[bankKey] == nil {
					bankOwners[bankKey] = map[string]bool{}
				}
				bankOwners[bankKey][tryout.ID] = true
			}
		}
	}

	questions := make([]models.Question, 0, len(defs))
	canonical := make([]string, 0, len(defs))
	for _, def := range defs {
		owners := make([]string, 0, 4)
		for _, tryout := range tryouts {
			if bankOwners[def.BankKey][tryout.ID] {
				owners = append(owners, tryout.ID)
			}
		}

		if len(owners) == 0 {
			// Bank yang tidak dipakai tryout mana pun tetap disimpan sebagai soal
			// "belum ditautkan" agar tidak hilang dari panel bank soal.
			questions = append(questions, newSeedQuestion(def, def.ID, nil))
			canonical = append(canonical, def.ID)
			continue
		}

		for index, tryoutID := range owners {
			id := def.ID
			if index > 0 {
				id = stableID("question/" + tryoutID + "/" + def.ID)
			}
			owner := tryoutID
			questions = append(questions, newSeedQuestion(def, id, &owner))
			canonical = append(canonical, id)
		}
	}

	tryoutIDs := make([]string, 0, len(tryouts))
	for _, tryout := range tryouts {
		tryoutIDs = append(tryoutIDs, tryout.ID)
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"bank_key", "tryout_id", "text", "options", "answer_index",
				"explanation", "difficulty", "position", "active", "updated_at",
			}),
		}).CreateInBatches(questions, 50).Error; err != nil {
			return err
		}

		// Buang soal demo yang tidak lagi bagian dari himpunan kanonik.
		if len(tryoutIDs) == 0 {
			return nil
		}
		return tx.Where("tryout_id IN ?", tryoutIDs).
			Where("id NOT IN ?", canonical).
			Delete(&models.Question{}).Error
	})
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan bank soal: %w", err)
	}
	return nil
}

// newSeedQuestion membentuk model soal dari definisi data.json.
func newSeedQuestion(def QuestionDef, id string, tryoutID *string) models.Question {
	return models.Question{
		ID:          id,
		BankKey:     def.BankKey,
		TryoutID:    tryoutID,
		Text:        def.Text,
		Options:     models.StringList(def.Options),
		AnswerIndex: def.AnswerIndex,
		Explanation: def.Explanation,
		Difficulty:  models.Difficulty(def.Difficulty),
		Position:    def.Position,
		Active:      true,
	}
}

// seedTryouts menyisipkan/memperbarui paket tryout beserta subtesnya.
//
// Subtes diganti seluruhnya di dalam satu transaksi agar susunan yang lama tidak
// tertinggal ketika paket diubah (pola yang sama dipakai repository admin).
func seedTryouts(ctx context.Context, db *gorm.DB, defs []TryoutDef) error {
	for _, def := range defs {
		pkg := models.TryoutPackage{
			ID:              def.ID,
			Title:           def.Title,
			Description:     def.Description,
			Tier:            models.Tier(def.Tier),
			RequiredProduct: stringPtr(def.RequiredProduct),
			Attempts:        def.Attempts,
			AccessUntil:     def.AccessUntil,
			Active:          true,
			SortOrder:       def.SortOrder,
		}

		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"title", "description", "tier", "required_product",
					"attempts", "access_until", "active", "sort_order", "updated_at",
				}),
			}).Create(&pkg).Error; err != nil {
				return err
			}

			if err := tx.Where("tryout_id = ?", def.ID).Delete(&models.Subtest{}).Error; err != nil {
				return err
			}
			if len(def.Subtests) == 0 {
				return nil
			}

			subtests := make([]models.Subtest, 0, len(def.Subtests))
			for index, item := range def.Subtests {
				subtests = append(subtests, models.Subtest{
					Base:      models.Base{ID: stableID("subtest/" + def.ID + "/" + item.Name)},
					TryoutID:  def.ID,
					Name:      item.Name,
					Minutes:   item.Minutes,
					Banks:     models.StringList(item.Banks),
					SortOrder: index,
				})
			}
			return tx.Create(&subtests).Error
		})
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan paket tryout %s: %w", def.ID, err)
		}
	}
	return nil
}

/* -------------------------------- ruang kelas ------------------------------- */

// seedClassroom menyisipkan/memperbarui modul dan materi ruang kelas.
//
// Materi video mengarah ke berkas di `media/lessons/` (disajikan backend pada
// `/api/v1/media/*`), sedangkan materi e-book memakai kolom `content` yang
// dirender menjadi PDF saat diminta (`GET /classroom/lessons/{id}/ebook`).
func seedClassroom(ctx context.Context, db *gorm.DB, dataset *ClassroomDataset) error {
	for _, moduleDef := range dataset.Modules {
		module := models.CourseModule{
			ID:          moduleDef.ID,
			ProductSlug: moduleDef.ProductSlug,
			Title:       moduleDef.Title,
			Description: moduleDef.Description,
			SortOrder:   moduleDef.SortOrder,
			Active:      true,
		}
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"product_slug", "title", "description", "sort_order", "active", "updated_at",
			}),
		}).Create(&module).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan modul %s: %w", moduleDef.ID, err)
		}

		for _, lessonDef := range moduleDef.Lessons {
			lesson := models.Lesson{
				ID:              lessonDef.ID,
				ModuleID:        moduleDef.ID,
				ProductSlug:     moduleDef.ProductSlug,
				Title:           lessonDef.Title,
				Summary:         lessonDef.Summary,
				Kind:            models.LessonKind(lessonDef.Kind),
				VideoURL:        stringPtr(lessonDef.VideoURL),
				EbookURL:        stringPtr(lessonDef.EbookURL),
				PosterURL:       stringPtr(lessonDef.PosterURL),
				Content:         lessonDef.Content,
				DurationSeconds: lessonDef.DurationSeconds,
				SortOrder:       lessonDef.SortOrder,
				Active:          true,
			}
			err := db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"module_id", "product_slug", "title", "summary", "kind",
					"video_url", "ebook_url", "poster_url", "content",
					"duration_seconds", "sort_order", "active", "updated_at",
				}),
			}).Create(&lesson).Error
			if err != nil {
				return fmt.Errorf("seed: gagal menyimpan materi %s: %w", lessonDef.ID, err)
			}
		}
	}
	return nil
}

/* ---------------------------------- katalog --------------------------------- */

// seedPrograms menyisipkan/memperbarui program katalog publik.
func seedPrograms(ctx context.Context, db *gorm.DB, defs []ProgramDef) error {
	for _, def := range defs {
		program := models.Program{
			Base:        models.Base{ID: stableID("program/" + def.Slug)},
			Slug:        def.Slug,
			Name:        def.Name,
			Tagline:     def.Tagline,
			Audience:    def.Audience,
			Format:      def.Format,
			Duration:    def.Duration,
			Features:    models.StringList(def.Features),
			Highlighted: def.Highlighted,
			SortOrder:   def.SortOrder,
		}
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "slug"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "tagline", "audience", "format", "duration",
				"features", "highlighted", "sort_order", "updated_at",
			}),
		}).Create(&program).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan program %s: %w", def.Slug, err)
		}
	}
	return nil
}

// seedMateriCategories menyisipkan/memperbarui kategori materi.
func seedMateriCategories(ctx context.Context, db *gorm.DB, defs []MateriCategoryDef) error {
	for _, def := range defs {
		category := models.MateriCategory{
			ID:          def.ID,
			Label:       def.Label,
			Description: def.Description,
			Topics:      models.StringList(def.Topics),
			SortOrder:   def.SortOrder,
		}
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"label", "description", "topics", "sort_order", "updated_at",
			}),
		}).Create(&category).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan kategori materi %s: %w", def.ID, err)
		}
	}
	return nil
}

// seedProducts menyisipkan/memperbarui produk yang dijual.
func seedProducts(ctx context.Context, db *gorm.DB, defs []ProductDef) error {
	for _, def := range defs {
		product := models.Product{
			Base:        models.Base{ID: stableID("product/" + def.Slug)},
			Slug:        def.Slug,
			Name:        def.Name,
			Description: def.Description,
			Price:       def.Price,
			Type:        models.ProductType(def.Type),
			Includes:    models.StringList(def.Includes),
			Badge:       stringPtr(def.Badge),
			Active:      true,
			SortOrder:   def.SortOrder,
		}
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "slug"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "description", "price", "type", "includes",
				"badge", "active", "sort_order", "updated_at",
			}),
		}).Create(&product).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan produk %s: %w", def.Slug, err)
		}
	}
	return nil
}

// seedPaymentMethods menyisipkan/memperbarui metode pembayaran.
func seedPaymentMethods(ctx context.Context, db *gorm.DB, defs []PaymentMethodDef) error {
	for _, def := range defs {
		method := models.PaymentMethod{
			ID:        def.ID,
			Label:     def.Label,
			Channel:   def.Channel,
			Fee:       def.Fee,
			Active:    true,
			SortOrder: def.SortOrder,
		}
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"label", "channel", "fee", "active", "sort_order",
			}),
		}).Create(&method).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan metode bayar %s: %w", def.ID, err)
		}
	}
	return nil
}

// seedVouchers menyisipkan/memperbarui kode promo.
//
// `used_count` sengaja tidak di-reset agar kuota voucher tetap terjaga setelah
// dipakai pada data demo.
func seedVouchers(ctx context.Context, db *gorm.DB, defs []VoucherDef) error {
	for _, def := range defs {
		voucher := models.Voucher{
			Code:        def.Code,
			Label:       def.Label,
			Type:        models.VoucherType(def.Type),
			Value:       def.Value,
			MinPurchase: def.MinPurchase,
			MaxUses:     def.MaxUses,
			Active:      true,
		}
		err := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"label", "type", "value", "min_purchase", "max_uses", "active", "updated_at",
			}),
		}).Create(&voucher).Error
		if err != nil {
			return fmt.Errorf("seed: gagal menyimpan voucher %s: %w", def.Code, err)
		}
	}
	return nil
}
