package seed

import (
	"context"
	"fmt"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Seed kamus kompetensi (Fase 3 langkah 1, masukan #2 "pemetaan skill dan
// kompetensi"): kamus kompetensi + deskriptor level, standar jabatan dasar, dan
// pemetaan kategori bank soal → kompetensi.
//
// Pemetaan bank soal inilah yang membuat data latihan yang sudah ada langsung
// menghasilkan level kompetensi (proksi), tanpa menunggu instrumen berlisensi.

// competencyDef adalah satu entri kamus kompetensi demo.
type competencyDef struct {
	Code       string
	Name       string
	Cluster    string
	Definition string
}

var competencyDefs = []competencyDef{
	{"penalaran_numerik", "Penalaran Numerik", models.CompetencyClusterTeknis,
		"Mengolah angka, rasio, dan pola kuantitatif untuk mengambil keputusan."},
	{"penalaran_verbal", "Penalaran Verbal", models.CompetencyClusterTeknis,
		"Memahami hubungan kata, makna, dan gagasan tertulis."},
	{"penalaran_analitis", "Penalaran Analitis", models.CompetencyClusterTeknis,
		"Menalar logis, menemukan pola, dan menarik simpulan dari data."},
	{"penalaran_figural", "Penalaran Figural", models.CompetencyClusterTeknis,
		"Mengenali pola gambar dan hubungan ruang."},
	{"integritas_akhlaq", "Integritas & AKHLAK", models.CompetencyClusterSosialKultural,
		"Berperilaku sesuai nilai AKHLAK: amanah, kompeten, harmonis, loyal, adaptif, kolaboratif."},
	{"wawasan_kebangsaan", "Wawasan Kebangsaan", models.CompetencyClusterSosialKultural,
		"Pemahaman kebangsaan, kenegaraan, dan nilai Pancasila."},
	{"bahasa_inggris", "Bahasa Inggris", models.CompetencyClusterTeknis,
		"Membaca dan memahami struktur bahasa Inggris profesional."},
	{"akuntansi_keuangan", "Akuntansi & Keuangan", models.CompetencyClusterTeknis,
		"Dasar akuntansi, laporan keuangan, dan analisis keuangan."},
	{"manajemen_risiko", "Manajemen Risiko & Kepatuhan", models.CompetencyClusterTeknis,
		"Mengidentifikasi, menilai, dan memitigasi risiko serta menjaga kepatuhan."},
	{"teknis_produksi", "Teknis Produksi & Mutu", models.CompetencyClusterTeknis,
		"Menjalankan proses produksi dan pengendalian mutu."},
	{"teknis_operasi", "Teknis Operasi", models.CompetencyClusterTeknis,
		"Mengoperasikan dan memelihara peralatan serta sistem operasi."},
	{"k3_lingkungan", "K3 & Lingkungan", models.CompetencyClusterTeknis,
		"Menerapkan keselamatan, kesehatan kerja, dan perlindungan lingkungan."},
	{"teknologi_informasi", "Teknologi Informasi & Data", models.CompetencyClusterTeknis,
		"Dasar teknologi informasi, pengolahan data, dan keamanan dasar."},
	{"layanan_pelanggan", "Layanan Pelanggan", models.CompetencyClusterSosialKultural,
		"Berkomunikasi dan melayani pelanggan secara profesional."},
	{"manajemen_proyek", "Manajemen Proyek", models.CompetencyClusterManajerial,
		"Merencanakan, menjalankan, dan mengendalikan proyek."},
}

// competencyLevelDescriptors membentuk deskriptor level 1–5 untuk satu kompetensi.
//
// Memakai pola kalimat yang sama untuk semua kompetensi agar matriks mudah dibaca,
// dengan kata kerja yang naik tingkat (memahami → menerapkan → menganalisis →
// menjadi rujukan).
func competencyLevelDescriptors(name string) [5]string {
	return [5]string{
		fmt.Sprintf("Memahami dasar %s dengan pendampingan.", name),
		fmt.Sprintf("Menerapkan %s pada tugas rutin sesuai panduan.", name),
		fmt.Sprintf("Menerapkan %s secara mandiri pada kasus yang beragam.", name),
		fmt.Sprintf("Menganalisis persoalan %s yang kompleks dan membimbing rekan.", name),
		fmt.Sprintf("Menjadi rujukan %s: menyusun standar dan mengembangkan orang lain.", name),
	}
}

// positionRequirementDef adalah standar minimum satu kompetensi pada satu keluarga jabatan.
type positionRequirementDef struct {
	Family   string
	Sector   string
	Code     string
	MinLevel int
}

// positionRequirementDefs adalah standar jabatan dasar untuk beberapa keluarga jabatan.
var positionRequirementDefs = []positionRequirementDef{
	{"keuangan", "umum", "penalaran_numerik", 4},
	{"keuangan", "umum", "akuntansi_keuangan", 4},
	{"keuangan", "umum", "manajemen_risiko", 3},
	{"keuangan", "umum", "penalaran_analitis", 3},
	{"keuangan", "umum", "integritas_akhlaq", 4},

	{"manufaktur", "umum", "teknis_produksi", 4},
	{"manufaktur", "umum", "k3_lingkungan", 4},
	{"manufaktur", "umum", "penalaran_numerik", 3},
	{"manufaktur", "umum", "integritas_akhlaq", 4},

	{"operasi", "umum", "teknis_operasi", 4},
	{"operasi", "umum", "k3_lingkungan", 4},
	{"operasi", "umum", "penalaran_analitis", 3},
	{"operasi", "umum", "integritas_akhlaq", 4},

	{"ti", "umum", "teknologi_informasi", 4},
	{"ti", "umum", "penalaran_analitis", 4},
	{"ti", "umum", "penalaran_numerik", 3},
	{"ti", "umum", "integritas_akhlaq", 4},

	{"sdm", "umum", "layanan_pelanggan", 3},
	{"sdm", "umum", "penalaran_verbal", 3},
	{"sdm", "umum", "wawasan_kebangsaan", 3},
	{"sdm", "umum", "integritas_akhlaq", 5},

	{"umum", "umum", "penalaran_verbal", 3},
	{"umum", "umum", "penalaran_numerik", 3},
	{"umum", "umum", "penalaran_analitis", 3},
	{"umum", "umum", "bahasa_inggris", 3},
	{"umum", "umum", "integritas_akhlaq", 4},
}

// bankCompetencyDef memetakan kategori bank soal → kompetensi.
//
// Beberapa kunci adalah **alias** kategori yang muncul di data demo (mis.
// `akhlak`, `inggris`, `kognitif`) agar seluruh subtes tryout terpetakan.
type bankCompetencyDef struct {
	BankKey string
	Code    string
}

var bankCompetencyDefs = []bankCompetencyDef{
	{"numerik", "penalaran_numerik"},
	{"verbal", "penalaran_verbal"},
	{"analitis", "penalaran_analitis"},
	{"silogisme", "penalaran_analitis"},
	{"kognitif", "penalaran_analitis"},
	{"figural", "penalaran_figural"},
	{"akhlaq", "integritas_akhlaq"},
	{"akhlak", "integritas_akhlaq"},
	{"wawasan", "wawasan_kebangsaan"},
	{"reading", "bahasa_inggris"},
	{"structure", "bahasa_inggris"},
	{"inggris", "bahasa_inggris"},
	{"akuntansi", "akuntansi_keuangan"},
	{"risiko_kepatuhan", "manajemen_risiko"},
	{"teknis_produksi", "teknis_produksi"},
	{"teknis_operasi", "teknis_operasi"},
	{"k3", "k3_lingkungan"},
	{"hse", "k3_lingkungan"},
	{"it_dasar", "teknologi_informasi"},
	{"layanan", "layanan_pelanggan"},
	{"manajemen_proyek", "manajemen_proyek"},
}

/* ------------------------------ fungsi seed ------------------------------- */

// seedCompetencies menyimpan kamus kompetensi + deskriptor level (idempoten).
//
// Kolom `active` sengaja tidak ikut diperbarui: admin boleh menonaktifkan
// kompetensi, dan menjalankan seed ulang tidak boleh mengaktifkannya kembali.
func seedCompetencies(ctx context.Context, db *gorm.DB) error {
	competencies := make([]models.Competency, 0, len(competencyDefs))
	levels := make([]models.CompetencyLevel, 0, len(competencyDefs)*5)
	for _, def := range competencyDefs {
		competencies = append(competencies, models.Competency{
			Code:       def.Code,
			Name:       def.Name,
			Cluster:    def.Cluster,
			Definition: def.Definition,
			Active:     true,
		})
		for index, descriptor := range competencyLevelDescriptors(def.Name) {
			levels = append(levels, models.CompetencyLevel{
				CompetencyCode: def.Code,
				Level:          index + 1,
				Descriptor:     descriptor,
			})
		}
	}

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "cluster", "definition"}),
	}).Create(&competencies).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan kamus kompetensi: %w", err)
	}

	err = db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "competency_code"}, {Name: "level"}},
		DoUpdates: clause.AssignmentColumns([]string{"descriptor"}),
	}).Create(&levels).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan level kompetensi: %w", err)
	}
	return nil
}

