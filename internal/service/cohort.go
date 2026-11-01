package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/config"
	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/repository"
)

// inviteAlphabet adalah karakter kode undangan: huruf besar + angka tanpa
// karakter ambigu (`0`/`O`, `1`/`I`) supaya kode tidak salah dibaca peserta.
//
// Panjangnya 32 (2^5) sehingga pemetaan byte acak ke karakter tetap seragam.
const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// inviteCodeLength adalah panjang kode undangan (aturan §4.4).
const inviteCodeLength = 8

// CohortService menjalankan kelas khusus (kohort) per instansi: instansi mitra,
// kelas, keanggotaan, kurikulum, kode undangan, jejak audit, dan sisi peserta
// (gabung kelas dengan kode undangan).
//
// Aturan bisnis yang dipegang service ini:
//   - kode undangan menegakkan pencabutan, kedaluwarsa, dan kuota (§4.4);
//   - peserta yang sudah menjadi anggota tidak menambah `used_count` (idempoten);
//   - kuota kelas dihitung dari anggota **aktif** saja;
//   - setiap perubahan penting dicatat pada jejak audit kelas.
type CohortService struct {
	cohorts   *repository.CohortRepository
	users     *repository.UserRepository
	classroom *repository.ClassroomRepository
	cfg       *config.Config
}

// NewCohortService membentuk service kelas khusus.
func NewCohortService(
	cohorts *repository.CohortRepository,
	users *repository.UserRepository,
	classroom *repository.ClassroomRepository,
	cfg *config.Config,
) *CohortService {
	return &CohortService{cohorts: cohorts, users: users, classroom: classroom, cfg: cfg}
}

/* ---------------------------- helper murni -------------------------------- */

// generateInviteCode menghasilkan kode undangan acak 8 karakter.
//
// Sumber acaknya disuntikkan (io.Reader) agar bisa diuji deterministik.
func generateInviteCode(source io.Reader) (string, error) {
	buf := make([]byte, inviteCodeLength)
	if _, err := io.ReadFull(source, buf); err != nil {
		return "", fmt.Errorf("service: gagal membuat kode undangan: %w", err)
	}

	out := make([]byte, inviteCodeLength)
	for i, b := range buf {
		out[i] = inviteAlphabet[int(b)%len(inviteAlphabet)]
	}
	return string(out), nil
}

// newInviteCode menghasilkan kode undangan memakai sumber acak kriptografis.
func newInviteCode() (string, error) { return generateInviteCode(rand.Reader) }

// validateInviteCodeFormat memeriksa bentuk kode undangan yang dikirim klien.
func validateInviteCodeFormat(code string) error {
	if len(code) != inviteCodeLength {
		return apperror.Validation(
			fmt.Sprintf("Kode undangan harus %d karakter.", inviteCodeLength), nil)
	}
	for _, char := range code {
		if !strings.ContainsRune(inviteAlphabet, char) {
			return apperror.Validation("Kode undangan hanya berisi huruf besar dan angka (tanpa 0/O dan 1/I).", nil)
		}
	}
	return nil
}

// inviteRejectError memetakan alasan kode undangan ditolak ke kode galat §5.6.
//
// Kuota kode yang habis adalah konflik keadaan (bukan kesalahan pengetikan),
// sedangkan kode salah/dicabut/kedaluwarsa adalah kesalahan validasi.
func inviteRejectError(reason string) error {
	if reason == "" {
		return nil
	}
	if strings.Contains(strings.ToLower(reason), "kuota") {
		return apperror.Conflict(reason)
	}
	return apperror.Validation(reason, nil)
}

// joinabilityError memeriksa kelas masih boleh diikuti peserta baru.
//
// `activeMembers` adalah jumlah anggota berstatus aktif (kuota 0 = tanpa batas).
func joinabilityError(cohort models.Cohort, activeMembers int, now time.Time) error {
	switch cohort.Status {
	case models.CohortDraft:
		return apperror.Validation("Kelas ini belum dibuka untuk peserta.", nil)
	case models.CohortArsip:
		return apperror.Validation("Kelas ini sudah diarsipkan.", nil)
	case models.CohortSelesai:
		return apperror.Validation("Kelas ini sudah selesai.", nil)
	}
	if !cohort.ActiveAt(now) {
		return apperror.Validation("Kelas ini belum dimulai atau sudah berakhir.", nil)
	}
	if !cohort.AcceptsMembers(activeMembers) {
		return apperror.Conflict("Kuota kelas ini sudah penuh.")
	}
	return nil
}

