package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// CompetencyService mengelola kamus kompetensi, standar jabatan, penilaian
// peserta (tiga sumber), analisis gap, dan matriks kompetensi (Fase 3 langkah 1–2).
//
// Aturan yang dipegang:
//   - level dari `tryout` adalah **proksi latihan** dan selalu berlabel sumber,
//     sehingga tidak disalahartikan sebagai asesmen formal (§2.4);
//   - peserta tanpa data masuk kelompok "data belum cukup" (level 0), bukan nilai
//     nol yang terlihat seperti kelemahan (§4.4);
//   - gap dihitung dari level **tertinggi lintas sumber** terhadap standar jabatan.
type CompetencyService struct {
	repo *repository.CompetencyRepository
}

// NewCompetencyService membentuk service kompetensi.
func NewCompetencyService(repo *repository.CompetencyRepository) *CompetencyService {
	return &CompetencyService{repo: repo}
}

/* ---------------------------- helper murni -------------------------------- */

// Ambang konversi skor latihan → level kompetensi (proksi tryout, §4.3).
//
// Disimpan di satu tempat agar aturan ini bisa diuji dan diubah sekali:
// <50 → 2, 50–69 → 3, 70–84 → 4, ≥85 → 5. Level 1 tidak pernah dihasilkan
// tryout — itu hanya muncul dari penilaian manusia (diri/asesor/psikolog).
const (
	competencyLevel5Min = 85
	competencyLevel4Min = 70
	competencyLevel3Min = 50
)

// competencyLevelFromScore mengubah skor 0–100 menjadi level 1–5.
func competencyLevelFromScore(score float64) int {
	switch {
	case score >= competencyLevel5Min:
		return 5
	case score >= competencyLevel4Min:
		return 4
	case score >= competencyLevel3Min:
		return 3
	default:
		return 2
	}
}

// competencyCategoryMatches memeriksa apakah nama kategori soal (subtes) memetakan
// ke sebuah kunci bank soal.
//
// Pencocokan case-insensitive memakai substring: nama subtes seperti "Penalaran
// Numerik" tetap terpetakan ke kunci "numerik" (atau labelnya "Penalaran Numerik").
func competencyCategoryMatches(categoryName, bankKey string) bool {
	name := strings.ToLower(strings.TrimSpace(categoryName))
	if name == "" {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(bankKey))
	if key != "" && strings.Contains(name, key) {
		return true
	}
	label := strings.ToLower(strings.TrimSpace(bank.Label(bankKey)))
	return label != "" && label != key && strings.Contains(name, label)
}

// competencyGap menghitung kesenjangan level terhadap standar jabatan.
func competencyGap(need, have int) (int, bool) {
	gap := need - have
	if gap < 0 {
		gap = 0
	}
	return gap, gap == 0
}

