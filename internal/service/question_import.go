package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/bumn-academy/be-bumn-academy/internal/dto"
	"github.com/bumn-academy/be-bumn-academy/internal/models"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/apperror"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/bank"
	"github.com/bumn-academy/be-bumn-academy/internal/pkg/fingerprint"
	"github.com/xuri/excelize/v2"
)

// importDuplicateLimit membatasi jumlah temuan soal kembar yang dilaporkan pada
// hasil import: cukup untuk ditinjau, tidak membuat balasan API membengkak.
const importDuplicateLimit = 50

// ImportQuestions mengimpor soal dari berkas Excel (.xlsx) atau CSV.
//
// Kolom yang dikenali (nama kolom tidak peka huruf besar/kecil):
//
//	bankKey / kategori    → kunci bank soal (numerik, verbal, …)
//	tryoutId / tryout     → paket tryout pemilik soal (opsional bila
//	                        `defaultTryoutID` dikirim dari formulir)
//	sectorKey / sektor    → sektor/klaster BUMN (mis. jasa_keuangan)
//	testType / jenistes   → jenis tes (tkd, akhlaq, twk, bing, tkb, psikotes)
//	positionFamily        → keluarga jabatan (mis. keuangan, ti)
//	companyKey            → BUMN tertentu (opsional)
//	variantGroup / varian → grup varian soal sekeluarga
//	retentionUntil        → batas masa simpan soal (opsional)
//	text / soal           → isi pertanyaan
//	optionA..optionD      → pilihan jawaban (atau "options" dipisah "|")
//	answer / kunci        → huruf (A–D) atau angka indeks
//	explanation           → pembahasan
//	difficulty            → mudah | sedang | sulit (opsional)
//	id                    → id soal (opsional; kosong = dibuat otomatis)
//	position              → urutan dalam bank (opsional)
//	licenseScope          → internal | vendor | public_domain | client_owned
//	licenseOwner          → pemilik hak cipta (opsional)
//	agreementNo           → nomor perjanjian (WAJIB bila lisensi vendor)
//	licenseValidUntil     → masa berlaku izin (opsional)
//
// Baris yang gagal dilewati dan dilaporkan pada `errors`, sehingga satu berkas
// besar tidak perlu diulang hanya karena beberapa baris bermasalah. Baris tanpa
// tryout (kolom kosong dan tanpa `defaultTryoutID`) tetap diimpor sebagai soal
// "belum ditautkan" dan dihitung pada `unassigned`.
//
// Setelah tersimpan, tiap soal baru mendapat izin (default: milik sendiri),
// sidik jari teksnya dihitung, dan kemungkinan soal kembar dilaporkan pada
// `duplicates` — import tetap berjalan, temuan itu bahan tinjauan tim akademik.
func (s *AdminService) ImportQuestions(ctx context.Context, actorID, filename string, data []byte, defaultTryoutID string) (*dto.ImportResult, error) {
	rows, err := readTabularRows(filename, data)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, apperror.BadRequest("Berkas import tidak memiliki baris data.")
	}

	header := map[string]int{}
	for index, name := range rows[0] {
		header[normalizeHeader(name)] = index
	}

	// Daftar tryout dipakai untuk memvalidasi kolom tryoutId per baris sekaligus
	// menjadi himpunan valid — sekali query, bukan per baris.
	titles, err := s.tryoutTitles(ctx)
	if err != nil {
		return nil, err
	}

	defaultTryout := strings.TrimSpace(defaultTryoutID)
	if defaultTryout != "" {
		if _, ok := titles[defaultTryout]; !ok {
			return nil, apperror.Validation("Tryout tujuan tidak ditemukan.", nil)
		}
	}

	result := &dto.ImportResult{BankKeys: map[string]int{}}
	questions := make([]models.Question, 0, len(rows)-1)
	licenses := make(map[string]*dto.QuestionLicenseInput, len(rows)-1)

	for rowIndex, row := range rows[1:] {
		excelRow := rowIndex + 2 // baris 1 = header
		if isBlankRow(row) {
			continue
		}
		result.TotalRows++

		parsed, err := s.parseImportRow(row, header, excelRow, titles, defaultTryout)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, dto.ImportRowError{Row: excelRow, Message: err.Error()})
			continue
		}

		questions = append(questions, parsed.question)
		licenses[parsed.question.ID] = parsed.license
		result.BankKeys[parsed.question.BankKey]++
		if parsed.question.TryoutID == nil {
			result.Unassigned++
		}
	}

	if len(questions) == 0 {
		return result, nil
	}

	// Hitung berapa baris yang akan memperbarui soal yang sudah ada.
	existing, err := s.existingQuestionIDs(ctx, questions)
	if err != nil {
		return nil, err
	}
	for _, question := range questions {
		if existing[question.ID] {
			result.Updated++
		} else {
			result.Imported++
		}
	}

	if err := s.tryouts.BulkUpsertQuestions(ctx, questions); err != nil {
		return nil, err
	}

	// Izin: soal yang sudah punya izin tidak ditimpa berkas import.
	if s.licenses != nil {
		if _, err := s.licenses.EnsureLicensesForQuestions(ctx, licenses); err != nil {
			return nil, err
		}
	}

	// Sidik jari & temuan kembar (masukan "susah mencari soal yang sama").
	if s.compliance != nil {
		for _, question := range questions {
			s.compliance.Refresh(ctx, question, actorID, models.QuestionActionImport,
				"Soal masuk lewat import "+filename+".")
		}
		result.Duplicates = s.compliance.HintsFor(ctx, questions, fingerprint.DefaultThreshold, importDuplicateLimit)
	}

	s.tryoutSvc.InvalidateBankCache(ctx)
	return result, nil
}