// slugify mengubah nama menjadi slug yang aman dipakai pada URL.
func slugify(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

// cohortCodeFromName menyusun kode internal kelas dari namanya (huruf besar).
//
// Kode ini adalah label laporan, bukan kode undangan, sehingga cukup unik &
// mudah dibaca manusia.
func cohortCodeFromName(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range strings.ToUpper(strings.TrimSpace(name)) {
		switch {
		case char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	code := strings.Trim(builder.String(), "-")
	if len(code) > 40 {
		code = strings.Trim(code[:40], "-")
	}
	if code == "" {
		code = "KELAS"
	}
	return code
}

// normalizeEmails merapikan daftar email: trim, huruf kecil, tanpa duplikat.
func normalizeEmails(emails []string) []string {
	out := make([]string, 0, len(emails))
	seen := map[string]struct{}{}
	for _, email := range emails {
		value := strings.ToLower(strings.TrimSpace(email))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// memberRoleFromInput memetakan peran dari input admin (bawaan: peserta).
func memberRoleFromInput(raw string) (models.CohortMemberRole, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return models.CohortRolePeserta, nil
	}
	role := models.CohortMemberRole(value)
	if !role.Valid() {
		return "", apperror.Validation("Peran anggota tidak dikenal: "+raw, nil)
	}
	return role, nil
}

// memberStatusFromInput memetakan status keanggotaan dari input admin.
func memberStatusFromInput(raw string) (models.CohortMemberStatus, error) {
	status := models.CohortMemberStatus(strings.TrimSpace(strings.ToLower(raw)))
	if !status.Valid() {
		return "", apperror.Validation("Status anggota tidak dikenal: "+raw, nil)
	}
	return status, nil
}

// cohortStatusFromInput memetakan status kelas dari input admin.
func cohortStatusFromInput(raw string) (models.CohortStatus, error) {
	status := models.CohortStatus(strings.TrimSpace(strings.ToLower(raw)))
	if !status.Valid() {
		return "", apperror.Validation("Status kelas tidak dikenal: "+raw, nil)
	}
	return status, nil
}

// validEmail memeriksa bentuk alamat email (dipakai import anggota massal).
//
// Selain sintaks RFC, domain wajib memuat titik: alamat seperti `a@b` sah secara
// sintaks tetapi hampir pasti salah ketik pada daftar peserta.
func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return false
	}
	at := strings.LastIndex(value, "@")
	if at < 0 {
		return false
	}
	return strings.Contains(value[at+1:], ".")
}

// cohortActionNote menjelaskan aksi jejak audit untuk ditampilkan di UI.
func cohortActionNote(action string) string {
	switch action {
	case models.CohortActionCreated:
		return "Kelas khusus dibuat"
	case models.CohortActionInviteCreated:
		return "Kode undangan dibuat"
	case models.CohortActionInviteRevoked:
		return "Kode undangan dicabut"
	case models.CohortActionInviteUsed:
		return "Kode undangan dipakai"
	case models.CohortActionMemberAdded:
		return "Anggota ditambahkan"
	case models.CohortActionMemberRemoved:
		return "Anggota dikeluarkan"
	case models.CohortActionMemberStatus:
		return "Status anggota diubah"
	case models.CohortActionCurriculumSet:
		return "Kurikulum kelas diperbarui"
	case models.CohortActionStatusChanged:
		return "Status kelas diubah"
	default:
		return action
	}
}

/* ------------------------------- instansi --------------------------------- */

// Organizations mengembalikan daftar instansi mitra beserta jumlah kelasnya.
func (s *CohortService) Organizations(ctx context.Context, filter repository.OrganizationFilter) ([]dto.OrganizationView, error) {
	rows, err := s.cohorts.ListOrganizations(ctx, filter)
	if err != nil {
		return nil, err
	}

	counts, err := s.cohorts.CohortCountsByOrg(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]dto.OrganizationView, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.organizationView(row, counts[row.ID]))
	}
	return out, nil
}

// CreateOrganization menambah instansi mitra.
func (s *CohortService) CreateOrganization(ctx context.Context, req dto.OrganizationInput) (*dto.OrganizationView, error) {
	slug := slugify(req.Slug)
	if slug == "" {
		slug = slugify(req.Name)
	}
	if slug == "" {
		return nil, apperror.Validation("Slug instansi tidak bisa dibentuk dari nama.", nil)
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	org := models.Organization{
		ID:        models.NewID(),
		Name:      strings.TrimSpace(req.Name),
		Slug:      slug,
		SectorKey: strings.TrimSpace(req.SectorKey),
		PICName:   strings.TrimSpace(req.PICName),
		PICEmail:  strings.TrimSpace(req.PICEmail),
		PICPhone:  strings.TrimSpace(req.PICPhone),
		Active:    active,
	}
	if org.SectorKey == "" {
		org.SectorKey = "umum"
	}
	if !bank.IsValidSector(org.SectorKey) {
		return nil, apperror.Validation("Klaster BUMN tidak dikenal: "+org.SectorKey, nil)
	}
	if logo := strings.TrimSpace(req.LogoURL); logo != "" {
		org.LogoURL = &logo
	}

	if err := s.cohorts.UpsertOrganization(ctx, &org); err != nil {
		return nil, err
	}
	view := s.organizationView(org, 0)
	return &view, nil
}

// UpdateOrganization mengubah sebagian data instansi (logo, PIC, status).
func (s *CohortService) UpdateOrganization(ctx context.Context, id string, req dto.OrganizationPatch) (*dto.OrganizationView, error) {
	org, err := s.cohorts.FindOrganization(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Slug != nil {
		slug := slugify(*req.Slug)
		if slug == "" {
			return nil, apperror.Validation("Slug instansi tidak boleh kosong.", nil)
		}
		updates["slug"] = slug
	}
	if req.SectorKey != nil {
		sector := strings.TrimSpace(*req.SectorKey)
		if !bank.IsValidSector(sector) {
			return nil, apperror.Validation("Klaster BUMN tidak dikenal: "+sector, nil)
		}
		updates["sector_key"] = sector
	}
	if req.LogoURL != nil {
		updates["logo_url"] = strings.TrimSpace(*req.LogoURL)
	}
	if req.PICName != nil {
		updates["pic_name"] = strings.TrimSpace(*req.PICName)
	}
	if req.PICEmail != nil {
		updates["pic_email"] = strings.TrimSpace(*req.PICEmail)
	}
	if req.PICPhone != nil {
		updates["pic_phone"] = strings.TrimSpace(*req.PICPhone)
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}

	if err := s.cohorts.UpdateOrganizationFields(ctx, org.ID, updates); err != nil {
		return nil, err
	}

	updated, err := s.cohorts.FindOrganization(ctx, org.ID)
	if err != nil {
		return nil, err
	}
	counts, err := s.cohorts.CohortCountsByOrg(ctx)
	if err != nil {
		return nil, err
	}
	view := s.organizationView(*updated, counts[updated.ID])
	return &view, nil
}

// organizationView menyusun tampilan instansi.
func (s *CohortService) organizationView(org models.Organization, cohortCount int) dto.OrganizationView {
	return dto.OrganizationView{
		ID:          org.ID,
		Name:        org.Name,
		Slug:        org.Slug,
		SectorKey:   org.SectorKey,
		SectorLabel: bank.SectorLabel(org.SectorKey),
		LogoURL:     strPtrOrEmpty(org.LogoURL),
		PICName:     org.PICName,
		PICEmail:    org.PICEmail,
		PICPhone:    org.PICPhone,
		Active:      org.Active,
		CohortCount: cohortCount,
	}
}

/* -------------------------------- kohort ---------------------------------- */

// cohortMaxModules membatasi jumlah modul dalam satu kurikulum kelas.
const cohortMaxModules = 50

// AdminCohorts mengembalikan daftar kelas khusus untuk panel admin.
func (s *CohortService) AdminCohorts(ctx context.Context, filter repository.CohortFilter) ([]dto.CohortSummaryView, error) {
	rows, err := s.cohorts.ListCohorts(ctx, filter)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(rows))
	mentorIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		if row.MentorUserID != nil {
			mentorIDs = append(mentorIDs, *row.MentorUserID)
		}
	}

	counts, err := s.cohortCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	mentors, err := s.users.ListByIDs(ctx, mentorIDs)
	if err != nil {
		return nil, err
	}

	out := make([]dto.CohortSummaryView, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.cohortSummaryView(row, counts[row.ID], mentorName(row, mentors)))
	}
	return out, nil
}

