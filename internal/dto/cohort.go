package dto

/* ------------------------------- instansi --------------------------------- */

// OrganizationView adalah satu instansi mitra pada panel admin.
type OrganizationView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	SectorKey   string `json:"sectorKey"`
	SectorLabel string `json:"sectorLabel,omitempty"`
	LogoURL     string `json:"logoUrl,omitempty"`
	PICName     string `json:"picName,omitempty"`
	PICEmail    string `json:"picEmail,omitempty"`
	PICPhone    string `json:"picPhone,omitempty"`
	Active      bool   `json:"active"`
	CohortCount int    `json:"cohortCount"`
}

// OrganizationInput adalah payload tambah instansi mitra.
type OrganizationInput struct {
	Name      string `json:"name" validate:"required,min=3,max=180"`
	Slug      string `json:"slug" validate:"omitempty,max=80"`
	SectorKey string `json:"sectorKey" validate:"omitempty,max=60"`
	LogoURL   string `json:"logoUrl" validate:"omitempty,max=500"`
	PICName   string `json:"picName" validate:"omitempty,max=120"`
	PICEmail  string `json:"picEmail" validate:"omitempty,email,max=180"`
	PICPhone  string `json:"picPhone" validate:"omitempty,max=40"`
	Active    *bool  `json:"active"`
}

// OrganizationPatch adalah payload ubah sebagian data instansi.
type OrganizationPatch struct {
	Name      *string `json:"name" validate:"omitempty,min=3,max=180"`
	Slug      *string `json:"slug" validate:"omitempty,max=80"`
	SectorKey *string `json:"sectorKey" validate:"omitempty,max=60"`
	LogoURL   *string `json:"logoUrl" validate:"omitempty,max=500"`
	PICName   *string `json:"picName" validate:"omitempty,max=120"`
	PICEmail  *string `json:"picEmail" validate:"omitempty,email,max=180"`
	PICPhone  *string `json:"picPhone" validate:"omitempty,max=40"`
	Active    *bool   `json:"active"`
}

/* -------------------------------- kohort ---------------------------------- */

// CohortSummaryView adalah ringkasan satu kelas khusus pada panel admin.
type CohortSummaryView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	SectorKey   string `json:"sectorKey"`
	SectorLabel string `json:"sectorLabel,omitempty"`
	Status      string `json:"status"`
	StatusLabel string `json:"statusLabel"`
	// OrgName kosong berarti kelas internal (tanpa instansi).
	OrgID   string `json:"orgId,omitempty"`
	OrgName string `json:"orgName,omitempty"`
	OrgSlug string `json:"orgSlug,omitempty"`

	StartsAt    string `json:"startsAt,omitempty"`
	EndsAt      string `json:"endsAt,omitempty"`
	AccessUntil string `json:"accessUntil,omitempty"`
	Quota       int    `json:"quota"`
	// ActiveMembers adalah anggota berstatus aktif (yang memakai jatah kuota).
	ActiveMembers   int    `json:"activeMembers"`
	MemberCount     int    `json:"memberCount"`
	MentorUserID    string `json:"mentorUserId,omitempty"`
	MentorName      string `json:"mentorName,omitempty"`
	ModuleCount     int    `json:"moduleCount"`
	RequiredModules int    `json:"requiredModuleCount"`
	InviteCount     int    `json:"inviteCount"`
	Notes           string `json:"notes,omitempty"`
	// AccessOpen menandai kelas sedang berjalan & masa aksesnya belum lewat.
	AccessOpen bool `json:"accessOpen"`
}

// CohortDetailView adalah detail satu kelas khusus (ringkasan + kurikulum).
type CohortDetailView struct {
	CohortSummaryView
	Curriculum []CohortCurriculumView `json:"curriculum"`
}

