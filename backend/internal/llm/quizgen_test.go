package llm

import (
	"strings"
	"testing"
	"unicode/utf8"
)

const validQuestionsJSON = `{"questions": [
  {"tipe": "pilihan_ganda", "pertanyaan": "2+2?", "opsi": ["3","4","5","6"], "jawaban": 1, "pembahasan": "2+2=4"},
  {"tipe": "benar_salah", "pertanyaan": "1=1?", "jawaban": "benar", "pembahasan": "ya"},
  {"tipe": "isian", "pertanyaan": "Ibukota RI?", "jawaban": "Jakarta", "pembahasan": "Jakarta"}
]}`

var validCounts = map[string]int{"pilihan_ganda": 1, "benar_salah": 1, "isian": 1}

func TestBuildPromptPreservesArabicQuotesInstruction(t *testing.T) {
	prompt := buildPrompt("Pendidikan Agama Islam", 5, validCounts, "Materi Hadis", 0)
	if !strings.Contains(prompt, "PERTAHANKAN teks Arabnya") {
		t.Error("prompt missing Arabic-preservation instruction")
	}
}

func TestBuildPromptCountsIncluded(t *testing.T) {
	counts := map[string]int{"pilihan_ganda": 10, "benar_salah": 0, "isian": 5, "deskripsi": 5}
	prompt := buildPrompt("IPA", 5, counts, "", 0)
	for _, want := range []string{"10 soal pilihan ganda", "5 soal isian singkat", "5 soal uraian/deskripsi"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(prompt, "benar/salah") {
		t.Error("a zero-count type should not be mentioned")
	}
}

func TestValidateQuestionsValidPasses(t *testing.T) {
	qs, err := validateQuestions([]byte(validQuestionsJSON), validCounts, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(qs) != 3 {
		t.Fatalf("len = %d, want 3", len(qs))
	}
}

func TestValidateQuestionsWrongCountFails(t *testing.T) {
	_, err := validateQuestions([]byte(validQuestionsJSON), map[string]int{"pilihan_ganda": 1, "benar_salah": 1, "isian": 3}, 0)
	assertErrContains(t, err, "jumlah/komposisi soal")
}

func TestValidateQuestionsWrongCompositionFails(t *testing.T) {
	_, err := validateQuestions([]byte(validQuestionsJSON), map[string]int{"pilihan_ganda": 2, "isian": 1}, 0)
	assertErrContains(t, err, "komposisi")
}

func TestValidateQuestionsWrongOptionCountFails(t *testing.T) {
	bad := `{"questions": [
	  {"tipe": "pilihan_ganda", "pertanyaan": "2+2?", "opsi": ["a","b","c"], "jawaban": 1, "pembahasan": "2+2=4"},
	  {"tipe": "benar_salah", "pertanyaan": "1=1?", "jawaban": "benar", "pembahasan": "ya"},
	  {"tipe": "isian", "pertanyaan": "Ibukota RI?", "jawaban": "Jakarta", "pembahasan": "Jakarta"}
	]}`
	_, err := validateQuestions([]byte(bad), validCounts, 0)
	assertErrContains(t, err, "opsi")
}

func TestValidateQuestionsBadAnswerIndexFails(t *testing.T) {
	bad := strings.Replace(validQuestionsJSON, `"jawaban": 1, "pembahasan": "2+2=4"`, `"jawaban": 4, "pembahasan": "2+2=4"`, 1)
	if _, err := validateQuestions([]byte(bad), validCounts, 0); err == nil {
		t.Fatal("expected error for out-of-range jawaban index")
	}
}

func TestValidateQuestionsBadTFAnswerFails(t *testing.T) {
	bad := strings.Replace(validQuestionsJSON, `"jawaban": "benar"`, `"jawaban": "mungkin"`, 1)
	if _, err := validateQuestions([]byte(bad), validCounts, 0); err == nil {
		t.Fatal("expected error for invalid benar_salah answer")
	}
}

func TestValidateQuestionsEmptyIsianFails(t *testing.T) {
	bad := strings.Replace(validQuestionsJSON, `"jawaban": "Jakarta", "pembahasan": "Jakarta"`, `"jawaban": "   ", "pembahasan": "Jakarta"`, 1)
	if _, err := validateQuestions([]byte(bad), validCounts, 0); err == nil {
		t.Fatal("expected error for blank isian answer")
	}
}

func TestValidateQuestionsDeskripsiValidPasses(t *testing.T) {
	data := `{"questions": [
	  {"tipe": "deskripsi", "pertanyaan": "Jelaskan siklus air!",
	   "jawaban": "Siklus air adalah perputaran air dari laut ke awan lalu turun sebagai hujan, berulang terus menerus.",
	   "pembahasan": "Evaporasi, kondensasi, presipitasi."}
	]}`
	qs, err := validateQuestions([]byte(data), map[string]int{"deskripsi": 1}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(qs) != 1 {
		t.Fatalf("len = %d, want 1", len(qs))
	}
}

func TestValidateQuestionsDeskripsiEmptyJawabanFails(t *testing.T) {
	data := `{"questions": [
	  {"tipe": "deskripsi", "pertanyaan": "Jelaskan siklus air!", "jawaban": "  ", "pembahasan": "..."}
	]}`
	_, err := validateQuestions([]byte(data), map[string]int{"deskripsi": 1}, 0)
	assertErrContains(t, err, "jawaban deskripsi")
}

func TestValidateQuestionsBalancedMathDelimitersPass(t *testing.T) {
	good := strings.Replace(validQuestionsJSON, `"pertanyaan": "2+2?"`, `"pertanyaan": "Berapa $x^2$ jika $x=2$?"`, 1)
	if _, err := validateQuestions([]byte(good), validCounts, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateQuestionsUnbalancedMathDelimitersFail(t *testing.T) {
	bad := strings.Replace(validQuestionsJSON, `"pertanyaan": "2+2?"`, `"pertanyaan": "Berapa $x^2 jika x=2?"`, 1)
	_, err := validateQuestions([]byte(bad), validCounts, 0)
	assertErrContains(t, err, "delimiter $")
}

func TestValidateQuestionsUnbalancedMathDelimitersInOpsiFail(t *testing.T) {
	bad := strings.Replace(validQuestionsJSON, `["3","4","5","6"]`, `["3","$4","5","6"]`, 1)
	_, err := validateQuestions([]byte(bad), validCounts, 0)
	assertErrContains(t, err, "delimiter $")
}

func TestValidateQuestionsGambarGeneratedValidPasses(t *testing.T) {
	good := strings.Replace(validQuestionsJSON, `"pembahasan": "2+2=4"`, `"pembahasan": "2+2=4", "gambar_tipe": "generated", "gambar_prompt": "diagram segitiga siku-siku"`, 1)
	if _, err := validateQuestions([]byte(good), validCounts, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateQuestionsGambarStockValidPasses(t *testing.T) {
	good := strings.Replace(validQuestionsJSON, `"pembahasan": "2+2=4"`, `"pembahasan": "2+2=4", "gambar_tipe": "stock", "gambar_cari": "traditional Indonesian house"`, 1)
	if _, err := validateQuestions([]byte(good), validCounts, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateQuestionsGambarInvalidTipeFails(t *testing.T) {
	bad := strings.Replace(validQuestionsJSON, `"pembahasan": "2+2=4"`, `"pembahasan": "2+2=4", "gambar_tipe": "lainnya"`, 1)
	_, err := validateQuestions([]byte(bad), validCounts, 0)
	assertErrContains(t, err, "gambar_tipe")
}

func TestValidateQuestionsGambarGeneratedMissingPromptFails(t *testing.T) {
	bad := strings.Replace(validQuestionsJSON, `"pembahasan": "2+2=4"`, `"pembahasan": "2+2=4", "gambar_tipe": "generated"`, 1)
	_, err := validateQuestions([]byte(bad), validCounts, 0)
	assertErrContains(t, err, "gambar_prompt")
}

func TestValidateQuestionsGambarStockMissingQueryFails(t *testing.T) {
	bad := strings.Replace(validQuestionsJSON, `"pembahasan": "2+2=4"`, `"pembahasan": "2+2=4", "gambar_tipe": "stock", "gambar_cari": "   "`, 1)
	_, err := validateQuestions([]byte(bad), validCounts, 0)
	assertErrContains(t, err, "gambar_cari")
}

func TestTruncateUTF8DoesNotSplitMultiByteRune(t *testing.T) {
	// "materi ajar" repeated with an Arabic word ("بسم") right at the cut
	// point — a naive s[:maxBytes] would slice through the middle of one of
	// its multi-byte UTF-8 runes and produce invalid UTF-8.
	s := strings.Repeat("a", 10) + "بسم" + strings.Repeat("b", 10)
	for cut := 8; cut <= 14; cut++ {
		got := truncateUTF8(s, cut)
		if !utf8.ValidString(got) {
			t.Errorf("truncateUTF8(s, %d) = %q, not valid UTF-8", cut, got)
		}
		if len(got) > cut {
			t.Errorf("truncateUTF8(s, %d) = %q, len %d > %d", cut, got, len(got), cut)
		}
	}
}

func TestTruncateUTF8NoopWhenUnderLimit(t *testing.T) {
	if got := truncateUTF8("pendek", 100); got != "pendek" {
		t.Errorf("truncateUTF8 = %q, want unchanged", got)
	}
}

func assertErrContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
		t.Fatalf("error %q does not contain %q", err.Error(), substr)
	}
}