// importRow adalah satu baris berkas import yang sudah lolos parsing.
type importRow struct {
	question models.Question
	license  *dto.QuestionLicenseInput
}

// parseImportRow mengubah satu baris berkas menjadi model soal.
//
// `validTryouts` adalah peta id → judul paket tryout (dipakai memvalidasi kolom
// tryoutId tanpa query per baris); `defaultTryoutID` dipakai bila baris tidak
// menyebut tryout.
func (s *AdminService) parseImportRow(
	row []string,
	header map[string]int,
	excelRow int,
	validTryouts map[string]string,
	defaultTryoutID string,
) (importRow, error) {
	bankKey := strings.TrimSpace(cell(row, header, "bankkey", "kategori", "bank"))
	if bankKey == "" {
		return importRow{}, fmt.Errorf("kolom bankKey/kategori kosong")
	}
	if !bank.IsValid(bankKey) {
		return importRow{}, fmt.Errorf("kategori soal %q tidak dikenal", bankKey)
	}

	tryoutID := strings.TrimSpace(cell(row, header, "tryoutid", "tryout", "pakettryout"))
	if tryoutID == "" {
		tryoutID = defaultTryoutID
	}
	if tryoutID != "" {
		if _, ok := validTryouts[tryoutID]; !ok {
			return importRow{}, fmt.Errorf("tryoutId %q tidak dikenali", tryoutID)
		}
	}

	text := strings.TrimSpace(cell(row, header, "text", "soal", "pertanyaan", "question"))
	if text == "" {
		return importRow{}, fmt.Errorf("kolom text/soal kosong")
	}

	options := collectOptions(row, header)
	if len(options) < 2 {
		return importRow{}, fmt.Errorf("minimal dua pilihan jawaban diperlukan")
	}

	answerIndex, err := parseAnswerIndex(cell(row, header, "answer", "kunci", "jawaban", "answerindex"), len(options))
	if err != nil {
		return importRow{}, err
	}

	explanation := strings.TrimSpace(cell(row, header, "explanation", "pembahasan"))
	if explanation == "" {
		return importRow{}, fmt.Errorf("kolom explanation/pembahasan kosong")
	}

	id := strings.TrimSpace(cell(row, header, "id"))
	if id == "" {
		id = fmt.Sprintf("%s-%s", bankKey, shortID())
	}

	position := 0
	if raw := strings.TrimSpace(cell(row, header, "position", "urutan")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			return importRow{}, fmt.Errorf("kolom position/urutan bukan angka: %q", raw)
		}
		position = parsed
	}

	sectorKey, err := validateSector(cell(row, header, "sectorkey", "sektor"))
	if err != nil {
		return importRow{}, fmt.Errorf("kolom sectorKey/sektor tidak dikenal")
	}
	testType, err := validateTestType(cell(row, header, "testtype", "jenistes"))
	if err != nil {
		return importRow{}, fmt.Errorf("kolom testType/jenistes tidak dikenal")
	}
	positionFamily, err := validatePositionFamily(cell(row, header, "positionfamily", "keluargajabatan"))
	if err != nil {
		return importRow{}, fmt.Errorf("kolom positionFamily/keluargajabatan tidak dikenal")
	}

	retention, err := parseOptionalTime(cell(row, header, "retentionuntil", "masasimpan"))
	if err != nil {
		return importRow{}, fmt.Errorf("kolom retentionUntil tidak valid")
	}

	question := models.Question{
		ID:             id,
		BankKey:        bankKey,
		SectorKey:      sectorKey,
		CompanyKey:     optionalString(strings.TrimSpace(cell(row, header, "companykey", "bumn"))),
		TestType:       testType,
		PositionFamily: positionFamily,
		VariantGroup:   optionalString(strings.TrimSpace(cell(row, header, "variantgroup", "varian"))),
		RetentionUntil: retention,
		Text:           text,
		Options:        models.StringList(options),
		AnswerIndex:    answerIndex,
		Explanation:    explanation,
		Difficulty:     difficultyOr(strings.ToLower(strings.TrimSpace(cell(row, header, "difficulty", "kesulitan")))),
		Position:       position,
		Active:         true,
	}
	if tryoutID != "" {
		question.TryoutID = &tryoutID
	}

	return importRow{question: question, license: importLicense(row, header)}, nil
}