// competencyInitials membentuk inisial dari nama (maksimal dua huruf).
func competencyInitials(name string) string {
	out := make([]string, 0, 2)
	for _, field := range strings.Fields(strings.TrimSpace(name)) {
		if field == "" {
			continue
		}
		out = append(out, strings.ToUpper(string([]rune(field)[0])))
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return strings.Join(out, "")
}

// competencyBestLevel mengumpulkan level tertinggi per kompetensi lintas sumber.
func competencyBestLevel(rows []models.UserCompetency) map[string]int {
	best := make(map[string]int, len(rows))
	for _, row := range rows {
		if row.Level > best[row.CompetencyCode] {
			best[row.CompetencyCode] = row.Level
		}
	}
	return best
}

// tryoutDerivation adalah hasil proksi tryout untuk satu kompetensi.
type tryoutDerivation struct {
	Level    int
	Evidence string
}

// competencyTryoutLevels menurunkan level kompetensi dari skor kategori peserta.
//
// Bila beberapa kategori memetakan ke kompetensi yang sama, level tertinggi yang
// dipakai dan buktinya menyebut kategori dengan skor tertinggi itu.
func competencyTryoutLevels(
	scores []repository.UserSkillScoreRow,
	bankMap []models.BankCompetencyMap,
) map[string]tryoutDerivation {
	out := make(map[string]tryoutDerivation)
	for _, score := range scores {
		for _, mapping := range bankMap {
			if !competencyCategoryMatches(score.Name, mapping.BankKey) {
				continue
			}
			level := competencyLevelFromScore(score.AverageScore)
			if current, ok := out[mapping.CompetencyCode]; ok && level <= current.Level {
				continue
			}
			out[mapping.CompetencyCode] = tryoutDerivation{
				Level:    level,
				Evidence: fmt.Sprintf("Rata-rata skor %s: %.1f", score.Name, score.AverageScore),
			}
		}
	}
	return out
}

/* -------------------------------- kamus ----------------------------------- */

// Framework mengembalikan kamus kompetensi lengkap (kompetensi + level + pemetaan bank).
func (s *CompetencyService) Framework(ctx context.Context) (dto.CompetencyFrameworkView, error) {
	items, err := s.repo.ListCompetencies(ctx, false)
	if err != nil {
		return dto.CompetencyFrameworkView{}, err
	}
	levels, err := s.repo.ListAllLevels(ctx)
	if err != nil {
		return dto.CompetencyFrameworkView{}, err
	}
	bankMap, err := s.repo.ListBankMap(ctx)
	if err != nil {
		return dto.CompetencyFrameworkView{}, err
	}

	levelsByCode := make(map[string][]dto.CompetencyLevelView, len(items))
	for _, level := range levels {
		levelsByCode[level.CompetencyCode] = append(levelsByCode[level.CompetencyCode],
			dto.CompetencyLevelView{Level: level.Level, Descriptor: level.Descriptor})
	}

	nameByCode := make(map[string]string, len(items))
	views := make([]dto.CompetencyView, 0, len(items))
	for _, item := range items {
		nameByCode[item.Code] = item.Name
		views = append(views, dto.CompetencyView{
			Code:         item.Code,
			Name:         item.Name,
			Cluster:      item.Cluster,
			ClusterLabel: models.CompetencyClusterLabel(item.Cluster),
			Definition:   item.Definition,
			Active:       item.Active,
			Levels:       levelsByCode[item.Code],
		})
	}

	mappings := make([]dto.BankCompetencyMapping, 0, len(bankMap))
	for _, mapping := range bankMap {
		mappings = append(mappings, dto.BankCompetencyMapping{
			BankKey:        mapping.BankKey,
			BankLabel:      bank.Label(mapping.BankKey),
			CompetencyCode: mapping.CompetencyCode,
			CompetencyName: nameByCode[mapping.CompetencyCode],
			Weight:         mapping.Weight,
		})
	}

	return dto.CompetencyFrameworkView{
		Competencies: views,
		BankMap:      mappings,
		Clusters: []string{
			models.CompetencyClusterTeknis,
			models.CompetencyClusterManajerial,
			models.CompetencyClusterSosialKultural,
		},
	}, nil
}

// UpsertCompetency menambah/mengubah satu kompetensi dan mengembalikan framework.
func (s *CompetencyService) UpsertCompetency(ctx context.Context, in dto.CompetencyInput) (dto.CompetencyFrameworkView, error) {
	cluster := strings.TrimSpace(in.Cluster)
	if cluster == "" {
		cluster = models.CompetencyClusterTeknis
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}

	item := models.Competency{
		Code:       strings.ToLower(strings.TrimSpace(in.Code)),
		Name:       strings.TrimSpace(in.Name),
		Cluster:    cluster,
		Definition: strings.TrimSpace(in.Definition),
		Active:     active,
	}
	if err := s.repo.UpsertCompetency(ctx, &item); err != nil {
		return dto.CompetencyFrameworkView{}, err
	}
	return s.Framework(ctx)
}

// UpsertLevel menyimpan deskriptor satu level kompetensi dan mengembalikan framework.
func (s *CompetencyService) UpsertLevel(ctx context.Context, code string, in dto.CompetencyLevelInput) (dto.CompetencyFrameworkView, error) {
	found, err := s.repo.FindCompetency(ctx, code)
	if err != nil {
		return dto.CompetencyFrameworkView{}, err
	}
	if found == nil {
		return dto.CompetencyFrameworkView{}, apperror.NotFound("Kompetensi tidak ditemukan.")
	}

	level := models.CompetencyLevel{
		CompetencyCode: code,
		Level:          in.Level,
		Descriptor:     strings.TrimSpace(in.Descriptor),
	}
	if err := s.repo.UpsertLevel(ctx, &level); err != nil {
		return dto.CompetencyFrameworkView{}, err
	}
	return s.Framework(ctx)
}

/* ---------------------------- standar jabatan ----------------------------- */

// Requirements mengembalikan standar kompetensi satu keluarga jabatan.
func (s *CompetencyService) Requirements(ctx context.Context, family string) ([]dto.PositionRequirementView, error) {
	items, err := s.repo.ListRequirements(ctx, family, "")
	if err != nil {
		return nil, err
	}
	competencies, err := s.repo.ListCompetencies(ctx, false)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]models.Competency, len(competencies))
	for _, item := range competencies {
		byCode[item.Code] = item
	}

	out := make([]dto.PositionRequirementView, 0, len(items))
	for _, item := range items {
		competency := byCode[item.CompetencyCode]
		out = append(out, dto.PositionRequirementView{
			PositionFamily:    item.PositionFamily,
			PositionLabel:     bank.PositionFamilyLabel(item.PositionFamily),
			SectorKey:         item.SectorKey,
			SectorLabel:       bank.SectorLabel(item.SectorKey),
			CompetencyCode:    item.CompetencyCode,
			CompetencyName:    competency.Name,
			CompetencyCluster: competency.Cluster,
			MinLevel:          item.MinLevel,
			Weight:            item.Weight,
		})
	}
	return out, nil
}