// AdminCohort mengembalikan detail satu kelas beserta kurikulumnya.
func (s *CohortService) AdminCohort(ctx context.Context, id string) (*dto.CohortDetailView, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}

	counts, err := s.cohortCounts(ctx, []string{cohort.ID})
	if err != nil {
		return nil, err
	}
	mentors, err := s.users.ListByIDs(ctx, mentorIDsOf(*cohort))
	if err != nil {
		return nil, err
	}
	curriculum, err := s.curriculumViewsFor(ctx, cohort.ID)
	if err != nil {
		return nil, err
	}

	return &dto.CohortDetailView{
		CohortSummaryView: s.cohortSummaryView(*cohort, counts[cohort.ID], mentorName(*cohort, mentors)),
		Curriculum:        curriculum,
	}, nil
}

// CreateCohort menambah kelas khusus beserta kurikulum awalnya.
func (s *CohortService) CreateCohort(ctx context.Context, actorID string, req dto.CohortInput) (*dto.CohortDetailView, error) {
	cohort, err := s.buildCohort(ctx, req)
	if err != nil {
		return nil, err
	}
	cohort.ID = models.NewID()

	// Kode kelas adalah label laporan yang unik: bentrok harus ditolak dengan pesan
	// jelas, bukan gagal di tingkat database.
	if existing, err := s.cohorts.FindCohortByCode(ctx, cohort.Code); err == nil && existing != nil {
		return nil, apperror.Conflict("Kode kelas sudah dipakai kelas lain.")
	} else if err != nil && !isNotFound(err) {
		return nil, err
	}

	if err := s.cohorts.UpsertCohort(ctx, cohort); err != nil {
		return nil, err
	}

	if len(req.Curriculum) > 0 {
		items, err := s.curriculumItems(ctx, cohort.ID, req.Curriculum)
		if err != nil {
			return nil, err
		}
		if err := s.cohorts.ReplaceCohortCurriculum(ctx, cohort.ID, items); err != nil {
			return nil, err
		}
	}

	if err := s.audit(ctx, cohort.ID, actorID, models.CohortActionCreated, cohort.Name); err != nil {
		return nil, err
	}
	return s.AdminCohort(ctx, cohort.ID)
}

// UpdateCohort mengubah sebagian data kelas khusus (termasuk status & kurikulum).
func (s *CohortService) UpdateCohort(ctx context.Context, actorID, id string, req dto.CohortPatch) (*dto.CohortDetailView, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if err := s.applyCohortPatch(ctx, cohort, req, updates); err != nil {
		return nil, err
	}
	if err := s.validateCohortPeriod(cohort, updates); err != nil {
		return nil, err
	}
	if err := s.cohorts.UpdateCohortFields(ctx, cohort.ID, updates); err != nil {
		return nil, err
	}

	if req.Status != nil {
		status, err := cohortStatusFromInput(*req.Status)
		if err != nil {
			return nil, err
		}
		if status != cohort.Status {
			detail := fmt.Sprintf("%s → %s", cohort.Status, status)
			if err := s.audit(ctx, cohort.ID, actorID, models.CohortActionStatusChanged, detail); err != nil {
				return nil, err
			}
		}
	}

	if req.Curriculum != nil {
		items, err := s.curriculumItems(ctx, cohort.ID, *req.Curriculum)
		if err != nil {
			return nil, err
		}
		if err := s.cohorts.ReplaceCohortCurriculum(ctx, cohort.ID, items); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, cohort.ID, actorID, models.CohortActionCurriculumSet, fmt.Sprintf("%d modul", len(items))); err != nil {
			return nil, err
		}
	}

	return s.AdminCohort(ctx, cohort.ID)
}

// SetCohortCurriculum mengganti seluruh kurikulum sebuah kelas.
func (s *CohortService) SetCohortCurriculum(ctx context.Context, actorID, id string, req []dto.CohortCurriculumInput) (*dto.CohortDetailView, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}

	items, err := s.curriculumItems(ctx, cohort.ID, req)
	if err != nil {
		return nil, err
	}
	if err := s.cohorts.ReplaceCohortCurriculum(ctx, cohort.ID, items); err != nil {
		return nil, err
	}
	if err := s.audit(ctx, cohort.ID, actorID, models.CohortActionCurriculumSet, fmt.Sprintf("%d modul", len(items))); err != nil {
		return nil, err
	}
	return s.AdminCohort(ctx, cohort.ID)
}

