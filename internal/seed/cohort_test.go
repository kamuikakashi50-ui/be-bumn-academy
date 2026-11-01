package seed

import (
	"strings"
	"testing"
)

// Uji kontrak data kelas khusus demo: kurikulum harus menunjuk modul ruang kelas
// yang benar-benar ada, keanggotaan harus menunjuk akun demo yang benar-benar
// dibuat, dan kode undangannya mengikuti format yang bisa dipakai peserta.
//
// Tanpa uji ini, salah ketik id modul hanya terlihat sebagai peringatan di log saat
// seed dijalankan (atau sebagai foreign key error di lingkungan bersih).

func TestKelasDemoMengacuDataSeedYangAda(t *testing.T) {
	data, err := loadDataset()
	if err != nil {
		t.Fatalf("loadDataset error: %v", err)
	}
	classroom, err := loadClassroom()
	if err != nil {
		t.Fatalf("loadClassroom error: %v", err)
	}

	modules := map[string]bool{}
	for _, module := range classroom.Modules {
		modules[module.ID] = true
	}

	accounts := map[string]bool{}
	for _, user := range data.Users {
		accounts[user.Email] = true
	}
	for _, user := range hrCohortUsers {
		accounts[user.Email] = true
	}

	if len(cohortDemoDefs) == 0 {
		t.Fatal("kelas khusus demo kosong")
	}

	seenCohort := map[string]bool{}
	seenOrgSlug := map[string]bool{}
	seenInvite := map[string]bool{}

	for _, def := range cohortDemoDefs {
		if def.CohortID == "" || def.CohortCode == "" || def.CohortName == "" {
			t.Errorf("kelas %q belum lengkap: id/kode/nama wajib diisi", def.CohortID)
		}
		if seenCohort[def.CohortID] {
			t.Errorf("id kelas %s muncul lebih dari sekali", def.CohortID)
		}
		seenCohort[def.CohortID] = true
		if seenOrgSlug[def.OrgSlug] {
			t.Errorf("slug instansi %s muncul lebih dari sekali", def.OrgSlug)
		}
		seenOrgSlug[def.OrgSlug] = true

		if !accounts[def.MentorEmail] {
			t.Errorf("mentor kelas %s memakai akun yang tidak dibuat seed: %s", def.CohortID, def.MentorEmail)
		}
		if len(def.MemberEmails) == 0 {
			t.Errorf("kelas %s tidak punya anggota demo", def.CohortID)
		}
		for _, email := range def.MemberEmails {
			if !accounts[email] {
				t.Errorf("anggota kelas %s memakai akun yang tidak dibuat seed: %s", def.CohortID, email)
			}
		}

		required := 0
		for _, item := range def.Curriculum {
			if !modules[item.ModuleID] {
				t.Errorf("kurikulum kelas %s memakai modul yang tidak ada di classroom.json: %s",
					def.CohortID, item.ModuleID)
			}
			if item.Required {
				required++
			}
		}
		if len(def.Curriculum) == 0 {
			t.Errorf("kelas %s tidak punya kurikulum", def.CohortID)
		}
		if required == 0 {
			t.Errorf("kelas %s tidak punya modul wajib — syarat sertifikat & laporan jadi kosong", def.CohortID)
		}

		if def.InviteCode != "" {
			if seenInvite[def.InviteCode] {
				t.Errorf("kode undangan %s dipakai lebih dari sekali", def.InviteCode)
			}
			seenInvite[def.InviteCode] = true
		}
	}
}

func TestKodeUndanganKelasDemoMengikutiFormat(t *testing.T) {
	// Format §4.4: 8 karakter huruf besar + angka, tanpa karakter ambigu
	// (0/O dan 1/I) supaya tidak salah dibaca peserta.
	const ambiguous = "01OI"

	for _, def := range cohortDemoDefs {
		if def.InviteCode == "" {
			continue
		}
		if len(def.InviteCode) != 8 {
			t.Errorf("kode undangan %q panjangnya %d, harap 8", def.InviteCode, len(def.InviteCode))
		}
		if def.InviteCode != strings.ToUpper(def.InviteCode) {
			t.Errorf("kode undangan %q harus huruf besar", def.InviteCode)
		}
		for _, char := range def.InviteCode {
			if strings.ContainsRune(ambiguous, char) {
				t.Errorf("kode undangan %q memuat karakter ambigu %q", def.InviteCode, char)
			}
			if !strings.ContainsRune("ABCDEFGHJKLMNPQRSTUVWXYZ23456789", char) {
				t.Errorf("kode undangan %q memuat karakter di luar huruf besar/angka", def.InviteCode)
			}
		}
	}
}
