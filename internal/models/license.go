package models

import "time"

/* ------------------------------- izin soal ------------------------------- */

// LicenseScope menjelaskan asal hak pakai sebuah soal atau instrumen.
type LicenseScope string

const (
	// LicenseInternal adalah soal yang dibuat sendiri oleh tim akademik.
	LicenseInternal LicenseScope = "internal"
	// LicenseVendor adalah soal yang dibeli/dilisensikan dari pihak ketiga.
	LicenseVendor LicenseScope = "vendor"
	// LicensePublicDomain adalah soal berstatus domain publik.
	LicensePublicDomain LicenseScope = "public_domain"
	// LicenseClientOwned adalah soal milik instansi klien.
	LicenseClientOwned LicenseScope = "client_owned"
)

// Valid memeriksa apakah cakupan lisensi dikenal.
func (s LicenseScope) Valid() bool {
	switch s {
	case LicenseInternal, LicenseVendor, LicensePublicDomain, LicenseClientOwned:
		return true
	default:
		return false
	}
}

// LicenseStatus adalah keadaan sebuah izin.
type LicenseStatus string

const (
	// LicenseActive berarti izin masih berlaku.
	LicenseActive LicenseStatus = "aktif"
	// LicenseExpired berarti masa berlaku sudah lewat.
	LicenseExpired LicenseStatus = "kedaluwarsa"
	// LicenseRevoked berarti izin dicabut sebelum masa berlakunya habis.
	LicenseRevoked LicenseStatus = "dicabut"
)

// Valid memeriksa apakah status lisensi dikenal.
func (s LicenseStatus) Valid() bool {
	return s == LicenseActive || s == LicenseExpired || s == LicenseRevoked
}