// applyCohortPatch menyalin field yang dikirim admin ke peta perubahan.
func (s *CohortService) applyCohortPatch(
	ctx context.Context,
	cohort *models.Cohort,
	req dto.CohortPatch,
	updates map[string]any,
) error {
	if req.OrgID != nil {
		orgID := strings.TrimSpace(*req.OrgID)
		switch {
		case orgID == "":
			updates["org_id"] = nil
		default:
			if _, err := s.cohorts.FindOrganization(ctx, orgID); err != nil {
				return err
			}
			updates["org_id"] = orgID
		}
	}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Code != nil {
		code := strings.TrimSpace(*req.Code)
		if code == "" {
			return apperror.Validation("Kode kelas tidak boleh kosong.", nil)
		}
		if existing, err := s.cohorts.FindCohortByCode(ctx, code); err == nil && existing != nil {
			if existing.ID != cohort.ID {
				return apperror.Conflict("Kode kelas sudah dipakai kelas lain.")
			}
		} else if err != nil && !isNotFound(err) {
			return err
		}
		updates["code"] = code
	}
	if req.SectorKey != nil {
		sector := strings.TrimSpace(*req.SectorKey)
		if !bank.IsValidSector(sector) {
			return apperror.Validation("Klaster BUMN tidak dikenal: "+sector, nil)
		}
		updates["sector_key"] = sector
	}
	if req.Status != nil {
		status, err := cohortStatusFromInput(*req.Status)
		if err != nil {
			return err
		}
		updates["status"] = status
	}
	for field, raw := range map[string]*string{
		"starts_at":    req.StartsAt,
		"ends_at":      req.EndsAt,
		"access_until": req.AccessUntil,
	} {
		if raw == nil {
			continue
		}
		value, err := parseOptionalTime(*raw)
		if err != nil {
			return err
		}
		updates[field] = value
	}
	if req.Quota != nil {
		updates["quota"] = *req.Quota
	}
	if req.MentorUserID != nil {
		mentorID := strings.TrimSpace(*req.MentorUserID)
		if mentorID == "" {
			updates["mentor_user_id"] = nil
		} else {
			if _, err := s.users.FindByID(ctx, mentorID); err != nil {
				return err
			}
			updates["mentor_user_id"] = mentorID
		}
	}
	if req.Notes != nil {
		updates["notes"] = strings.TrimSpace(*req.Notes)
	}
	return nil
}

/* ------------------------ tampilan & bantuan kohort ----------------------- */

// cohortCountRow merangkum jumlah anggota/kurikulum/undangan satu kelas.
type cohortCountRow struct {
	Members    repository.CohortMemberCount
	Curriculum repository.CohortCurriculumCount
	Invites    int
}

// cohortCounts menghitung anggota, modul kurikulum, dan kode undangan beberapa
// kelas sekaligus (tiga query, bukan tiga per kelas).
func (s *CohortService) cohortCounts(ctx context.Context, ids []string) (map[string]cohortCountRow, error) {
	out := map[string]cohortCountRow{}
	if len(ids) == 0 {
		return out, nil
	}

	members, err := s.cohorts.CohortMemberCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	curriculum, err := s.cohorts.CohortCurriculumCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	invites, err := s.cohorts.CohortInviteCounts(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		out[id] = cohortCountRow{
			Members:    members[id],
			Curriculum: curriculum[id],
			Invites:    invites[id],
		}
	}
	return out, nil
}

// mentorIDsOf mengumpulkan id mentor sebuah kelas (0 atau 1 nilai).
func mentorIDsOf(cohort models.Cohort) []string {
	if cohort.MentorUserID == nil || *cohort.MentorUserID == "" {
		return nil
	}
	return []string{*cohort.MentorUserID}
}

// mentorName mengembalikan nama mentor kelas (kosong bila tidak ada).
func mentorName(cohort models.Cohort, mentors map[string]models.User) string {
	if cohort.MentorUserID == nil {
		return ""
	}
	if user, ok := mentors[*cohort.MentorUserID]; ok {
		return user.Name
	}
	return ""
}

// cohortSummaryView menyusun ringkasan kelas untuk panel admin.
func (s *CohortService) cohortSummaryView(
	cohort models.Cohort,
	counts cohortCountRow,
	mentor string,
) dto.CohortSummaryView {
	now := time.Now().UTC()
	view := dto.CohortSummaryView{
		ID:              cohort.ID,
		Name:            cohort.Name,
		Code:            cohort.Code,
		SectorKey:       cohort.SectorKey,
		SectorLabel:     bank.SectorLabel(cohort.SectorKey),
		Status:          string(cohort.Status),
		StatusLabel:     cohort.Status.Label(),
		StartsAt:        formatTimePtr(cohort.StartsAt),
		EndsAt:          formatTimePtr(cohort.EndsAt),
		AccessUntil:     formatTimePtr(cohort.AccessUntil),
		Quota:           cohort.Quota,
		ActiveMembers:   counts.Members.Active,
		MemberCount:     counts.Members.Total,
		MentorUserID:    strPtrOrEmpty(cohort.MentorUserID),
		MentorName:      mentor,
		ModuleCount:     counts.Curriculum.Total,
		RequiredModules: counts.Curriculum.Required,
		InviteCount:     counts.Invites,
		Notes:           cohort.Notes,
		AccessOpen:      cohort.ActiveAt(now) && !cohort.AccessExpired(now),
	}
	if cohort.Organization != nil {
		view.OrgID = cohort.Organization.ID
		view.OrgName = cohort.Organization.Name
		view.OrgSlug = cohort.Organization.Slug
	}
	return view
}

// curriculumViews menyusun tampilan kurikulum kelas (beserta jumlah materinya).
func (s *CohortService) curriculumViews(items []models.CohortCurriculumItem) []dto.CohortCurriculumView {
	out := make([]dto.CohortCurriculumView, 0, len(items))
	for _, item := range items {
		view := dto.CohortCurriculumView{
			ModuleID:  item.ModuleID,
			Required:  item.Required,
			SortOrder: item.SortOrder,
		}
		if item.Module != nil {
			view.ModuleTitle = item.Module.Title
			view.ProductSlug = item.Module.ProductSlug
			view.LessonCount = len(item.Module.Lessons)
		}
		out = append(out, view)
	}
	return out
}