// importLicense membaca kolom izin pada berkas import.
//
// Bila berkas tidak menyebut izin sama sekali, nil dikembalikan dan soal
// diperlakukan sebagai milik sendiri (`internal`). Baris dengan lisensi vendor
// tanpa nomor perjanjian akan ditolak saat izinnya disimpan, sehingga tidak ada
// soal yang status izinnya kabur.
func importLicense(row []string, header map[string]int) *dto.QuestionLicenseInput {
	scope := strings.TrimSpace(cell(row, header, "licensescope", "cakupanizin", "cakupan"))
	owner := strings.TrimSpace(cell(row, header, "licenseowner", "pemilikizin"))
	agreement := strings.TrimSpace(cell(row, header, "agreementno", "noperjanjian", "nomorperjanjian"))
	validUntil := strings.TrimSpace(cell(row, header, "licensevaliduntil", "izinsampai"))

	if scope == "" && owner == "" && agreement == "" && validUntil == "" {
		return nil
	}
	return &dto.QuestionLicenseInput{
		Scope:       scope,
		Owner:       owner,
		AgreementNo: agreement,
		ValidUntil:  validUntil,
	}
}

// collectOptions mengambil pilihan jawaban dari kolom optionA..optionF atau
// dari satu kolom "options" yang dipisah karakter "|".
func collectOptions(row []string, header map[string]int) []string {
	options := make([]string, 0, 4)
	for _, key := range []string{"optiona", "optionb", "optionc", "optiond", "optione", "optionf"} {
		if value := strings.TrimSpace(cell(row, header, key)); value != "" {
			options = append(options, value)
		}
	}
	if len(options) >= 2 {
		return options
	}

	combined := cell(row, header, "options", "pilihan")
	if combined == "" {
		return options
	}
	parts := strings.Split(combined, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// parseAnswerIndex menerima huruf (A–F) atau angka (0-based / 1-based).
func parseAnswerIndex(raw string, optionCount int) (int, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return 0, fmt.Errorf("kolom answer/kunci kosong")
	}

	if len(value) == 1 && value[0] >= 'A' && value[0] <= 'F' {
		index := int(value[0] - 'A')
		if index >= optionCount {
			return 0, fmt.Errorf("kunci jawaban %q di luar jumlah pilihan", raw)
		}
		return index, nil
	}

	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("kunci jawaban %q tidak dikenali", raw)
	}
	// Angka 1..N dianggap 1-based (paling umum di berkas Excel).
	if number >= 1 && number <= optionCount {
		return number - 1, nil
	}
	if number >= 0 && number < optionCount {
		return number, nil
	}
	return 0, fmt.Errorf("kunci jawaban %q di luar jumlah pilihan", raw)
}

