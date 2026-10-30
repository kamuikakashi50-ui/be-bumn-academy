package dto

/* ------------------------------- sertifikat -------------------------------- */

// CertificateView adalah satu sertifikat milik peserta.
type CertificateView struct {
	ID            string `json:"id"`
	Number        string `json:"number"`
	PathID        string `json:"pathId,omitempty"`
	PathTitle     string `json:"pathTitle,omitempty"`
	SectorKey     string `json:"sectorKey,omitempty"`
	SectorLabel   string `json:"sectorLabel,omitempty"`
	TestType      string `json:"testType,omitempty"`
	TestTypeLabel string `json:"testTypeLabel,omitempty"`
	IssuedAt      string `json:"issuedAt"`
	// Status bernilai "sah" atau "dicabut".
	Status    string `json:"status"`
	RevokedAt string `json:"revokedAt,omitempty"`
	// Organization adalah instansi pada sertifikat kelas khusus (bila ada).
	Organization string `json:"organization,omitempty"`
	// DownloadURL relatif terhadap base API, siap dipakai front-end.
	DownloadURL string `json:"downloadUrl"`
	VerifyURL   string `json:"verifyUrl"`
}

// CertificateEligibilityView menjelaskan mengapa sertifikat belum bisa terbit.
type CertificateEligibilityView struct {
	Eligible bool `json:"eligible"`
	// CertificateEnabled false berarti jalur ini memang tidak menerbitkan sertifikat.
	CertificateEnabled bool `json:"certificateEnabled"`
	Issued             bool `json:"issued"`
	// MissingSteps adalah langkah wajib yang belum selesai.
	MissingSteps []CertificateBlockerView `json:"missingSteps"`
	// MissingQuizzes adalah kuis yang belum pernah lulus.
	MissingQuizzes []CertificateBlockerView `json:"missingQuizzes"`
	Certificate    *CertificateView         `json:"certificate,omitempty"`
}

// CertificateBlockerView adalah satu syarat sertifikat yang belum terpenuhi.
type CertificateBlockerView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// ProgressPercent melengkapi langkah (mis. 40% dari pelajaran modul).
	ProgressPercent int `json:"progressPercent,omitempty"`
}

// CertificateVerificationView adalah hasil verifikasi publik sebuah sertifikat.
//
// Sengaja hanya memuat data yang memang boleh publik: nama peserta, jalur,
// tanggal terbit, dan status. Tidak ada email, nilai, atau data kohort lain.
type CertificateVerificationView struct {
	Number        string `json:"number"`
	Valid         bool   `json:"valid"`
	Status        string `json:"status"`
	Participant   string `json:"participant"`
	PathTitle     string `json:"pathTitle,omitempty"`
	TestTypeLabel string `json:"testTypeLabel,omitempty"`
	SectorLabel   string `json:"sectorLabel,omitempty"`
	Organization  string `json:"organization,omitempty"`
	IssuedAt      string `json:"issuedAt"`
	RevokedAt     string `json:"revokedAt,omitempty"`
}

/* ---------------------------- sertifikat (admin) --------------------------- */

// AdminCertificateView adalah sertifikat pada panel admin (dengan identitas peserta).
type AdminCertificateView struct {
	CertificateView
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}