// curriculumItems memvalidasi daftar modul kurikulum kelas.
//
// Modul wajib ada di ruang kelas dan tidak boleh tercantum dua kali; urutan yang
// tidak dikirim admin diisi mengikuti posisi daftar.
func (s *CohortService) curriculumItems(
	ctx context.Context,
	cohortID string,
	inputs []dto.CohortCurriculumInput,
) ([]models.CohortCurriculumItem, error) {
	if len(inputs) > cohortMaxModules {
		return nil, apperror.Validation(
			fmt.Sprintf("Kurikulum kelas maksimal %d modul.", cohortMaxModules), nil)
	}

	// Validasi bentuk daftar lebih dulu (tanpa menyentuh database): id kosong dan
	// modul ganda ditolak sebelum query apa pun dijalankan.
	seen := map[string]struct{}{}
	for _, input := range inputs {
		moduleID := strings.TrimSpace(input.ModuleID)
		if moduleID == "" {
			return nil, apperror.Validation("Setiap modul kurikulum harus punya id modul.", nil)
		}
		if _, ok := seen[moduleID]; ok {
			return nil, apperror.Validation("Modul "+moduleID+" tercantum lebih dari sekali di kurikulum.", nil)
		}
		seen[moduleID] = struct{}{}
	}

	items := make([]models.CohortCurriculumItem, 0, len(inputs))
	for index, input := range inputs {
		moduleID := strings.TrimSpace(input.ModuleID)
		if _, err := s.classroom.FindModule(ctx, moduleID); err != nil {
			if isNotFound(err) {
				return nil, apperror.Validation("Modul "+moduleID+" tidak ditemukan di ruang kelas.", nil)
			}
			return nil, err
		}

		required := true
		if input.Required != nil {
			required = *input.Required
		}
		sortOrder := input.SortOrder
		if sortOrder == 0 {
			sortOrder = index
		}
		items = append(items, models.CohortCurriculumItem{
			CohortID:  cohortID,
			ModuleID:  moduleID,
			Required:  required,
			SortOrder: sortOrder,
		})
	}
	return items, nil
}

// audit mencatat satu aksi kelas pada jejak audit.
func (s *CohortService) audit(ctx context.Context, cohortID, actorID, action, detail string) error {
	log := models.CohortAuditLog{CohortID: cohortID, Action: action, Detail: detail}
	if actorID != "" {
		log.ActorID = &actorID
	}
	return s.cohorts.AppendCohortAudit(ctx, &log)
}

// isNotFound memakai pemeriksa bersama di auth.go (kode apperror NOT_FOUND).

/* --------------------------- validasi kelas baru -------------------------- */

// buildCohort menyusun kelas baru dari payload admin (belum disimpan).
func (s *CohortService) buildCohort(ctx context.Context, req dto.CohortInput) (*models.Cohort, error) {
	cohort := &models.Cohort{
		Name:      strings.TrimSpace(req.Name),
		Code:      strings.TrimSpace(req.Code),
		SectorKey: strings.TrimSpace(req.SectorKey),
		Quota:     req.Quota,
		Notes:     strings.TrimSpace(req.Notes),
	}
	if cohort.Code == "" {
		cohort.Code = cohortCodeFromName(cohort.Name)
	}
	if cohort.SectorKey == "" {
		cohort.SectorKey = "umum"
	}
	if !bank.IsValidSector(cohort.SectorKey) {
		return nil, apperror.Validation("Klaster BUMN tidak dikenal: "+cohort.SectorKey, nil)
	}

	status := models.CohortDraft
	if strings.TrimSpace(req.Status) != "" {
		parsed, err := cohortStatusFromInput(req.Status)
		if err != nil {
			return nil, err
		}
		status = parsed
	}
	cohort.Status = status

	if orgID := strings.TrimSpace(req.OrgID); orgID != "" {
		org, err := s.cohorts.FindOrganization(ctx, orgID)
		if err != nil {
			return nil, err
		}
		cohort.OrgID = &org.ID
	}

	starts, err := parseOptionalTime(req.StartsAt)
	if err != nil {
		return nil, err
	}
	ends, err := parseOptionalTime(req.EndsAt)
	if err != nil {
		return nil, err
	}
	accessUntil, err := parseOptionalTime(req.AccessUntil)
	if err != nil {
		return nil, err
	}
	cohort.StartsAt, cohort.EndsAt, cohort.AccessUntil = starts, ends, accessUntil
	if err := s.validateCohortPeriod(&models.Cohort{
		StartsAt:    starts,
		EndsAt:      ends,
		AccessUntil: accessUntil,
	}, nil); err != nil {
		return nil, err
	}

	if mentorID := strings.TrimSpace(req.MentorUserID); mentorID != "" {
		if _, err := s.users.FindByID(ctx, mentorID); err != nil {
			return nil, err
		}
		cohort.MentorUserID = &mentorID
	}
	return cohort, nil
}

// validateCohortPeriod memeriksa urutan tanggal kelas: tanggal berakhir dan akhir
// masa akses tidak boleh mendahului tanggal mulai.
//
// Nilai yang sedang diubah dibaca dari `updates` supaya pemeriksaan tetap benar
// pada PATCH sebagian (mis. hanya mengubah `ends_at`).
func (s *CohortService) validateCohortPeriod(cohort *models.Cohort, updates map[string]any) error {
	starts, ends, accessUntil := cohort.StartsAt, cohort.EndsAt, cohort.AccessUntil
	if updates != nil {
		if value, ok := updates["starts_at"]; ok {
			starts = asTimePtr(value)
		}
		if value, ok := updates["ends_at"]; ok {
			ends = asTimePtr(value)
		}
		if value, ok := updates["access_until"]; ok {
			accessUntil = asTimePtr(value)
		}
	}

	if starts != nil && ends != nil && ends.Before(*starts) {
		return apperror.Validation("Tanggal berakhir tidak boleh mendahului tanggal mulai.", nil)
	}
	if starts != nil && accessUntil != nil && accessUntil.Before(*starts) {
		return apperror.Validation("Akhir masa akses tidak boleh mendahului tanggal mulai.", nil)
	}
	return nil
}

// asTimePtr membaca nilai waktu dari peta perubahan (nil berarti dihapus).
func asTimePtr(value any) *time.Time {
	switch typed := value.(type) {
	case *time.Time:
		return typed
	case time.Time:
		return &typed
	default:
		return nil
	}
}

/* ------------------------------ keanggotaan -------------------------------- */

// CohortMembers mengembalikan anggota sebuah kelas (beserta identitasnya).
func (s *CohortService) CohortMembers(ctx context.Context, id string) ([]dto.CohortMemberView, error) {
	if _, err := s.cohorts.FindCohort(ctx, id); err != nil {
		return nil, err
	}

	rows, err := s.cohorts.ListCohortMembers(ctx, id)
	if err != nil {
		return nil, err
	}

	out := make([]dto.CohortMemberView, 0, len(rows))
	for _, row := range rows {
		out = append(out, memberView(row))
	}
	return out, nil
}