// seedPositionRequirements menyimpan standar kompetensi jabatan dasar (idempoten).
func seedPositionRequirements(ctx context.Context, db *gorm.DB) error {
	items := make([]models.PositionRequirement, 0, len(positionRequirementDefs))
	for _, def := range positionRequirementDefs {
		items = append(items, models.PositionRequirement{
			PositionFamily: def.Family,
			SectorKey:      def.Sector,
			CompetencyCode: def.Code,
			MinLevel:       def.MinLevel,
			Weight:         1,
		})
	}

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "position_family"}, {Name: "sector_key"}, {Name: "competency_code"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"min_level", "weight"}),
	}).Create(&items).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan standar kompetensi jabatan: %w", err)
	}
	return nil
}

// seedBankCompetencyMap menyimpan pemetaan kategori bank soal → kompetensi (idempoten).
func seedBankCompetencyMap(ctx context.Context, db *gorm.DB) error {
	items := make([]models.BankCompetencyMap, 0, len(bankCompetencyDefs))
	for _, def := range bankCompetencyDefs {
		items = append(items, models.BankCompetencyMap{
			BankKey:        def.BankKey,
			CompetencyCode: def.Code,
			Weight:         1,
		})
	}

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "bank_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"competency_code", "weight"}),
	}).Create(&items).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan pemetaan bank soal: %w", err)
	}
	return nil
}