// QuestionLicense adalah bukti hak pakai satu soal.
//
// Ini jawaban atas permintaan "harus dipastikan izinnya": setiap soal wajib
// punya baris lisensi, dan soal hanya boleh masuk sesi tryout bila izinnya
// aktif. Nomor perjanjian beserta berkasnya disimpan agar klaim izin bisa
// ditelusuri, bukan sekadar catatan bebas.
type QuestionLicense struct {
	ID         string       `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	QuestionID string       `gorm:"column:question_id;not null" json:"questionId"`
	Scope      LicenseScope `gorm:"column:scope;not null;default:internal" json:"scope"`
	// Owner adalah pemilik hak cipta (mis. nama vendor atau instansi).
	Owner string `gorm:"column:owner;not null;default:''" json:"owner"`
	// AgreementNo adalah nomor PKS/perjanjian kerjasama.
	AgreementNo string `gorm:"column:agreement_no;not null;default:''" json:"agreementNo"`
	// AgreementURL menunjuk berkas scan perjanjian (opsional).
	AgreementURL *string `gorm:"column:agreement_url" json:"agreementUrl,omitempty"`
	// CommercialUse menandai izin pemakaian pada produk berbayar.
	CommercialUse bool `gorm:"column:commercial_use;not null;default:true" json:"commercialUse"`
	// SelectionUse menandai izin pemakaian untuk asesmen/seleksi, bukan hanya latihan.
	SelectionUse bool `gorm:"column:selection_use;not null;default:false" json:"selectionUse"`
	// AllowedSectors membatasi sektor pemakaian; kosong berarti semua sektor.
	AllowedSectors StringList `gorm:"column:allowed_sectors;type:jsonb;not null;default:'[]'" json:"allowedSectors"`
	ValidFrom      time.Time  `gorm:"column:valid_from;not null;default:now()" json:"validFrom"`
	// ValidUntil NULL berarti tanpa batas waktu.
	ValidUntil *time.Time    `gorm:"column:valid_until" json:"validUntil,omitempty"`
	Status     LicenseStatus `gorm:"column:status;not null;default:aktif" json:"status"`
	Notes      string        `gorm:"column:notes;not null;default:''" json:"notes"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (QuestionLicense) TableName() string { return "question_licenses" }

// EffectiveStatus menghitung status lisensi pada waktu tertentu.
//
// Status tersimpan hanya menyimpan keputusan manusia (dicabut atau tidak);
// kedaluwarsa dihitung dari `ValidUntil` supaya izin otomatis berhenti berlaku
// tanpa perlu tugas terjadwal yang mengubah barisnya.
func (l QuestionLicense) EffectiveStatus(now time.Time) LicenseStatus {
	if l.Status == LicenseRevoked {
		return LicenseRevoked
	}
	if l.ValidUntil != nil && now.After(*l.ValidUntil) {
		return LicenseExpired
	}
	return LicenseActive
}

// IsUsable menentukan apakah izin boleh dipakai untuk latihan peserta.
func (l QuestionLicense) IsUsable(now time.Time) bool {
	return l.EffectiveStatus(now) == LicenseActive
}

/* --------------------------- instrumen psikologi -------------------------- */

// InstrumentKind membedakan instrumen latihan dan instrumen berlisensi.
type InstrumentKind string

const (
	// InstrumentPractice adalah psikotes bergaya latihan yang boleh dilatihkan
	// peserta (deret angka, Kraepelin/Pauli, DAP, Wartegg, EPPS).
	InstrumentPractice InstrumentKind = "latihan"
	// InstrumentLicensed adalah instrumen berlisensi (mis. IST, Papi Kostick,
	// DISC) yang hanya boleh dijalankan psikolog berkompeten.
	InstrumentLicensed InstrumentKind = "berlisensi"
)

// Valid memeriksa apakah jenis instrumen dikenal.
func (k InstrumentKind) Valid() bool {
	return k == InstrumentPractice || k == InstrumentLicensed
}

// PsychologicalInstrument adalah instrumen tes psikologi beserta status izinnya.
//
// Untuk instrumen berlisensi, `AdministeredBy` wajib diisi: instrumen hanya
// boleh dibuka bila ada psikolog penanggung jawab dengan izin praktik aktif
// (SIPP HIMPSI atau SIPPK untuk psikolog klinis).
type PsychologicalInstrument struct {
	ID   string         `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	Code string         `gorm:"column:code;not null" json:"code"`
	Name string         `gorm:"column:name;not null" json:"name"`
	Kind InstrumentKind `gorm:"column:kind;not null;default:latihan" json:"kind"`
	// Vendor adalah pemilik alat tes (mis. penerbit atau lembaga psikologi).
	Vendor       string        `gorm:"column:vendor;not null;default:''" json:"vendor"`
	LicenseNo    string        `gorm:"column:license_no;not null;default:''" json:"licenseNo"`
	LicenseURL   *string       `gorm:"column:license_url" json:"licenseUrl,omitempty"`
	LicenseFrom  *time.Time    `gorm:"column:license_from" json:"licenseFrom,omitempty"`
	LicenseUntil *time.Time    `gorm:"column:license_until" json:"licenseUntil,omitempty"`
	Status       LicenseStatus `gorm:"column:status;not null;default:aktif" json:"status"`
	// AdministeredBy adalah id pengguna ber-role psikolog penanggung jawab.
	AdministeredBy *string `gorm:"column:administered_by" json:"administeredBy,omitempty"`
	// ScoringNotes menjelaskan norma & cara skoring (boleh dikosongkan untuk
	// instrumen latihan).
	ScoringNotes string `gorm:"column:scoring_notes;not null;default:''" json:"scoringNotes"`
	Active       bool   `gorm:"column:active;not null;default:true" json:"active"`
	Timestamps
}

// TableName menetapkan nama tabel.
func (PsychologicalInstrument) TableName() string { return "psychological_instruments" }

// EffectiveStatus menghitung status izin instrumen pada waktu tertentu.
func (i PsychologicalInstrument) EffectiveStatus(now time.Time) LicenseStatus {
	if i.Status == LicenseRevoked {
		return LicenseRevoked
	}
	if i.LicenseUntil != nil && now.After(*i.LicenseUntil) {
		return LicenseExpired
	}
	return LicenseActive
}

// Usable menentukan apakah instrumen boleh dipakai pada waktu tertentu.
//
// Instrumen berlisensi wajib punya psikolog penanggung jawab; tanpa itu
// instrumen tidak boleh dibuka walau masa lisensinya masih berlaku.
func (i PsychologicalInstrument) Usable(now time.Time) bool {
	if !i.Active || i.EffectiveStatus(now) != LicenseActive {
		return false
	}
	if i.Kind == InstrumentLicensed {
		return i.AdministeredBy != nil && *i.AdministeredBy != ""
	}
	return true
}