// UpsertRequirement menambah/mengubah satu standar kompetensi jabatan.
func (s *CompetencyService) UpsertRequirement(ctx context.Context, family string, in dto.PositionRequirementInput) ([]dto.PositionRequirementView, error) {
	found, err := s.repo.FindCompetency(ctx, in.CompetencyCode)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, apperror.Validation("Kompetensi tidak dikenal.", nil)
	}

	sector := strings.TrimSpace(in.SectorKey)
	if sector == "" {
		sector = bank.DefaultSector
	}
	weight := in.Weight
	if weight <= 0 {
		weight = 1
	}

	item := models.PositionRequirement{
		PositionFamily: family,
		SectorKey:      sector,
		CompetencyCode: in.CompetencyCode,
		MinLevel:       in.MinLevel,
		Weight:         weight,
	}
	if err := s.repo.UpsertRequirement(ctx, &item); err != nil {
		return nil, err
	}
	return s.Requirements(ctx, family)
}

// DeleteRequirement menghapus satu standar kompetensi jabatan.
func (s *CompetencyService) DeleteRequirement(ctx context.Context, family, sector, code string) error {
	if strings.TrimSpace(sector) == "" {
		sector = bank.DefaultSector
	}
	return s.repo.DeleteRequirement(ctx, family, sector, code)
}

/* --------------------------- profil peserta ------------------------------- */

// SelfAssess menyimpan penilaian diri satu kompetensi (sumber `self`).
func (s *CompetencyService) SelfAssess(ctx context.Context, userID string, in dto.SelfAssessmentInput) (dto.CompetencyProfileView, error) {
	found, err := s.repo.FindCompetency(ctx, in.CompetencyCode)
	if err != nil {
		return dto.CompetencyProfileView{}, err
	}
	if found == nil || !found.Active {
		return dto.CompetencyProfileView{}, apperror.Validation("Kompetensi tidak dikenal atau tidak aktif.", nil)
	}

	item := models.UserCompetency{
		UserID:         userID,
		CompetencyCode: found.Code,
		Level:          in.Level,
		Source:         models.CompetencySourceSelf,
		Evidence:       strings.TrimSpace(in.Evidence),
		AssessedAt:     time.Now().UTC(),
	}
	if err := s.repo.UpsertUserCompetency(ctx, &item); err != nil {
		return dto.CompetencyProfileView{}, err
	}
	return s.Profile(ctx, userID, "")
}