// CohortInput adalah payload tambah kelas khusus (kurikulum opsional).
type CohortInput struct {
	OrgID        string                  `json:"orgId" validate:"omitempty,uuid4"`
	Name         string                  `json:"name" validate:"required,min=3,max=180"`
	Code         string                  `json:"code" validate:"omitempty,max=40"`
	SectorKey    string                  `json:"sectorKey" validate:"omitempty,max=60"`
	Status       string                  `json:"status" validate:"omitempty,oneof=draft aktif selesai arsip"`
	StartsAt     string                  `json:"startsAt" validate:"omitempty"`
	EndsAt       string                  `json:"endsAt" validate:"omitempty"`
	Quota        int                     `json:"quota" validate:"gte=0,lte=100000"`
	AccessUntil  string                  `json:"accessUntil" validate:"omitempty"`
	MentorUserID string                  `json:"mentorUserId" validate:"omitempty,uuid4"`
	Notes        string                  `json:"notes" validate:"omitempty,max=1000"`
	Curriculum   []CohortCurriculumInput `json:"curriculum" validate:"omitempty,dive"`
}

// CohortPatch adalah payload ubah sebagian kelas khusus.
type CohortPatch struct {
	OrgID        *string                  `json:"orgId" validate:"omitempty,uuid4"`
	Name         *string                  `json:"name" validate:"omitempty,min=3,max=180"`
	Code         *string                  `json:"code" validate:"omitempty,max=40"`
	SectorKey    *string                  `json:"sectorKey" validate:"omitempty,max=60"`
	Status       *string                  `json:"status" validate:"omitempty,oneof=draft aktif selesai arsip"`
	StartsAt     *string                  `json:"startsAt" validate:"omitempty"`
	EndsAt       *string                  `json:"endsAt" validate:"omitempty"`
	Quota        *int                     `json:"quota" validate:"omitempty,gte=0,lte=100000"`
	AccessUntil  *string                  `json:"accessUntil" validate:"omitempty"`
	MentorUserID *string                  `json:"mentorUserId" validate:"omitempty,uuid4"`
	Notes        *string                  `json:"notes" validate:"omitempty,max=1000"`
	Curriculum   *[]CohortCurriculumInput `json:"curriculum" validate:"omitempty,dive"`
}

// CohortCurriculumInput adalah satu modul pada kurikulum kelas.
type CohortCurriculumInput struct {
	ModuleID  string `json:"moduleId" validate:"required,max=64"`
	Required  *bool  `json:"required"`
	SortOrder int    `json:"sortOrder" validate:"gte=0,lte=999"`
}

// CohortCurriculumView adalah satu modul kurikulum kelas beserta ringkasannya.
type CohortCurriculumView struct {
	ModuleID    string `json:"moduleId"`
	ModuleTitle string `json:"moduleTitle"`
	ProductSlug string `json:"productSlug"`
	Required    bool   `json:"required"`
	SortOrder   int    `json:"sortOrder"`
	LessonCount int    `json:"lessonCount"`
}

/* ------------------------------ keanggotaan -------------------------------- */