// existingQuestionIDs mengambil id soal yang sudah ada pada database.
func (s *AdminService) existingQuestionIDs(ctx context.Context, questions []models.Question) (map[string]bool, error) {
	ids := make([]string, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.ID)
	}

	var found []string
	err := s.tryouts.DB().WithContext(ctx).Model(&models.Question{}).
		Where("id IN ?", ids).
		Pluck("id", &found).Error
	if err != nil {
		return nil, apperror.Internal(err, "Gagal memeriksa soal yang sudah ada.")
	}

	out := make(map[string]bool, len(found))
	for _, id := range found {
		out[id] = true
	}
	return out, nil
}

/* ------------------------------- pembacaan -------------------------------- */

// readTabularRows membaca CSV atau XLSX menjadi matriks string.
//
// Format ditentukan dari ekstensi berkas; bila ekstensi tidak dikenal, format
// dideteksi dari isi (berkas XLSX selalu diawali tanda tangan ZIP "PK").
func readTabularRows(filename string, data []byte) ([][]string, error) {
	lower := strings.ToLower(filename)

	switch {
	case strings.HasSuffix(lower, ".csv"), strings.HasSuffix(lower, ".txt"):
		return readCSVRows(data)
	case strings.HasSuffix(lower, ".xlsx"), strings.HasSuffix(lower, ".xlsm"), strings.HasSuffix(lower, ".xltx"):
		return readExcelRows(data)
	}

	// Deteksi dari isi berkas.
	if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
		return readExcelRows(data)
	}
	return readCSVRows(data)
}

// readCSVRows membaca berkas CSV.
func readCSVRows(data []byte) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	var rows [][]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, apperror.BadRequest("Berkas CSV tidak bisa dibaca: " + err.Error())
		}
		rows = append(rows, record)
	}
	if len(rows) == 0 {
		return nil, apperror.BadRequest("Berkas CSV kosong.")
	}
	return rows, nil
}

// readExcelRows membaca berkas Excel (.xlsx) memakai sheet pertama.
func readExcelRows(data []byte) ([][]string, error) {
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, apperror.BadRequest("Berkas Excel tidak bisa dibaca: " + err.Error())
	}
	defer func() { _ = file.Close() }()

	sheet := file.GetSheetName(0)
	if sheet == "" {
		return nil, apperror.BadRequest("Berkas Excel tidak memiliki sheet.")
	}
	rows, err := file.GetRows(sheet)
	if err != nil {
		return nil, apperror.BadRequest("Gagal membaca isi sheet: " + err.Error())
	}
	if len(rows) == 0 {
		return nil, apperror.BadRequest("Sheet pertama pada berkas Excel kosong.")
	}
	return rows, nil
}

// cell mengambil nilai kolom berdasarkan beberapa alias nama header.
func cell(row []string, header map[string]int, aliases ...string) string {
	for _, alias := range aliases {
		if index, ok := header[normalizeHeader(alias)]; ok && index < len(row) {
			return row[index]
		}
	}
	return ""
}

// normalizeHeader menyamakan nama kolom (huruf kecil, tanpa spasi/underscore).
func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "")
	return replacer.Replace(value)
}

func isBlankRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
