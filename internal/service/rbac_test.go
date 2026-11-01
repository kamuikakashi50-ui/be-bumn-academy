package service

import (
	"testing"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji matriks akses data asesmen (Fase 3 §4.6), termasuk uji negatif.
//
// Matriks ini menyangkut data pribadi spesifik (UU PDP No. 27/2022), sehingga
// setiap baris diuji — bukan hanya jalur "boleh".

const (
	ownerUser   = "user-peserta-1"
	otherUser   = "user-peserta-2"
	adminActor  = "user-admin"
	psikoActor  = "user-psikolog"
	instansiAct = "user-hr-instansi"
)

func TestCanViewTalentProfile(t *testing.T) {
	cases := []struct {
		name  string
		actor AccessActor
		want  bool
	}{
		{"peserta pemilik", AccessActor{UserID: ownerUser, Role: models.RolePeserta}, true},
		{"peserta lain ditolak", AccessActor{UserID: otherUser, Role: models.RolePeserta}, false},
		{"psikolog yang ditugaskan", AccessActor{UserID: psikoActor, Role: models.RolePsikolog, AssignedPsychologist: true}, true},
		{"psikolog di luar penugasan ditolak", AccessActor{UserID: psikoActor, Role: models.RolePsikolog}, false},
		{"admin", AccessActor{UserID: adminActor, Role: models.RoleAdmin}, true},
		{"HR instansi ditolak", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi, CohortAllowed: true}, false},
		{"publik ditolak", AccessActor{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanViewTalentProfile(tc.actor, ownerUser); got != tc.want {
				t.Errorf("CanViewTalentProfile = %v, harap %v", got, tc.want)
			}
		})
	}
}

func TestCanViewCompetency(t *testing.T) {
	cases := []struct {
		name  string
		actor AccessActor
		want  bool
	}{
		{"peserta pemilik", AccessActor{UserID: ownerUser, Role: models.RolePeserta}, true},
		{"peserta lain ditolak", AccessActor{UserID: otherUser, Role: models.RolePeserta}, false},
		{"psikolog yang ditugaskan", AccessActor{UserID: psikoActor, Role: models.RolePsikolog, AssignedPsychologist: true}, true},
		{"psikolog di luar penugasan ditolak", AccessActor{UserID: psikoActor, Role: models.RolePsikolog}, false},
		{"admin", AccessActor{UserID: adminActor, Role: models.RoleAdmin}, true},
		{"HR instansi kohort sendiri", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi, CohortAllowed: true}, true},
		{"HR instansi luar kohort ditolak", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanViewCompetency(tc.actor, ownerUser); got != tc.want {
				t.Errorf("CanViewCompetency = %v, harap %v", got, tc.want)
			}
		})
	}
}

func TestCanViewItemAnswersHanyaPsikologDitugaskan(t *testing.T) {
	// Kode Etik Psikologi: jawaban item mentah tidak dibuka ke admin/HR/peserta.
	cases := []struct {
		name  string
		actor AccessActor
		want  bool
	}{
		{"psikolog ditugaskan", AccessActor{UserID: psikoActor, Role: models.RolePsikolog, AssignedPsychologist: true}, true},
		{"psikolog di luar penugasan", AccessActor{UserID: psikoActor, Role: models.RolePsikolog}, false},
		{"admin", AccessActor{UserID: adminActor, Role: models.RoleAdmin}, false},
		{"HR instansi", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi, CohortAllowed: true}, false},
		{"peserta pemilik", AccessActor{UserID: ownerUser, Role: models.RolePeserta}, false},
		{"publik", AccessActor{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanViewItemAnswers(tc.actor); got != tc.want {
				t.Errorf("CanViewItemAnswers = %v, harap %v", got, tc.want)
			}
		})
	}
}

