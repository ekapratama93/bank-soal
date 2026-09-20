package httpapi

import "testing"

func validAdminQuestions() []adminQuestionInput {
	return []adminQuestionInput{
		{Tipe: "pilihan_ganda", Pertanyaan: "2+2?", Opsi: []string{"3", "4", "5", "6"}, Jawaban: float64(1), Pembahasan: "2+2=4"},
		{Tipe: "benar_salah", Pertanyaan: "1=1?", Jawaban: "benar", Pembahasan: "ya"},
		{Tipe: "isian", Pertanyaan: "Ibukota RI?", Jawaban: "Jakarta", Pembahasan: "Jakarta"},
	}
}

func TestValidateAdminQuestionsValidPasses(t *testing.T) {
	qs, errMsg := validateAdminQuestions(validAdminQuestions())
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if len(qs) != 3 {
		t.Fatalf("len = %d, want 3", len(qs))
	}
	if qs[0].Jawaban != 1 {
		t.Errorf("pilihan_ganda jawaban = %v, want int 1", qs[0].Jawaban)
	}
}

func TestValidateAdminQuestionsEmptyFails(t *testing.T) {
	_, errMsg := validateAdminQuestions(nil)
	if errMsg == "" {
		t.Fatal("expected error for empty question list")
	}
}

func TestValidateAdminQuestionsInvalidTipeFails(t *testing.T) {
	items := validAdminQuestions()
	items[0].Tipe = "esai"
	if _, errMsg := validateAdminQuestions(items); errMsg == "" {
		t.Fatal("expected error for invalid tipe")
	}
}

func TestValidateAdminQuestionsBlankPertanyaanFails(t *testing.T) {
	items := validAdminQuestions()
	items[0].Pertanyaan = "   "
	if _, errMsg := validateAdminQuestions(items); errMsg == "" {
		t.Fatal("expected error for blank pertanyaan")
	}
}

func TestValidateAdminQuestionsWrongOpsiCountFails(t *testing.T) {
	items := validAdminQuestions()
	items[0].Opsi = []string{"a", "b"}
	if _, errMsg := validateAdminQuestions(items); errMsg == "" {
		t.Fatal("expected error for wrong opsi count")
	}
}

func TestValidateAdminQuestionsOutOfRangeIndexFails(t *testing.T) {
	items := validAdminQuestions()
	items[0].Jawaban = float64(4)
	if _, errMsg := validateAdminQuestions(items); errMsg == "" {
		t.Fatal("expected error for out-of-range pilihan_ganda index")
	}
}

func TestValidateAdminQuestionsBadBenarSalahFails(t *testing.T) {
	items := validAdminQuestions()
	items[1].Jawaban = "mungkin"
	if _, errMsg := validateAdminQuestions(items); errMsg == "" {
		t.Fatal("expected error for invalid benar_salah answer")
	}
}

func TestValidateAdminQuestionsEmptyIsianFails(t *testing.T) {
	items := validAdminQuestions()
	items[2].Jawaban = "   "
	if _, errMsg := validateAdminQuestions(items); errMsg == "" {
		t.Fatal("expected error for blank isian answer")
	}
}