// CohortMemberView adalah satu anggota kelas khusus.
type CohortMemberView struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Initials string `json:"initials"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	JoinedAt string `json:"joinedAt,omitempty"`
}

// CohortMemberInput adalah payload tambah anggota kelas (satu atau massal).
//
// `userIds` dipakai bila id peserta sudah diketahui; `emails` dipakai saat admin
// menempelkan daftar email. Email yang tidak punya akun **tidak** dibuatkan akun
// baru — dilaporkan di `notFound` agar admin memutuskan sendiri.
type CohortMemberInput struct {
	UserIDs []string `json:"userIds" validate:"omitempty,dive,max=64"`
	Emails  []string `json:"emails" validate:"omitempty,dive,email,max=180"`
	Role    string   `json:"role" validate:"omitempty,oneof=peserta mentor admin_instansi"`
}

// CohortMemberPatch adalah payload ubah status keanggotaan.
type CohortMemberPatch struct {
	Status string `json:"status" validate:"required,oneof=aktif keluar lulus"`
}

// CohortMemberImportResult adalah hasil import anggota massal (Excel/CSV).
type CohortMemberImportResult struct {
	Added    int                       `json:"added"`
	Skipped  int                       `json:"skipped"`
	Failed   int                       `json:"failed"`
	Errors   []CohortMemberImportError `json:"errors,omitempty"`
	NotFound []string                  `json:"notFound,omitempty"`
}

// CohortMemberImportError adalah satu baris import yang gagal.
type CohortMemberImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

/* ------------------------------- laporan kelas ---------------------------- */

// CohortReportSummary adalah angka utama laporan satu kelas khusus.
type CohortReportSummary struct {
	TotalMembers     int `json:"totalMembers"`
	ActiveMembers    int `json:"activeMembers"`
	CompletedMembers int `json:"completedMembers"`
	LeftMembers      int `json:"leftMembers"`
	// ReportingMembers adalah anggota yang sudah pernah mengerjakan tryout.
	ReportingMembers  int     `json:"reportingMembers"`
	ParticipationRate float64 `json:"participationRate"`
	ActiveLast30Days  int     `json:"activeLast30Days"`
	ActiveWindowDays  int     `json:"activeWindowDays"`

	TotalAttempts int64   `json:"totalAttempts"`
	AverageScore  float64 `json:"averageScore"`
	TopScore      int     `json:"topScore"`
	PassingRate   float64 `json:"passingRate"`
	PracticeHours float64 `json:"practiceHours"`
	// InClassAttempts adalah percobaan yang tercatat milik kelas ini
	// (`attempts.cohort_id`), berbeda dari total percobaan anggota.
	InClassAttempts int64 `json:"inClassAttempts"`
	DistinctTryouts int64 `json:"distinctTryouts"`

	TotalLessons     int64   `json:"totalLessons"`
	CompletedLessons int64   `json:"completedLessons"`
	CompletionRate   float64 `json:"completionRate"`
	Certificates     int     `json:"certificates"`

	PassingThreshold int `json:"passingThreshold"`
}

// CohortReportPeriod adalah periode kelas yang dilaporkan.
type CohortReportPeriod struct {
	StartsAt    string `json:"startsAt,omitempty"`
	EndsAt      string `json:"endsAt,omitempty"`
	AccessUntil string `json:"accessUntil,omitempty"`
}

// CohortReportModuleView adalah capaian satu modul kurikulum kelas.
type CohortReportModuleView struct {
	ModuleID         string  `json:"moduleId"`
	ModuleTitle      string  `json:"moduleTitle"`
	ProductSlug      string  `json:"productSlug"`
	Required         bool    `json:"required"`
	SortOrder        int     `json:"sortOrder"`
	LessonCount      int     `json:"lessonCount"`
	CompletedLessons int     `json:"completedLessons"`
	CompletionRate   float64 `json:"completionRate"`
}

// CohortReportSkillView adalah rata-rata skor satu subtes (jenis tes) kelas.
type CohortReportSkillView struct {
	Name         string  `json:"name"`
	AverageScore float64 `json:"averageScore"`
	Attempts     int64   `json:"attempts"`
}

// CohortReportMemberView adalah capaian satu anggota kelas.
type CohortReportMemberView struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	Initials string `json:"initials"`
	Email    string `json:"email"`
	Program  string `json:"program"`
	// Status adalah status keanggotaan (aktif/keluar/lulus).
	Status   string `json:"status"`
	JoinedAt string `json:"joinedAt,omitempty"`

	AttemptCount int64   `json:"attemptCount"`
	AverageScore float64 `json:"averageScore"`
	BestScore    int     `json:"bestScore"`
	LastScore    *int    `json:"lastScore,omitempty"`
	LastActivity string  `json:"lastActivity,omitempty"`

	LessonsCompleted int `json:"lessonsCompleted"`
	LessonsTotal     int `json:"lessonsTotal"`
	ProgressPercent  int `json:"progressPercent"`
	Certificates     int `json:"certificates"`

	// Note menjelaskan pembacaan baris: "Prestasi", "Lancar", "Perlu pendampingan",
	// atau "Belum mulai" (dasar angka, bukan kesimpulan otomatis).
	Note string `json:"note"`
}

// CohortReportView adalah laporan lengkap satu kelas khusus.
type CohortReportView struct {
	Cohort      CohortSummaryView        `json:"cohort"`
	Period      CohortReportPeriod       `json:"period"`
	Summary     CohortReportSummary      `json:"summary"`
	Modules     []CohortReportModuleView `json:"modules"`
	Skills      []CohortReportSkillView  `json:"skills"`
	Members     []CohortReportMemberView `json:"members"`
	TopLearners []CohortReportMemberView `json:"topLearners"`
	AtRisk      []CohortReportMemberView `json:"atRisk"`
	GeneratedAt string                   `json:"generatedAt"`
	// Notes memuat catatan pembacaan laporan (mis. daftar ini bukan prediksi).
	Notes []string `json:"notes"`
}

// CohortInviteView adalah satu kode undangan kelas beserta status kelayakannya.
type CohortInviteView struct {
	Code       string `json:"code"`
	CohortID   string `json:"cohortId"`
	MaxUses    int    `json:"maxUses"`
	UsedCount  int    `json:"usedCount"`
	Remaining  int    `json:"remaining"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
	Active     bool   `json:"active"`
	Usable     bool   `json:"usable"`
	RejectNote string `json:"rejectReason,omitempty"`
}

