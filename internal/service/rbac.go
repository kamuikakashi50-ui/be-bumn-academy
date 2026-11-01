package service

import "github.com/bumn-academy/be-bumn-academy/internal/models"

// Matriks akses data asesmen (Fase 3 §4.6).
//
// Aturan di sini **murni** (tanpa database) supaya seluruh baris matriks bisa
// diuji otomatis, termasuk uji negatif: peserta lain ditolak, psikolog di luar
// penugasan ditolak, HR tanpa `shared_to_hr` ditolak, dan jawaban item mentah
// hanya untuk psikolog.
//
// Peran saja tidak cukup: kepemilikan resource (peserta pemilik, psikolog yang
// ditugaskan, kohort milik instansi) selalu diperiksa juga (§2.8).

// AccessActor adalah pemanggil yang hendak membuka data asesmen.
type AccessActor struct {
	UserID string
	Role   models.Role
	// AssignedPsychologist menandai psikolog memang ditugaskan pada peserta ini
	// (mis. lewat pemesanan konsultasi atau penugasan instrumen berlisensi).
	AssignedPsychologist bool
	// CohortAllowed menandai peserta berada pada kohort milik instansi pemanggil.
	CohortAllowed bool
	// OwnsNote menandai pemanggil adalah penulis catatan konsultasi.
	OwnsNote bool
}

// IsOwner menandai pemanggil adalah peserta pemilik data itu sendiri.
func (a AccessActor) IsOwner(subjectUserID string) bool {
	return a.UserID != "" && a.UserID == subjectUserID
}

// CanViewTalentProfile menentukan siapa boleh melihat profil minat–bakat.
//
// Peserta pemilik ✅ · psikolog yang ditugaskan ✅ · admin ✅ · HR instansi ❌ ·
// publik ❌ (§4.6).
func CanViewTalentProfile(actor AccessActor, subjectUserID string) bool {
	switch actor.Role {
	case models.RoleAdmin:
		return true
	case models.RolePsikolog:
		return actor.AssignedPsychologist
	case models.RolePeserta:
		return actor.IsOwner(subjectUserID)
	default:
		// `admin_instansi` tidak melihat profil minat–bakat mentah.
		return false
	}
}

// CanViewCompetency menentukan siapa boleh melihat matriks/profil kompetensi.
//
// Peserta hanya dirinya sendiri; psikolog harus ditugaskan; admin penuh;
// HR instansi hanya untuk kohortnya (§4.6).
func CanViewCompetency(actor AccessActor, subjectUserID string) bool {
	switch actor.Role {
	case models.RoleAdmin:
		return true
	case models.RolePsikolog:
		return actor.AssignedPsychologist
	case models.RolePeserta:
		return actor.IsOwner(subjectUserID)
	case models.RoleAdminInstansi:
		return actor.CohortAllowed
	default:
		return false
	}
}

// CanViewItemAnswers menentukan siapa boleh membuka jawaban item mentah.
//
// Hanya psikolog yang ditugaskan. Admin & HR tidak pernah melihat jawaban mentah
// (Kode Etik Psikologi, §4.6). Peserta pemilik pun tidak dibuka lewat endpoint
// ini — ia hanya melihat skor & interpretasinya.
func CanViewItemAnswers(actor AccessActor) bool {
	return actor.Role == models.RolePsikolog && actor.AssignedPsychologist
}

// CanViewConsultationNote menentukan siapa boleh membaca satu catatan konsultasi.
//
// Penulis (psikolog) selalu ✅; peserta pemilik hanya bila catatannya
// `shared_to_user`/`shared_to_hr`; HR instansi hanya bila `shared_to_hr` **dan**
// pesertanya berada di kohortnya. Admin tidak pernah membaca isi catatan (§4.6).
func CanViewConsultationNote(
	actor AccessActor,
	subjectUserID string,
	visibility models.NoteVisibility,
) bool {
	switch actor.Role {
	case models.RolePsikolog:
		return actor.OwnsNote || actor.AssignedPsychologist
	case models.RolePeserta:
		return actor.IsOwner(subjectUserID) && sharedToUser(visibility)
	case models.RoleAdminInstansi:
		return actor.CohortAllowed && visibility == models.NoteSharedToHR
	default:
		return false
	}
}

// sharedToUser menandai catatan yang boleh dibaca peserta pemiliknya.
func sharedToUser(visibility models.NoteVisibility) bool {
	return visibility == models.NoteSharedToUser || visibility == models.NoteSharedToHR
}

// CanViewAggregate menentukan siapa boleh melihat angka agregat (n ≥ 10).
//
// Peserta & psikolog tidak; admin ✅; HR instansi ✅ untuk kohortnya; publik ✅
// lewat halaman korporasi yang sudah menyamarkan identitas (§4.6).
func CanViewAggregate(actor AccessActor) bool {
	switch actor.Role {
	case models.RoleAdmin:
		return true
	case models.RoleAdminInstansi:
		return actor.CohortAllowed
	default:
		return false
	}
}

// AccessLogNeeded menandai resource yang wajib dicatat di `assessment_access_logs`.
//
// Setiap akses ke `talent_profile`/`competency`/`consultation_note` dicatat
// (§4.6). Akses peserta ke datanya sendiri tidak dicatat agar log tetap terbaca
// sebagai jejak akses pihak ketiga.
func AccessLogNeeded(actor AccessActor, subjectUserID string) bool {
	if actor.Role == models.RolePeserta && actor.IsOwner(subjectUserID) {
		return false
	}
	return actor.UserID != ""
}
