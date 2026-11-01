package seed

import (
	"context"
	"fmt"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Seed instrumen minat–bakat & bobot lini bisnis (Fase 3 langkah 3–4, masukan #1).
//
// Dua instrumen demo disiapkan:
//   - `riasec-v1`  → dimensi minat RIASEC/Holland (6 dimensi, 4 butir/dimensi);
//   - `aptitude-v1` → dimensi bakat (numerik, verbal, figural, learning agility).
//
// Keduanya instrumen **latihan milik kami** (bukan alat tes berlisensi), sehingga
// interpretasinya tetap bahan pertimbangan — bukan diagnosis. Instrumen berlisensi
// (mis. IST/Papi Kostick) menyusul lewat registri Fase 0 + mitra psikolog.

// talentNormDef adalah parameter norma demo satu dimensi.
type talentNormDef struct {
	Dimension string
	Mean      float64
	SD        float64
}

// talentInstrumentDef adalah definisi instrumen demo.
type talentInstrumentDef struct {
	ID          string
	Code        string
	Name        string
	Kind        string
	Version     string
	NormVersion string
	Description string
	Items       []talentItemDef
}

// talentItemDef adalah satu butir instrumen demo.
type talentItemDef struct {
	Dimension string
	Text      string
	Reverse   bool
}

// riasecItems adalah butir minat RIASEC (4 per dimensi, 1 di antaranya terbalik).
var riasecItems = []talentItemDef{
	{models.TalentDimensionRealistic, "Saya suka bekerja dengan alat, mesin, atau perangkat fisik.", false},
	{models.TalentDimensionRealistic, "Saya senang memperbaiki benda yang rusak.", false},
	{models.TalentDimensionRealistic, "Saya tertarik pada pekerjaan lapangan yang praktis.", false},
	{models.TalentDimensionRealistic, "Saya cepat bosan bila harus bekerja dengan benda nyata.", true},

	{models.TalentDimensionInvestigative, "Saya suka menganalisis data untuk menemukan pola.", false},
	{models.TalentDimensionInvestigative, "Saya senang memecahkan masalah yang rumit.", false},
	{models.TalentDimensionInvestigative, "Saya tertarik mempelajari teori di balik suatu fenomena.", false},
	{models.TalentDimensionInvestigative, "Saya menghindari tugas yang menuntut penalaran mendalam.", true},

	{models.TalentDimensionArtistic, "Saya suka menuangkan ide dalam bentuk visual atau tulisan.", false},
	{models.TalentDimensionArtistic, "Saya senang menciptakan sesuatu yang orisinal.", false},
	{models.TalentDimensionArtistic, "Saya tertarik pada desain, musik, atau seni.", false},
	{models.TalentDimensionArtistic, "Saya lebih suka tugas yang sepenuhnya mengikuti contoh yang ada.", true},

	{models.TalentDimensionSocial, "Saya senang membantu orang lain belajar hal baru.", false},
	{models.TalentDimensionSocial, "Saya suka bekerja dalam tim yang saling mendukung.", false},
	{models.TalentDimensionSocial, "Saya mudah memahami perasaan orang lain.", false},
	{models.TalentDimensionSocial, "Saya lebih suka bekerja sendiri daripada berdiskusi dengan orang lain.", true},

	{models.TalentDimensionEnterprising, "Saya suka memimpin dan mengarahkan orang lain.", false},
	{models.TalentDimensionEnterprising, "Saya senang meyakinkan orang untuk menerima gagasan saya.", false},
	{models.TalentDimensionEnterprising, "Saya nyaman mengambil keputusan yang berdampak besar.", false},
	{models.TalentDimensionEnterprising, "Saya menghindari peran yang menuntut negosiasi.", true},

	{models.TalentDimensionConventional, "Saya suka bekerja dengan data yang rapi dan terstruktur.", false},
	{models.TalentDimensionConventional, "Saya senang mengikuti prosedur yang jelas.", false},
	{models.TalentDimensionConventional, "Saya teliti dalam memeriksa detail.", false},
	{models.TalentDimensionConventional, "Saya sering mengabaikan kelengkapan dokumen.", true},
}

// aptitudeItems adalah butir bakat (3 per dimensi).
var aptitudeItems = []talentItemDef{
	{models.TalentDimensionNumerik, "Saya cepat menghitung angka tanpa alat bantu.", false},
	{models.TalentDimensionNumerik, "Saya nyaman bekerja dengan tabel dan grafik angka.", false},
	{models.TalentDimensionNumerik, "Saya bisa menaksir hasil perhitungan dengan cepat.", false},

	{models.TalentDimensionVerbal, "Saya cepat memahami maksud bacaan yang panjang.", false},
	{models.TalentDimensionVerbal, "Saya mudah menemukan ide pokok sebuah teks.", false},
	{models.TalentDimensionVerbal, "Saya senang menulis dengan kalimat yang tepat.", false},

	{models.TalentDimensionFigural, "Saya cepat mengenali pola gambar.", false},
	{models.TalentDimensionFigural, "Saya mudah membayangkan bentuk dalam ruang tiga dimensi.", false},
	{models.TalentDimensionFigural, "Saya senang menyusun potongan gambar atau diagram.", false},

	{models.TalentDimensionLearningAgility, "Saya cepat menguasai keterampilan baru.", false},
	{models.TalentDimensionLearningAgility, "Saya senang mencoba cara kerja yang belum pernah saya pakai.", false},
	{models.TalentDimensionLearningAgility, "Saya mudah beradaptasi saat aturan berubah.", false},
}

// talentInstruments adalah instrumen demo yang disemai.
//
// Norma memakai sebaran normal dengan rata-rata ~70% skor maksimum agar persentil
// demo tersebar wajar (bukan menumpuk di ujung).
var talentInstruments = []talentInstrumentDef{
	{
		ID:          "riasec-v1",
		Code:        "riasec-v1",
		Name:        "Peta Minat Kerja (RIASEC)",
		Kind:        models.TalentKindMinat,
		Version:     "v1",
		NormVersion: "norm-2026",
		Description: "Kuesioner minat kerja berbasis kerangka RIASEC/Holland: enam dimensi minat.",
		Items:       riasecItems,
	},
	{
		ID:          "aptitude-v1",
		Code:        "aptitude-v1",
		Name:        "Peta Bakat Dasar",
		Kind:        models.TalentKindBakat,
		Version:     "v1",
		NormVersion: "norm-2026",
		Description: "Penilaian diri bakat dasar: numerik, verbal, figural, dan learning agility.",
		Items:       aptitudeItems,
	},
}

// talentNorms adalah norma demo per dimensi (versi `norm-2026`).
var talentNorms = []talentNormDef{
	{models.TalentDimensionRealistic, 14, 3.5},
	{models.TalentDimensionInvestigative, 14, 3.5},
	{models.TalentDimensionArtistic, 13, 3.5},
	{models.TalentDimensionSocial, 15, 3.5},
	{models.TalentDimensionEnterprising, 14, 3.5},
	{models.TalentDimensionConventional, 14, 3.5},
	{models.TalentDimensionNumerik, 10.5, 2.5},
	{models.TalentDimensionVerbal, 10.5, 2.5},
	{models.TalentDimensionFigural, 10.5, 2.5},
	{models.TalentDimensionLearningAgility, 11, 2.5},
}

// businessLineDef adalah profil lini bisnis demo beserta bobot dimensinya.
type businessLineDef struct {
	Key         string
	SectorKey   string
	Name        string
	Description string
	Weights     map[string]float64
}

// businessLineDefs adalah bobot default untuk klaster BUMN prioritas (§10 butir 5).
var businessLineDefs = []businessLineDef{
	{
		Key: "jasa_keuangan", SectorKey: "jasa_keuangan",
		Name:        "Jasa Keuangan",
		Description: "Perbankan & jasa keuangan: menekankan ketelitian, struktur, dan penalaran analitis.",
		Weights: map[string]float64{
			models.TalentDimensionConventional:  3,
			models.TalentDimensionInvestigative: 3,
			models.TalentDimensionEnterprising:  2,
			models.TalentDimensionNumerik:       3,
			models.TalentDimensionVerbal:        2,
		},
	},
	{
		Key: "pertahanan_manufaktur", SectorKey: "pertahanan_manufaktur",
		Name:        "Pertahanan & Manufaktur",
		Description: "Produksi & manufaktur: menekankan kerja praktis, mutu, dan kedisiplinan prosedur.",
		Weights: map[string]float64{
			models.TalentDimensionRealistic:     3,
			models.TalentDimensionConventional:  3,
			models.TalentDimensionInvestigative: 2,
			models.TalentDimensionFigural:       2,
		},
	},
	{
		Key: "migas_energi", SectorKey: "migas_energi",
		Name:        "Migas & Energi",
		Description: "Energi & migas: menekankan kerja lapangan, keselamatan, dan analisis teknis.",
		Weights: map[string]float64{
			models.TalentDimensionRealistic:     3,
			models.TalentDimensionInvestigative: 3,
			models.TalentDimensionConventional:  2,
			models.TalentDimensionNumerik:       2,
		},
	},
	{
		Key: "telko_media", SectorKey: "telko_media",
		Name:        "Telekomunikasi & Media",
		Description: "Telko & media: menekankan eksplorasi gagasan, kreativitas, dan inisiatif.",
		Weights: map[string]float64{
			models.TalentDimensionInvestigative:   3,
			models.TalentDimensionArtistic:        3,
			models.TalentDimensionEnterprising:    2,
			models.TalentDimensionLearningAgility: 3,
		},
	},
	{
		Key: "infrastruktur", SectorKey: "infrastruktur",
		Name:        "Infrastruktur",
		Description: "Infrastruktur & konstruksi: menekankan kerja lapangan, ketelitian, dan penalaran teknis.",
		Weights: map[string]float64{
			models.TalentDimensionRealistic:     3,
			models.TalentDimensionConventional:  2,
			models.TalentDimensionInvestigative: 2,
			models.TalentDimensionFigural:       2,
		},
	},
	{
		Key: "umum", SectorKey: "umum",
		Name:        "Lintas Klaster",
		Description: "Profil seimbang untuk penempatan lintas klaster BUMN.",
		Weights: map[string]float64{
			models.TalentDimensionInvestigative: 2,
			models.TalentDimensionSocial:        2,
			models.TalentDimensionEnterprising:  2,
			models.TalentDimensionConventional:  2,
			models.TalentDimensionRealistic:     1,
			models.TalentDimensionArtistic:      1,
		},
	},
}

/* ------------------------------ fungsi seed ------------------------------- */

// seedTalentInstruments menyimpan instrumen, butir, dan norma demo (idempoten).
//
// Kolom `active` sengaja tidak ikut diperbarui agar admin bisa menonaktifkan
// instrumen tanpa diaktifkan kembali oleh seed.
func seedTalentInstruments(ctx context.Context, db *gorm.DB) error {
	instruments := make([]models.TalentInstrument, 0, len(talentInstruments))
	items := make([]models.TalentItem, 0)
	for _, def := range talentInstruments {
		instruments = append(instruments, models.TalentInstrument{
			ID:          def.ID,
			Code:        def.Code,
			Name:        def.Name,
			Kind:        def.Kind,
			Version:     def.Version,
			NormVersion: def.NormVersion,
			Description: def.Description,
			Active:      true,
		})
		for index, item := range def.Items {
			items = append(items, models.TalentItem{
				IDBase:       models.IDBase{ID: stableID(fmt.Sprintf("talent-item/%s/%s/%d", def.ID, item.Dimension, index))},
				InstrumentID: def.ID,
				Text:         item.Text,
				Dimension:    item.Dimension,
				Weight:       1,
				Reverse:      item.Reverse,
				SortOrder:    index,
			})
		}
	}

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"code", "name", "kind", "version", "norm_version", "description",
		}),
	}).Create(&instruments).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan instrumen minat–bakat: %w", err)
	}

	err = db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"instrument_id", "text", "dimension", "weight", "reverse", "sort_order",
		}),
	}).Create(&items).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan butir instrumen: %w", err)
	}

	norms := make([]models.TalentNorm, 0, len(talentNorms))
	for _, def := range talentNorms {
		norms = append(norms, models.TalentNorm{
			NormVersion: "norm-2026",
			Dimension:   def.Dimension,
			Mean:        def.Mean,
			SD:          def.SD,
		})
	}
	err = db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "norm_version"}, {Name: "dimension"}},
		DoUpdates: clause.AssignmentColumns([]string{"mean", "sd"}),
	}).Create(&norms).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan norma instrumen: %w", err)
	}
	return nil
}

// seedBusinessLines menyimpan bobot lini bisnis demo (idempoten).
func seedBusinessLines(ctx context.Context, db *gorm.DB) error {
	lines := make([]models.BusinessLineProfile, 0, len(businessLineDefs))
	for _, def := range businessLineDefs {
		weights := make(models.JSONMap, len(def.Weights))
		for dimension, weight := range def.Weights {
			weights[dimension] = weight
		}
		lines = append(lines, models.BusinessLineProfile{
			IDBase:      models.IDBase{ID: stableID("business-line/" + def.Key)},
			SectorKey:   def.SectorKey,
			Name:        def.Name,
			Weights:     weights,
			Description: def.Description,
			Active:      true,
		})
	}

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"sector_key", "name", "weights", "description",
		}),
	}).Create(&lines).Error
	if err != nil {
		return fmt.Errorf("seed: gagal menyimpan bobot lini bisnis: %w", err)
	}
	return nil
}
