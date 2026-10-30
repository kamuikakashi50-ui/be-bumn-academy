package models

import (
	"testing"
	"time"
)

func ptrTime(value time.Time) *time.Time { return &value }

func TestQuestionLicenseEffectiveStatus(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -1)
	future := now.AddDate(0, 0, 30)

	cases := []struct {
		name    string
		license QuestionLicense
		want    LicenseStatus
	}{
		{
			name:    "tanpa batas waktu tetap aktif",
			license: QuestionLicense{Status: LicenseActive},
			want:    LicenseActive,
		},
		{
			name:    "masa berlaku belum lewat",
			license: QuestionLicense{Status: LicenseActive, ValidUntil: &future},
			want:    LicenseActive,
		},
		{
			name:    "masa berlaku sudah lewat menjadi kedaluwarsa",
			license: QuestionLicense{Status: LicenseActive, ValidUntil: &past},
			want:    LicenseExpired,
		},
		{
			name:    "izin yang dicabut tidak pernah dianggap aktif",
			license: QuestionLicense{Status: LicenseRevoked, ValidUntil: &future},
			want:    LicenseRevoked,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.license.EffectiveStatus(now); got != tc.want {
				t.Errorf("EffectiveStatus = %q, harap %q", got, tc.want)
			}
		})
	}
}

func TestQuestionLicenseIsUsable(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

	active := QuestionLicense{Status: LicenseActive}
	if !active.IsUsable(now) {
		t.Error("izin aktif harus bisa dipakai")
	}

	expired := QuestionLicense{Status: LicenseActive, ValidUntil: ptrTime(now.Add(-time.Hour))}
	if expired.IsUsable(now) {
		t.Error("izin kedaluwarsa tidak boleh dipakai")
	}
}

func TestLicenseScopeDanStatusValid(t *testing.T) {
	for _, scope := range []LicenseScope{
		LicenseInternal, LicenseVendor, LicensePublicDomain, LicenseClientOwned,
	} {
		if !scope.Valid() {
			t.Errorf("cakupan %q harus valid", scope)
		}
	}
	if LicenseScope("karangan").Valid() {
		t.Error("cakupan tak dikenal harus invalid")
	}

	for _, status := range []LicenseStatus{LicenseActive, LicenseExpired, LicenseRevoked} {
		if !status.Valid() {
			t.Errorf("status %q harus valid", status)
		}
	}
	if LicenseStatus("karangan").Valid() {
		t.Error("status tak dikenal harus invalid")
	}
}

func TestPsychologicalInstrumentUsable(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	psychologist := "11111111-1111-1111-1111-111111111111"

	cases := []struct {
		name       string
		instrument PsychologicalInstrument
		want       bool
	}{
		{
			name:       "instrumen latihan aktif bisa dipakai",
			instrument: PsychologicalInstrument{Kind: InstrumentPractice, Status: LicenseActive, Active: true},
			want:       true,
		},
		{
			name:       "instrumen berlisensi tanpa psikolog tidak boleh dipakai",
			instrument: PsychologicalInstrument{Kind: InstrumentLicensed, Status: LicenseActive, Active: true},
			want:       false,
		},
		{
			name: "instrumen berlisensi dengan psikolog boleh dipakai",
			instrument: PsychologicalInstrument{
				Kind: InstrumentLicensed, Status: LicenseActive, Active: true,
				AdministeredBy: &psychologist,
			},
			want: true,
		},
		{
			name: "lisensi lewat membuat instrumen tidak bisa dipakai",
			instrument: PsychologicalInstrument{
				Kind: InstrumentLicensed, Status: LicenseActive, Active: true,
				AdministeredBy: &psychologist, LicenseUntil: ptrTime(now.Add(-time.Hour)),
			},
			want: false,
		},
		{
			name:       "instrumen nonaktif tidak bisa dipakai",
			instrument: PsychologicalInstrument{Kind: InstrumentPractice, Status: LicenseActive, Active: false},
			want:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.instrument.Usable(now); got != tc.want {
				t.Errorf("Usable = %v, harap %v", got, tc.want)
			}
		})
	}
}