// memberView menyusun tampilan satu anggota kelas.
func memberView(row repository.CohortMemberRow) dto.CohortMemberView {
	user := models.User{Name: row.Name, Email: row.Email}
	return dto.CohortMemberView{
		UserID:   row.UserID,
		Name:     row.Name,
		Email:    row.Email,
		Initials: user.Initials(),
		Role:     row.Role,
		Status:   row.Status,
		JoinedAt: formatTime(row.JoinedAt),
	}
}

// AddCohortMembers menambahkan anggota kelas lewat daftar id atau email.
//
// Email yang belum punya akun **tidak** dibuatkan akun baru (keputusan produk:
// penambahan akun hanya lewat alur pendaftaran); email itu dilaporkan pada
// `notFound` agar admin memutuskan sendiri.
func (s *CohortService) AddCohortMembers(
	ctx context.Context,
	actorID, id string,
	req dto.CohortMemberInput,
) (*dto.CohortMemberImportResult, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}
	role, err := memberRoleFromInput(req.Role)
	if err != nil {
		return nil, err
	}
	return s.addMembers(ctx, actorID, *cohort, role, req.UserIDs, req.Emails)
}

// addMembers adalah inti penambahan anggota (dipakai endpoint JSON & import berkas).
func (s *CohortService) addMembers(
	ctx context.Context,
	actorID string,
	cohort models.Cohort,
	role models.CohortMemberRole,
	userIDs, emails []string,
) (*dto.CohortMemberImportResult, error) {
	result := dto.CohortMemberImportResult{}
	if len(userIDs) == 0 && len(emails) == 0 {
		return nil, apperror.Validation("Daftar anggota (userIds atau emails) wajib diisi.", nil)
	}

	found := map[string]models.User{}
	if len(userIDs) > 0 {
		byID, err := s.users.ListByIDs(ctx, userIDs)
		if err != nil {
			return nil, err
		}
		for _, userID := range userIDs {
			user, ok := byID[userID]
			if !ok {
				result.NotFound = append(result.NotFound, userID)
				continue
			}
			found[user.ID] = user
		}
	}
	if len(emails) > 0 {
		normalized := normalizeEmails(emails)
		byEmail, err := s.users.FindByEmails(ctx, normalized)
		if err != nil {
			return nil, err
		}
		for _, email := range normalized {
			user, ok := byEmail[email]
			if !ok {
				result.NotFound = append(result.NotFound, email)
				continue
			}
			found[user.ID] = user
		}
	}
	if len(found) == 0 {
		return nil, apperror.Validation("Tidak ada akun peserta yang cocok dengan daftar yang dikirim.", nil)
	}

	// Anggota yang sudah ada tidak dihitung dua kali pada kuota.
	existing, err := s.cohorts.ListCohortMembers(ctx, cohort.ID)
	if err != nil {
		return nil, err
	}
	existingIDs := make(map[string]struct{}, len(existing))
	for _, row := range existing {
		existingIDs[row.UserID] = struct{}{}
	}

	now := time.Now().UTC()
	members := make([]models.CohortMember, 0, len(found))
	for _, user := range found {
		if _, ok := existingIDs[user.ID]; ok {
			result.Skipped++
		} else {
			result.Added++
		}
		members = append(members, models.CohortMember{
			CohortID: cohort.ID,
			UserID:   user.ID,
			Role:     role,
			Status:   models.CohortMemberAktif,
			JoinedAt: now,
		})
	}

	if result.Added > 0 && cohort.Quota > 0 {
		activeMembers, err := s.cohorts.CountCohortMembers(ctx, cohort.ID, models.CohortMemberAktif)
		if err != nil {
			return nil, err
		}
		if activeMembers+result.Added > cohort.Quota {
			return nil, apperror.Conflict(fmt.Sprintf(
				"Kuota kelas tersisa %d, sedangkan %d peserta baru dikirim.",
				cohort.Quota-activeMembers, result.Added))
		}
	}

	if err := s.cohorts.UpsertCohortMembers(ctx, members); err != nil {
		return nil, err
	}

	detail := fmt.Sprintf("%d anggota baru, %d sudah terdaftar", result.Added, result.Skipped)
	if err := s.audit(ctx, cohort.ID, actorID, models.CohortActionMemberAdded, detail); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateCohortMember mengubah status keanggotaan (mis. menandai lulus/keluar).
func (s *CohortService) UpdateCohortMember(
	ctx context.Context,
	actorID, id, userID string,
	req dto.CohortMemberPatch,
) ([]dto.CohortMemberView, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}
	status, err := memberStatusFromInput(req.Status)
	if err != nil {
		return nil, err
	}

	if err := s.cohorts.SetMemberStatus(ctx, cohort.ID, userID, status); err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("%s → %s", userID, status)
	if err := s.audit(ctx, cohort.ID, actorID, models.CohortActionMemberStatus, detail); err != nil {
		return nil, err
	}
	return s.CohortMembers(ctx, cohort.ID)
}

// RemoveCohortMember mengeluarkan anggota dari kelas.
func (s *CohortService) RemoveCohortMember(ctx context.Context, actorID, id, userID string) ([]dto.CohortMemberView, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.cohorts.RemoveCohortMember(ctx, cohort.ID, userID); err != nil {
		return nil, err
	}
	if err := s.audit(ctx, cohort.ID, actorID, models.CohortActionMemberRemoved, userID); err != nil {
		return nil, err
	}
	return s.CohortMembers(ctx, cohort.ID)
}

// ImportCohortMembers menambahkan anggota dari berkas Excel/CSV.
//
// Kolom yang dibaca: `email` (wajib) dan `nama` (opsional, hanya untuk laporan
// baris). Baris tanpa header juga diterima (kolom pertama = email).
func (s *CohortService) ImportCohortMembers(
	ctx context.Context,
	actorID, id, filename string,
	data []byte,
) (*dto.CohortMemberImportResult, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}

	emails, rowErrors, err := parseMemberRows(filename, data)
	if err != nil {
		return nil, err
	}
	if len(emails) == 0 {
		return nil, apperror.Validation("Berkas tidak memuat baris email yang bisa dibaca.", nil)
	}

	result, err := s.addMembers(ctx, actorID, *cohort, models.CohortRolePeserta, nil, emails)
	if err != nil {
		return nil, err
	}
	result.Errors = rowErrors
	result.Failed = len(rowErrors)
	return result, nil
}