// CohortInviteInput adalah payload pembuatan kode undangan.
//
// `code` opsional: bila kosong, server membuat kode 8 karakter (huruf besar+angka
// tanpa karakter ambigu).
type CohortInviteInput struct {
	Code      string `json:"code" validate:"omitempty,len=8,alphanum"`
	MaxUses   int    `json:"maxUses" validate:"gte=0,lte=100000"`
	ExpiresAt string `json:"expiresAt" validate:"omitempty"`
}

/* ------------------------------- jejak audit ------------------------------- */

// CohortAuditView adalah satu baris jejak audit kelas khusus.
type CohortAuditView struct {
	ID         string `json:"id"`
	CohortID   string `json:"cohortId"`
	ActorID    string `json:"actorId,omitempty"`
	ActorName  string `json:"actorName,omitempty"`
	Action     string `json:"action"`
	ActionNote string `json:"actionNote,omitempty"`
	Detail     string `json:"detail,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
}

/* --------------------------- kelas khusus (peserta) ----------------------- */

// MyCohortView adalah kelas khusus yang diikuti peserta.
type MyCohortView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Status      string `json:"status"`
	StatusLabel string `json:"statusLabel"`
	OrgName     string `json:"orgName,omitempty"`
	OrgSlug     string `json:"orgSlug,omitempty"`
	SectorKey   string `json:"sectorKey,omitempty"`
	SectorLabel string `json:"sectorLabel,omitempty"`
	MentorName  string `json:"mentorName,omitempty"`

	StartsAt    string `json:"startsAt,omitempty"`
	EndsAt      string `json:"endsAt,omitempty"`
	AccessUntil string `json:"accessUntil,omitempty"`
	// AccessOpen menandai materi kelas masih bisa dibuka (kelas berjalan & masa
	// akses belum lewat).
	AccessOpen bool                   `json:"accessOpen"`
	Curriculum []CohortCurriculumView `json:"curriculum"`
}

// JoinCohortRequest adalah payload gabung kelas dengan kode undangan.
type JoinCohortRequest struct {
	Code string `json:"code" validate:"required,len=8"`
}

// JoinCohortResult adalah hasil gabung kelas.
type JoinCohortResult struct {
	Cohort MyCohortView `json:"cohort"`
	// AlreadyMember menandai peserta sudah menjadi anggota (idempoten: kuota kode
	// tidak ikut bertambah).
	AlreadyMember bool `json:"alreadyMember"`
}
