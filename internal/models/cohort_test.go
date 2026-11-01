package models

import (
	"testing"
	"time"
)

// Uji logika murni model Fase 1 langkah 4: status kelas, kuota, masa akses,
// keanggotaan, kode undangan, dan isolasi soal per instansi. Semuanya perhitungan
// tanpa database supaya aturan bisnis bisa diuji cepat.

func TestCohortStatusValidDanLabel(t *testing.T) {
	cases := []struct {
		status CohortStatus
		label  string
		valid  bool
	}{
		{CohortDraft, "Draf", true},
		{CohortAktif, "Aktif", true},
		{CohortSelesai, "Selesai", true},
		{CohortArsip, "Arsip", true},
		{CohortStatus("berjalan"), "berjalan", false},
		{CohortStatus(""), "", false},
	}

	for _, tc := range cases {
		if got := tc.status.Valid(); got != tc.valid {
			t.Errorf("Valid(%q) = %v, harap %v", tc.status, got, tc.valid)
		}
		if got := tc.status.Label(); got != tc.label {
			t.Errorf("Label(%q) = %q, harap %q", tc.status, got, tc.label)
		}
	}
}

func TestCohortAcceptsMembersMenghormatiKuota(t *testing.T) {
	// Kuota 0 = tanpa batas, jadi berapa pun anggota aktifnya tetap menerima.
	unlimited := Cohort{Quota: 0}
	for _, active := range []int{0, 5, 500} {
		if !unlimited.AcceptsMembers(active) {
			t.Errorf("kelas tanpa kuota harus menerima anggota (aktif=%d)", active)
		}
	}

	limited := Cohort{Quota: 3}
	if !limited.AcceptsMembers(2) {
		t.Error("kelas berkuota 3 masih menerima anggota saat baru 2 aktif")
	}
	if limited.AcceptsMembers(3) {
		t.Error("kelas berkuota 3 harus menolak saat anggota aktif sudah 3")
	}
	if limited.AcceptsMembers(4) {
		t.Error("kelas berkuota 3 harus menolak saat anggota aktif melebihi kuota")
	}
}

func TestCohortActiveAtMemeriksaStatusDanPeriode(t *testing.T) {
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	start := now.Add(-24 * time.Hour)
	end := now.Add(24 * time.Hour)

	cases := []struct {
		name   string
		cohort Cohort
		want   bool
	}{
		{"kelas aktif tanpa periode", Cohort{Status: CohortAktif}, true},
		{"kelas aktif dalam periode", Cohort{Status: CohortAktif, StartsAt: &start, EndsAt: &end}, true},
		{"kelas aktif sebelum mulai", Cohort{Status: CohortAktif, StartsAt: &end}, false},
		{"kelas aktif setelah berakhir", Cohort{Status: CohortAktif, EndsAt: &start}, false},
		{"kelas draf", Cohort{Status: CohortDraft}, false},
		{"kelas selesai", Cohort{Status: CohortSelesai, StartsAt: &start, EndsAt: &end}, false},
		{"kelas arsip", Cohort{Status: CohortArsip}, false},
	}

	for _, tc := range cases {
		if got := tc.cohort.ActiveAt(now); got != tc.want {
			t.Errorf("%s: ActiveAt = %v, harap %v", tc.name, got, tc.want)
		}
	}
}

func TestCohortAccessExpired(t *testing.T) {
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	if (Cohort{}).AccessExpired(now) {
		t.Error("kelas tanpa access_until tidak pernah kedaluwarsa")
	}
	if !(Cohort{AccessUntil: &past}).AccessExpired(now) {
		t.Error("masa akses yang sudah lewat harus dianggap kedaluwarsa")
	}
	if (Cohort{AccessUntil: &future}).AccessExpired(now) {
		t.Error("masa akses yang belum lewat belum kedaluwarsa")
	}
}

func TestCohortMemberStatus(t *testing.T) {
	if !CohortMemberAktif.Valid() || !CohortMemberKeluar.Valid() || !CohortMemberLulus.Valid() {
		t.Error("status keanggotaan yang dikenal harus valid")
	}
	if CohortMemberStatus("ditangguhkan").Valid() {
		t.Error("status keanggotaan tak dikenal tidak boleh valid")
	}

	// Hanya anggota aktif yang memakai jatah kuota kelas.
	if !CohortMemberAktif.CountsTowardsQuota() {
		t.Error("anggota aktif harus dihitung pada kuota")
	}
	if CohortMemberLulus.CountsTowardsQuota() || CohortMemberKeluar.CountsTowardsQuota() {
		t.Error("anggota lulus/keluar tidak boleh dihitung pada kuota")
	}

	if !CohortRolePeserta.Valid() || !CohortRoleMentor.Valid() || !CohortRoleAdminInstansi.Valid() {
		t.Error("peran anggota yang dikenal harus valid")
	}
	if CohortMemberRole("pengawas").Valid() {
		t.Error("peran anggota tak dikenal tidak boleh valid")
	}
}