// parseMemberRows membaca berkas anggota massal dan mengembalikan email yang sah
// beserta daftar baris yang gagal.
//
// Baris pertama dianggap header bila memuat kolom "email"; kalau tidak, seluruh
// baris dibaca sebagai data dengan kolom pertama = email.
func parseMemberRows(filename string, data []byte) ([]string, []dto.CohortMemberImportError, error) {
	rows, err := readTabularRows(filename, data)
	if err != nil {
		return nil, nil, err
	}

	header := map[string]int{}
	if len(rows) > 0 {
		header = headerIndex(rows[0])
	}
	emailColumn, hasHeader := header["email"], false
	if hasHeader = emailColumn >= 0; !hasHeader {
		emailColumn = 0
	}

	emails := make([]string, 0, len(rows))
	failures := make([]dto.CohortMemberImportError, 0)
	for index, row := range rows {
		if hasHeader && index == 0 {
			continue
		}
		if isBlankRow(row) {
			continue
		}
		// Nomor baris mengikuti berkas (header = baris 1).
		line := index + 1
		value := ""
		if emailColumn < len(row) {
			value = strings.ToLower(strings.TrimSpace(row[emailColumn]))
		}
		if value == "" {
			failures = append(failures, dto.CohortMemberImportError{Row: line, Message: "email kosong"})
			continue
		}
		if !validEmail(value) {
			failures = append(failures, dto.CohortMemberImportError{Row: line, Message: "email tidak valid: " + value})
			continue
		}
		emails = append(emails, value)
	}
	return emails, failures, nil
}

// headerIndex memetakan nama kolom (huruf kecil) ke posisinya.
//
// Mengembalikan `email: -1` bila berkas tidak memakai header.
func headerIndex(row []string) map[string]int {
	out := map[string]int{"email": -1, "nama": -1}
	for index, cell := range row {
		key := strings.ToLower(strings.TrimSpace(cell))
		switch key {
		case "email", "e-mail", "surel", "alamat email":
			out["email"] = index
		case "nama", "name", "nama peserta":
			out["nama"] = index
		}
	}
	return out
}

/* ------------------------------ kode undangan ------------------------------ */

// CohortInvites mengembalikan kode undangan sebuah kelas beserta kelayakannya.
func (s *CohortService) CohortInvites(ctx context.Context, id string) ([]dto.CohortInviteView, error) {
	if _, err := s.cohorts.FindCohort(ctx, id); err != nil {
		return nil, err
	}

	rows, err := s.cohorts.ListCohortInvites(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	out := make([]dto.CohortInviteView, 0, len(rows))
	for _, row := range rows {
		out = append(out, inviteView(row, now))
	}
	return out, nil
}

// inviteView menyusun tampilan kode undangan (termasuk alasan tidak bisa dipakai).
func inviteView(invite models.CohortInvite, now time.Time) dto.CohortInviteView {
	view := dto.CohortInviteView{
		Code:       invite.Code,
		CohortID:   invite.CohortID,
		MaxUses:    invite.MaxUses,
		UsedCount:  invite.UsedCount,
		ExpiresAt:  formatTimePtr(invite.ExpiresAt),
		Active:     invite.Active,
		Usable:     invite.UsableAt(now),
		RejectNote: invite.RejectReason(now),
	}
	if invite.MaxUses > 0 {
		view.Remaining = max(0, invite.MaxUses-invite.UsedCount)
	}
	return view
}

// CreateCohortInvite membuat kode undangan untuk sebuah kelas.
//
// Kode boleh dikirim admin (mis. kode khusus satu batch); bila kosong, server
// membuat kode acak 8 karakter.
func (s *CohortService) CreateCohortInvite(
	ctx context.Context,
	actorID, id string,
	req dto.CohortInviteInput,
) (*dto.CohortInviteView, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}

	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		code, err = newInviteCode()
		if err != nil {
			return nil, apperror.Internal(err, "Gagal membuat kode undangan.")
		}
	}
	if err := validateInviteCodeFormat(code); err != nil {
		return nil, err
	}

	expiresAt, err := parseOptionalTime(req.ExpiresAt)
	if err != nil {
		return nil, err
	}

	if _, err := s.cohorts.FindInvite(ctx, code); err == nil {
		return nil, apperror.Conflict("Kode undangan sudah dipakai.")
	} else if !isNotFound(err) {
		return nil, err
	}

	invite := models.CohortInvite{
		Code:      code,
		CohortID:  cohort.ID,
		MaxUses:   req.MaxUses,
		ExpiresAt: expiresAt,
		Active:    true,
	}
	if actorID != "" {
		invite.CreatedBy = &actorID
	}
	if err := s.cohorts.UpsertInvite(ctx, &invite); err != nil {
		return nil, err
	}
	if err := s.audit(ctx, cohort.ID, actorID, models.CohortActionInviteCreated, code); err != nil {
		return nil, err
	}

	view := inviteView(invite, time.Now().UTC())
	return &view, nil
}