// Profile membentuk profil kompetensi peserta + gap terhadap standar jabatan.
func (s *CompetencyService) Profile(ctx context.Context, userID, family string) (dto.CompetencyProfileView, error) {
	if strings.TrimSpace(family) == "" {
		family = bank.DefaultPositionFamily
	}

	competencies, err := s.repo.ListCompetencies(ctx, true)
	if err != nil {
		return dto.CompetencyProfileView{}, err
	}
	rows, err := s.repo.ListUserCompetencies(ctx, userID)
	if err != nil {
		return dto.CompetencyProfileView{}, err
	}
	scores, err := s.repo.UserSkillScores(ctx, userID)
	if err != nil {
		return dto.CompetencyProfileView{}, err
	}
	bankMap, err := s.repo.ListBankMap(ctx)
	if err != nil {
		return dto.CompetencyProfileView{}, err
	}

	byCode := make(map[string][]models.UserCompetency, len(rows))
	for _, row := range rows {
		byCode[row.CompetencyCode] = append(byCode[row.CompetencyCode], row)
	}
	tryout := competencyTryoutLevels(scores, bankMap)

	assessments := make([]dto.CompetencyAssessmentView, 0, len(competencies))
	best := make(map[string]int, len(competencies))
	for _, competency := range competencies {
		view := dto.CompetencyAssessmentView{
			Code:         competency.Code,
			Name:         competency.Name,
			Cluster:      competency.Cluster,
			ClusterLabel: models.CompetencyClusterLabel(competency.Cluster),
			Sources:      []dto.CompetencySourceView{},
		}
		top := 0
		for _, row := range byCode[competency.Code] {
			view.Sources = append(view.Sources, dto.CompetencySourceView{
				Source:      row.Source,
				SourceLabel: models.CompetencySourceLabel(row.Source),
				Level:       row.Level,
				Evidence:    row.Evidence,
				AssessedAt:  formatTime(row.AssessedAt),
			})
			if row.Level > top {
				top = row.Level
			}
		}
		if derived, ok := tryout[competency.Code]; ok {
			view.Sources = append(view.Sources, dto.CompetencySourceView{
				Source:      models.CompetencySourceTryout,
				SourceLabel: models.CompetencySourceLabel(models.CompetencySourceTryout),
				Level:       derived.Level,
				Evidence:    derived.Evidence,
			})
			if derived.Level > top {
				top = derived.Level
			}
		}
		view.BestLevel = top
		view.HasData = top > 0
		best[competency.Code] = top
		assessments = append(assessments, view)
	}

	gap, err := s.gapForFamily(ctx, family, best)
	if err != nil {
		return dto.CompetencyProfileView{}, err
	}

	return dto.CompetencyProfileView{
		Assessments:    assessments,
		Gap:            gap,
		PositionFamily: family,
		PositionLabel:  bank.PositionFamilyLabel(family),
		Notes: []string{
			"Level dari tryout adalah proksi latihan, bukan asesmen formal.",
			"Kompetensi tanpa data ditampilkan sebagai \"data belum cukup\" (level 0), bukan nilai nol.",
		},
	}, nil
}

// gapForFamily menghitung kesenjangan terhadap standar satu keluarga jabatan.
func (s *CompetencyService) gapForFamily(ctx context.Context, family string, best map[string]int) ([]dto.CompetencyGapView, error) {
	reqs, err := s.repo.ListRequirements(ctx, family, "")
	if err != nil {
		return nil, err
	}
	competencies, err := s.repo.ListCompetencies(ctx, false)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]models.Competency, len(competencies))
	for _, item := range competencies {
		byCode[item.Code] = item
	}

	out := make([]dto.CompetencyGapView, 0, len(reqs))
	for _, req := range reqs {
		have := best[req.CompetencyCode]
		gap, satisfied := competencyGap(req.MinLevel, have)
		out = append(out, dto.CompetencyGapView{
			Code:      req.CompetencyCode,
			Name:      byCode[req.CompetencyCode].Name,
			Need:      req.MinLevel,
			Have:      have,
			Gap:       gap,
			Satisfied: satisfied,
		})
	}
	return out, nil
}

/* ---------------------------- matriks admin ------------------------------- */

