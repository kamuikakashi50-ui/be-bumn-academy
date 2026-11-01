package models

import "testing"

// Uji logika murni KPI kelulusan (Fase 2): status yang dihitung sebagai kelulusan,
// tingkat kepercayaan laporan, dan klasifikasi bukti dokumen. Semua keputusan ini
// memengaruhi angka KPI, jadi diuji tanpa database.

func TestOutcomeStatusMenentukanKelulusanDanPenyebut(t *testing.T) {
	cases := []struct {
		status  OutcomeStatus
		passed  bool
		counted bool
		valid   bool
	}{
		{status: OutcomeLolos, passed: true, counted: true, valid: true},
		{status: OutcomeGagal, passed: false, counted: true, valid: true},
		// `menunggu` & `mengundurkan_diri` bukan kelulusan dan bukan penyebut:
		// memasukkan keduanya menekan/menaikkan angka KPI secara palsu (§4.3).
		{status: OutcomeMenunggu, passed: false, counted: false, valid: true},
		{status: OutcomeMengundurkanDiri, passed: false, counted: false, valid: true},
		{status: OutcomeStatus("diterima"), passed: false, counted: false, valid: false},
	}

	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			if got := tc.status.Passed(); got != tc.passed {
				t.Errorf("Passed() = %v, harap %v", got, tc.passed)
			}
			if got := tc.status.Counted(); got != tc.counted {
				t.Errorf("Counted() = %v, harap %v", got, tc.counted)
			}
			if got := tc.status.Valid(); got != tc.valid {
				t.Errorf("Valid() = %v, harap %v", got, tc.valid)
			}
		})
	}
}

func TestOutcomeStatusSelaluPunyaLabel(t *testing.T) {
	for _, status := range []OutcomeStatus{OutcomeLolos, OutcomeGagal, OutcomeMenunggu, OutcomeMengundurkanDiri} {
		if status.Label() == "" || status.Label() == string(status) {
			t.Errorf("status %q belum punya label yang bisa dibaca pengguna", status)
		}
	}
}

func TestOutcomeSourceMemisahkanTingkatVerifikasi(t *testing.T) {
	cases := []struct {
		source   OutcomeSource
		verified bool
		label    string
	}{
		{source: OutcomeSelfReported, verified: false, label: "Dilaporkan sendiri"},
		{source: OutcomeAdminVerified, verified: true, label: "Terverifikasi"},
		{source: OutcomeDocumentVerified, verified: true, label: "Terverifikasi (dokumen)"},
	}

	for _, tc := range cases {
		t.Run(string(tc.source), func(t *testing.T) {
			if got := tc.source.Verified(); got != tc.verified {
				t.Errorf("Verified() = %v, harap %v", got, tc.verified)
			}
			if got := tc.source.Label(); got != tc.label {
				t.Errorf("Label() = %q, harap %q", got, tc.label)
			}
			if !tc.source.Valid() {
				t.Errorf("sumber %q harus dianggap valid", tc.source)
			}
		})
	}

	if OutcomeSource("klaim_peserta").Valid() {
		t.Error("sumber tak dikenal tidak boleh dianggap valid")
	}
}

func TestOutcomeVerifiedHanyaSaatDiperiksaAdmin(t *testing.T) {
	self := SelectionOutcome{Source: OutcomeSelfReported}
	if self.Verified() {
		t.Error("laporan yang belum diperiksa admin tidak boleh dianggap terverifikasi")
	}
	verified := SelectionOutcome{Source: OutcomeAdminVerified}
	if !verified.Verified() {
		t.Error("laporan yang sudah diperiksa admin harus dianggap terverifikasi")
	}
}

func TestIsDocumentEvidenceMengenaliBerkasPengumuman(t *testing.T) {
	cases := map[string]bool{
		"https://contoh.id/pengumuman.pdf": true,
		"https://contoh.id/pengumuman.PDF": true,
		"/media/outcomes/bukti-2026.pdf":   true,
		"https://contoh.id/bukti.png":      false,
		"https://contoh.id/tautan":         false,
		"":                                 false,
		"   ":                              false,
	}

	for url, want := range cases {
		if got := IsDocumentEvidence(url); got != want {
			t.Errorf("IsDocumentEvidence(%q) = %v, harap %v", url, got, want)
		}
	}
}

func TestKPIMetricHanyaMenerimaMetricYangDihitung(t *testing.T) {
	valid := []KPIMetric{
		KPIMetricFinalPassRate,
		KPIMetricStagePassRate,
		KPIMetricParticipants,
		KPIMetricCurriculumCompletion,
	}
	for _, metric := range valid {
		if !metric.Valid() {
			t.Errorf("metric %q harus dianggap valid", metric)
		}
		if metric.Label() == "" || metric.Label() == string(metric) {
			t.Errorf("metric %q belum punya label yang bisa dibaca pengguna", metric)
		}
	}

	if KPIMetric("prediksi_kelulusan").Valid() {
		t.Error("metric prediksi tidak boleh diterima — fase ini hanya mengukur hasil")
	}
}