func TestCohortInviteRejectReason(t *testing.T) {
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name   string
		invite CohortInvite
		reason string
	}{
		{"kode tanpa batas", CohortInvite{Active: true}, ""},
		{"kode berkuota masih sisa", CohortInvite{Active: true, MaxUses: 3, UsedCount: 2}, ""},
		{"kode belum kedaluwarsa", CohortInvite{Active: true, ExpiresAt: &future}, ""},
		{"kode dicabut", CohortInvite{Active: false}, "Kode undangan sudah dicabut."},
		{"kode kedaluwarsa", CohortInvite{Active: true, ExpiresAt: &past}, "Kode undangan sudah kedaluwarsa."},
		{"kuota kode habis", CohortInvite{Active: true, MaxUses: 2, UsedCount: 2}, "Kuota kode undangan sudah habis."},
	}

	for _, tc := range cases {
		if got := tc.invite.RejectReason(now); got != tc.reason {
			t.Errorf("%s: RejectReason = %q, harap %q", tc.name, got, tc.reason)
		}
		if usable := tc.invite.UsableAt(now); usable != (tc.reason == "") {
			t.Errorf("%s: UsableAt = %v, harap %v", tc.name, usable, tc.reason == "")
		}
	}
}

func TestQuestionVisibleToOrgsDanFilter(t *testing.T) {
	orgA := "org-a"
	orgB := "org-b"
	empty := ""

	questions := []Question{
		{ID: "soal-bersama", OwnerOrgID: nil},
		{ID: "soal-org-a", OwnerOrgID: &orgA},
		{ID: "soal-org-b", OwnerOrgID: &orgB},
		{ID: "soal-tanpa-pemilik", OwnerOrgID: &empty},
	}

	if !questions[0].VisibleToOrgs(nil) || !questions[0].VisibleToOrgs([]string{orgA}) {
		t.Error("soal bersama harus terlihat oleh semua konteks instansi")
	}
	if !questions[1].VisibleToOrgs([]string{orgA}) {
		t.Error("soal milik org A harus terlihat oleh org A")
	}
	if questions[1].VisibleToOrgs([]string{orgB}) {
		t.Error("soal milik org A tidak boleh terlihat oleh org B")
	}
	if questions[1].VisibleToOrgs(nil) {
		t.Error("soal ber-pemilik tidak boleh terlihat tanpa konteks instansi (fail-closed)")
	}
	if !questions[3].VisibleToOrgs(nil) {
		t.Error("owner_org_id kosong diperlakukan sebagai soal bersama")
	}

	visible := FilterQuestionsForOrgs(questions, []string{orgA})
	if len(visible) != 3 {
		t.Fatalf("jumlah soal untuk org A = %d, harap 3", len(visible))
	}
	if visible[0].ID != "soal-bersama" || visible[1].ID != "soal-org-a" || visible[2].ID != "soal-tanpa-pemilik" {
		t.Errorf("urutan soal harus dipertahankan, dapat %v", []string{visible[0].ID, visible[1].ID, visible[2].ID})
	}

	// Peserta yang mengikuti kelas dua instansi berhak memakai soal keduanya.
	both := FilterQuestionsForOrgs(questions, []string{orgA, orgB})
	if len(both) != 4 {
		t.Fatalf("jumlah soal untuk org A+B = %d, harap 4", len(both))
	}

	// Tanpa konteks instansi hanya soal bersama yang boleh dipakai.
	shared := FilterQuestionsForOrgs(questions, nil)
	if len(shared) != 2 {
		t.Fatalf("jumlah soal bersama = %d, harap 2", len(shared))
	}
	if shared[0].ID != "soal-bersama" || shared[1].ID != "soal-tanpa-pemilik" {
		t.Errorf("soal bersama tidak sesuai: %v", []string{shared[0].ID, shared[1].ID})
	}

	if got := FilterQuestionsForOrgs(nil, []string{orgA}); len(got) != 0 {
		t.Errorf("daftar soal kosong harus tetap kosong, dapat %d", len(got))
	}
}