// Matrix membentuk matriks peserta × kompetensi beserta penanda gap.
//
// Level tryout diturunkan di memori (bukan ditulis ulang tiap permintaan) agar
// GET tidak menimbulkan efek samping; penilaian manusia (self/asesor) tetap
// dibaca dari `user_competencies`.
func (s *CompetencyService) Matrix(ctx context.Context, filter repository.MatrixFilter, family string) (dto.CompetencyMatrixView, error) {
	if strings.TrimSpace(family) == "" {
		family = bank.DefaultPositionFamily
	}

	competencies, err := s.repo.ListCompetencies(ctx, true)
	if err != nil {
		return dto.CompetencyMatrixView{}, err
	}
	users, err := s.repo.ListPesertaForMatrix(ctx, filter)
	if err != nil {
		return dto.CompetencyMatrixView{}, err
	}
	bankMap, err := s.repo.ListBankMap(ctx)
	if err != nil {
		return dto.CompetencyMatrixView{}, err
	}
	reqs, err := s.repo.ListRequirements(ctx, family, "")
	if err != nil {
		return dto.CompetencyMatrixView{}, err
	}

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	rows, err := s.repo.ListUserCompetenciesByUsers(ctx, userIDs)
	if err != nil {
		return dto.CompetencyMatrixView{}, err
	}
	scores, err := s.repo.UserSkillScoresForUsers(ctx, userIDs)
	if err != nil {
		return dto.CompetencyMatrixView{}, err
	}

	byUser := make(map[string][]models.UserCompetency, len(users))
	for _, row := range rows {
		byUser[row.UserID] = append(byUser[row.UserID], row)
	}
	scoresByUser := make(map[string][]repository.UserSkillScoreRow, len(users))
	for _, score := range scores {
		scoresByUser[score.UserID] = append(scoresByUser[score.UserID], score)
	}

	nameByCode := make(map[string]string, len(competencies))
	views := make([]dto.CompetencyView, 0, len(competencies))
	for _, item := range competencies {
		nameByCode[item.Code] = item.Name
		views = append(views, dto.CompetencyView{
			Code:         item.Code,
			Name:         item.Name,
			Cluster:      item.Cluster,
			ClusterLabel: models.CompetencyClusterLabel(item.Cluster),
			Definition:   item.Definition,
			Active:       item.Active,
		})
	}
	needByCode := make(map[string]int, len(reqs))
	for _, req := range reqs {
		needByCode[req.CompetencyCode] = req.MinLevel
	}

	outRows := make([]dto.CompetencyMatrixRowView, 0, len(users))
	for _, user := range users {
		tryout := competencyTryoutLevels(scoresByUser[user.ID], bankMap)
		cells := make(map[string]dto.CompetencyMatrixCellView, len(tryout)+len(byUser[user.ID]))
		best := make(map[string]int, len(cells))
		for code, derived := range tryout {
			cells[code] = dto.CompetencyMatrixCellView{
				Level:       derived.Level,
				Source:      models.CompetencySourceTryout,
				SourceLabel: models.CompetencySourceLabel(models.CompetencySourceTryout),
			}
			best[code] = derived.Level
		}
		for _, row := range byUser[user.ID] {
			if current, ok := cells[row.CompetencyCode]; !ok || row.Level > current.Level {
				cells[row.CompetencyCode] = dto.CompetencyMatrixCellView{
					Level:       row.Level,
					Source:      row.Source,
					SourceLabel: models.CompetencySourceLabel(row.Source),
				}
			}
			if row.Level > best[row.CompetencyCode] {
				best[row.CompetencyCode] = row.Level
			}
		}

		gaps := make([]dto.CompetencyGapView, 0, len(needByCode))
		for code, need := range needByCode {
			have := best[code]
			gap, satisfied := competencyGap(need, have)
			if satisfied {
				continue
			}
			gaps = append(gaps, dto.CompetencyGapView{
				Code: code, Name: nameByCode[code], Need: need, Have: have, Gap: gap,
			})
		}
		sort.Slice(gaps, func(i, j int) bool {
			if gaps[i].Gap == gaps[j].Gap {
				return gaps[i].Code < gaps[j].Code
			}
			return gaps[i].Gap > gaps[j].Gap
		})

		outRows = append(outRows, dto.CompetencyMatrixRowView{
			UserID:   user.ID,
			Name:     user.Name,
			Initials: competencyInitials(user.Name),
			Email:    user.Email,
			Program:  user.Program,
			Cells:    cells,
			Gap:      gaps,
			GapCount: len(gaps),
		})
	}

	return dto.CompetencyMatrixView{
		Competencies:   views,
		Rows:           outRows,
		PositionFamily: family,
		PositionLabel:  bank.PositionFamilyLabel(family),
		TotalPeserta:   len(users),
		Notes: []string{
			"Level dari tryout adalah proksi latihan, bukan asesmen formal.",
			"Kolom gap hanya menampilkan kompetensi yang belum memenuhi standar jabatan terpilih.",
			"Peserta tanpa sel apa pun berarti data belum cukup, bukan penilaian rendah.",
		},
	}, nil
}