func TestCanViewConsultationNote(t *testing.T) {
	cases := []struct {
		name       string
		actor      AccessActor
		visibility models.NoteVisibility
		want       bool
	}{
		{"penulis selalu boleh (private)", AccessActor{UserID: psikoActor, Role: models.RolePsikolog, OwnsNote: true}, models.NotePrivate, true},
		{"penulis selalu boleh (shared_to_hr)", AccessActor{UserID: psikoActor, Role: models.RolePsikolog, OwnsNote: true}, models.NoteSharedToHR, true},
		{"psikolog ditugaskan boleh", AccessActor{UserID: psikoActor, Role: models.RolePsikolog, AssignedPsychologist: true}, models.NotePrivate, true},
		{"psikolog lain ditolak", AccessActor{UserID: psikoActor, Role: models.RolePsikolog}, models.NoteSharedToHR, false},
		{"peserta pemilik shared_to_user", AccessActor{UserID: ownerUser, Role: models.RolePeserta}, models.NoteSharedToUser, true},
		{"peserta pemilik shared_to_hr", AccessActor{UserID: ownerUser, Role: models.RolePeserta}, models.NoteSharedToHR, true},
		{"peserta pemilik private ditolak", AccessActor{UserID: ownerUser, Role: models.RolePeserta}, models.NotePrivate, false},
		{"peserta lain ditolak", AccessActor{UserID: otherUser, Role: models.RolePeserta}, models.NoteSharedToHR, false},
		{"admin selalu ditolak", AccessActor{UserID: adminActor, Role: models.RoleAdmin}, models.NoteSharedToHR, false},
		{"HR instansi shared_to_hr kohort sendiri", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi, CohortAllowed: true}, models.NoteSharedToHR, true},
		{"HR instansi shared_to_user ditolak", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi, CohortAllowed: true}, models.NoteSharedToUser, false},
		{"HR instansi private ditolak", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi, CohortAllowed: true}, models.NotePrivate, false},
		{"HR instansi luar kohort ditolak", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi}, models.NoteSharedToHR, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanViewConsultationNote(tc.actor, ownerUser, tc.visibility); got != tc.want {
				t.Errorf("CanViewConsultationNote = %v, harap %v", got, tc.want)
			}
		})
	}
}

func TestCanViewAggregate(t *testing.T) {
	cases := []struct {
		name  string
		actor AccessActor
		want  bool
	}{
		{"admin", AccessActor{UserID: adminActor, Role: models.RoleAdmin}, true},
		{"HR instansi kohort sendiri", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi, CohortAllowed: true}, true},
		{"HR instansi luar kohort ditolak", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi}, false},
		{"peserta ditolak", AccessActor{UserID: ownerUser, Role: models.RolePeserta}, false},
		{"psikolog ditolak", AccessActor{UserID: psikoActor, Role: models.RolePsikolog, AssignedPsychologist: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanViewAggregate(tc.actor); got != tc.want {
				t.Errorf("CanViewAggregate = %v, harap %v", got, tc.want)
			}
		})
	}
}

func TestAccessLogNeededMencatatAksesPihakKetiga(t *testing.T) {
	cases := []struct {
		name  string
		actor AccessActor
		want  bool
	}{
		{"peserta ke datanya sendiri tidak dicatat", AccessActor{UserID: ownerUser, Role: models.RolePeserta}, false},
		{"psikolog dicatat", AccessActor{UserID: psikoActor, Role: models.RolePsikolog, AssignedPsychologist: true}, true},
		{"admin dicatat", AccessActor{UserID: adminActor, Role: models.RoleAdmin}, true},
		{"HR instansi dicatat", AccessActor{UserID: instansiAct, Role: models.RoleAdminInstansi, CohortAllowed: true}, true},
		{"tanpa identitas tidak dicatat", AccessActor{Role: models.RoleAdmin}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AccessLogNeeded(tc.actor, ownerUser); got != tc.want {
				t.Errorf("AccessLogNeeded = %v, harap %v", got, tc.want)
			}
		})
	}
}
