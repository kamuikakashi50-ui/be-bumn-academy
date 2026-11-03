package service

import (
	"strings"
	"testing"
	"time"

	"github.com/bumn-academy/be-bumn-academy/internal/models"
)

// Uji gerbang persetujuan (consent) & penarikan persetujuan (Fase 3 §2.9, §4.7).
//
// Yang dijaga di sini: pemrosesan data psikologis butuh persetujuan eksplisit,
// dan penarikan persetujuan **menghentikan pemrosesan baru** tanpa menghapus
// riwayat yang wajib disimpan.

func consentRecord(revoked bool) *models.ConsentRecord {
	item := &models.ConsentRecord{
		IDBase:      models.IDBase{ID: "consent-1"},
		UserID:      "user-1",
		Purpose:     models.ConsentPurposeAssessment,
		TextVersion: "2026-11-v1",
		GrantedAt:   time.Now().UTC(),
	}
	if revoked {
		at := time.Now().UTC()
		item.RevokedAt = &at
	}
	return item
}

func TestConsentPurposeValid(t *testing.T) {
	for _, purpose := range models.ConsentPurposes {
		if !models.ConsentPurposeValid(purpose) {
			t.Errorf("tujuan %q harap dikenali", purpose)
		}
	}
	// Tujuan tak dikenal (termasuk beda huruf besar/kecil atau ada spasi) ditolak
	// supaya peserta tidak bisa "menarik" persetujuan yang tidak pernah ada.
	for _, purpose := range []string{"", "kepribadian", "ASSESSMENT", "assessment "} {
		if models.ConsentPurposeValid(purpose) {
			t.Errorf("tujuan %q harap ditolak", purpose)
		}
	}
}

func TestConsentPurposeLabel(t *testing.T) {
	for _, purpose := range models.ConsentPurposes {
		label := models.ConsentPurposeLabel(purpose)
		if label == "" || label == purpose {
			t.Errorf("label tujuan %q harap deskriptif, dapat %q", purpose, label)
		}
	}
	if got := models.ConsentPurposeLabel("tak-dikenal"); got != "tak-dikenal" {
		t.Errorf("tujuan tak dikenal harap dikembalikan apa adanya, dapat %q", got)
	}
}

func TestAssessmentConsentGate(t *testing.T) {
	cases := []struct {
		name     string
		existing *models.ConsentRecord
		granted  bool
		wantOK   bool
	}{
		{"tanpa persetujuan & tidak menyetujui ditolak", nil, false, false},
		{"tanpa persetujuan tapi menyetujui diterima", nil, true, true},
		{"persetujuan aktif diterima", consentRecord(false), false, true},
		{"persetujuan aktif walau menyetujui ulang diterima", consentRecord(false), true, true},
		{"persetujuan ditarik & tidak menyetujui ditolak", consentRecord(true), false, false},
		{"persetujuan ditarik tapi menyetujui ulang diterima", consentRecord(true), true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason := assessmentConsentRejection(tc.existing, tc.granted)
			if tc.wantOK && reason != "" {
				t.Errorf("harap diterima, tapi ditolak: %s", reason)
			}
			if !tc.wantOK && reason == "" {
				t.Error("harap ditolak, tapi diterima")
			}
		})
	}
}

func TestConsultationConsentGate(t *testing.T) {
	cases := []struct {
		name    string
		latest  *models.ConsentRecord
		granted bool
		wantOK  bool
	}{
		{"tanpa persetujuan & tidak menyetujui ditolak", nil, false, false},
		{"tanpa persetujuan tapi menyetujui diterima", nil, true, true},
		{"persetujuan aktif diterima", consentRecord(false), false, true},
		{"persetujuan ditarik & tidak menyetujui ditolak", consentRecord(true), false, false},
		{"persetujuan ditarik tapi menyetujui ulang diterima", consentRecord(true), true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason := consultationConsentRejection(tc.latest, tc.granted)
			if tc.wantOK && reason != "" {
				t.Errorf("harap diterima, tapi ditolak: %s", reason)
			}
			if !tc.wantOK && reason == "" {
				t.Error("harap ditolak, tapi diterima")
			}
		})
	}
}

// Pesan penolakan menyebut persetujuan supaya UI bisa menampilkan cara memberi
// persetujuan (§5.5).
func TestConsentRejectionMenunjukPersetujuan(t *testing.T) {
	if got := assessmentConsentRejection(nil, false); !strings.Contains(got, "Persetujuan") {
		t.Errorf("pesan asesmen = %q, harap menyebut persetujuan", got)
	}
	if got := consultationConsentRejection(nil, false); !strings.Contains(got, "Persetujuan") {
		t.Errorf("pesan konsultasi = %q, harap menyebut persetujuan", got)
	}
}