// RevokeCohortInvite mencabut kode undangan (kode langsung tidak bisa dipakai).
func (s *CohortService) RevokeCohortInvite(ctx context.Context, actorID, id, code string) (*dto.CohortInviteView, error) {
	cohort, err := s.cohorts.FindCohort(ctx, id)
	if err != nil {
		return nil, err
	}

	invite, err := s.cohorts.FindInvite(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	if invite.CohortID != cohort.ID {
		return nil, apperror.NotFound("Kode undangan tidak ditemukan pada kelas ini.")
	}
	if err := s.cohorts.SetInviteActive(ctx, invite.Code, false); err != nil {
		return nil, err
	}
	if err := s.audit(ctx, cohort.ID, actorID, models.CohortActionInviteRevoked, invite.Code); err != nil {
		return nil, err
	}

	invite.Active = false
	view := inviteView(*invite, time.Now().UTC())
	return &view, nil
}

/* ------------------------------- jejak audit ------------------------------- */

// CohortAudit mengembalikan jejak audit sebuah kelas (terbaru lebih dulu).
func (s *CohortService) CohortAudit(ctx context.Context, id string, limit int) ([]dto.CohortAuditView, error) {
	if _, err := s.cohorts.FindCohort(ctx, id); err != nil {
		return nil, err
	}

	rows, err := s.cohorts.ListCohortAudit(ctx, id, limit)
	if err != nil {
		return nil, err
	}

	actorIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ActorID != nil {
			actorIDs = append(actorIDs, *row.ActorID)
		}
	}
	actors, err := s.users.ListByIDs(ctx, actorIDs)
	if err != nil {
		return nil, err
	}

	out := make([]dto.CohortAuditView, 0, len(rows))
	for _, row := range rows {
		view := dto.CohortAuditView{
			ID:         row.ID,
			CohortID:   row.CohortID,
			ActorID:    strPtrOrEmpty(row.ActorID),
			Action:     row.Action,
			ActionNote: cohortActionNote(row.Action),
			Detail:     row.Detail,
			CreatedAt:  formatTime(row.CreatedAt),
		}
		if row.ActorID != nil {
			if actor, ok := actors[*row.ActorID]; ok {
				view.ActorName = actor.Name
			}
		}
		out = append(out, view)
	}
	return out, nil
}

/* --------------------------- kelas khusus (peserta) ----------------------- */

// Join menggabungkan peserta ke sebuah kelas memakai kode undangan.
//
// Aturan yang ditegakkan (bagian 4.4):
//   - kode salah/dicabut/kedaluwarsa → VALIDATION_ERROR;
//   - kuota kode atau kuota kelas penuh → CONFLICT;
//   - peserta yang sudah menjadi anggota → CONFLICT (dan `used_count` kode tidak
//     bertambah, sehingga satu akun tidak menghabiskan dua jatah);
//   - anggota yang pernah keluar/lulus bisa diaktifkan kembali lewat kode yang sama.
func (s *CohortService) Join(ctx context.Context, userID string, req dto.JoinCohortRequest) (*dto.JoinCohortResult, error) {
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if err := validateInviteCodeFormat(code); err != nil {
		return nil, err
	}

	invite, err := s.cohorts.FindInvite(ctx, code)
	if err != nil {
		if isNotFound(err) {
			return nil, apperror.Validation("Kode undangan tidak dikenal.", nil)
		}
		return nil, err
	}

	now := time.Now().UTC()
	if err := inviteRejectError(invite.RejectReason(now)); err != nil {
		return nil, err
	}
	if invite.Cohort == nil {
		return nil, apperror.NotFound("Kelas khusus untuk kode ini tidak ditemukan.")
	}
	cohort := *invite.Cohort

	activeMembers, err := s.cohorts.CountCohortMembers(ctx, cohort.ID, models.CohortMemberAktif)
	if err != nil {
		return nil, err
	}
	if err := joinabilityError(cohort, activeMembers, now); err != nil {
		return nil, err
	}

	member := models.CohortMember{
		CohortID: cohort.ID,
		UserID:   userID,
		Role:     models.CohortRolePeserta,
		Status:   models.CohortMemberAktif,
		JoinedAt: now,
	}
	params := repository.JoinParams{Member: member}
	alreadyMember := false

	if existing, err := s.cohorts.FindCohortMember(ctx, cohort.ID, userID); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.Status == models.CohortMemberAktif {
			return nil, apperror.Conflict("Kamu sudah menjadi anggota kelas ini.")
		}
		// Pernah keluar/lulus: diaktifkan kembali tanpa memakai jatah kode undangan.
		alreadyMember = true
		params.Audit = &models.CohortAuditLog{
			CohortID: cohort.ID,
			ActorID:  &userID,
			Action:   models.CohortActionMemberAdded,
			Detail:   "diaktifkan kembali lewat kode " + code,
		}
	} else {
		params.InviteCode = code
		params.Audit = &models.CohortAuditLog{
			CohortID: cohort.ID,
			ActorID:  &userID,
			Action:   models.CohortActionInviteUsed,
			Detail:   code,
		}
	}

	if err := s.cohorts.JoinCohort(ctx, params); err != nil {
		return nil, err
	}

	view, err := s.myCohortView(ctx, cohort)
	if err != nil {
		return nil, err
	}
	return &dto.JoinCohortResult{Cohort: *view, AlreadyMember: alreadyMember}, nil
}

// MyCohorts mengembalikan kelas khusus yang sedang diikuti peserta.
func (s *CohortService) MyCohorts(ctx context.Context, userID string) ([]dto.MyCohortView, error) {
	rows, err := s.cohorts.ActiveCohortsForUser(ctx, userID, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	out := make([]dto.MyCohortView, 0, len(rows))
	for _, row := range rows {
		view, err := s.myCohortView(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, *view)
	}
	return out, nil
}

// myCohortView menyusun tampilan satu kelas untuk peserta.
func (s *CohortService) myCohortView(ctx context.Context, cohort models.Cohort) (*dto.MyCohortView, error) {
	curriculum, err := s.curriculumViewsFor(ctx, cohort.ID)
	if err != nil {
		return nil, err
	}
	mentors, err := s.users.ListByIDs(ctx, mentorIDsOf(cohort))
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	view := dto.MyCohortView{
		ID:          cohort.ID,
		Name:        cohort.Name,
		Code:        cohort.Code,
		Status:      string(cohort.Status),
		StatusLabel: cohort.Status.Label(),
		SectorKey:   cohort.SectorKey,
		SectorLabel: bank.SectorLabel(cohort.SectorKey),
		MentorName:  mentorName(cohort, mentors),
		StartsAt:    formatTimePtr(cohort.StartsAt),
		EndsAt:      formatTimePtr(cohort.EndsAt),
		AccessUntil: formatTimePtr(cohort.AccessUntil),
		AccessOpen:  cohort.ActiveAt(now) && !cohort.AccessExpired(now),
		Curriculum:  curriculum,
	}
	if cohort.Organization != nil {
		view.OrgName = cohort.Organization.Name
		view.OrgSlug = cohort.Organization.Slug
	}
	return &view, nil
}
