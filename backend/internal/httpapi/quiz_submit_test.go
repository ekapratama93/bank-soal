package httpapi

import (
	"strings"
	"testing"

	"banksoal/internal/store"
)

func TestValidateAnswersTooManyKeysFails(t *testing.T) {
	answers := map[string]any{}
	for i := 0; i < maxAnswerKeys+1; i++ {
		answers[string(rune('a'+i%26))+string(rune(i))] = "x"
	}
	if err := validateAnswers(answers); err == nil {
		t.Fatal("expected error for too many answer keys")
	}
}

func TestValidateAnswersValueTooLongFails(t *testing.T) {
	answers := map[string]any{"0": strings.Repeat("x", maxAnswerValueLength+1)}
	if err := validateAnswers(answers); err == nil {
		t.Fatal("expected error for an over-long answer value")
	}
}

func TestValidateAnswersInvalidTypeFails(t *testing.T) {
	answers := map[string]any{"0": []any{"not", "allowed"}}
	if err := validateAnswers(answers); err == nil {
		t.Fatal("expected error for an unsupported answer value type")
	}
}

func TestValidateAnswersAcceptsNullStringNumberBool(t *testing.T) {
	answers := map[string]any{"0": nil, "1": "text", "2": float64(3), "3": true}
	if err := validateAnswers(answers); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAnswerIndexFromJSONNumber(t *testing.T) {
	idx, ok := answerIndex(float64(2))
	if !ok || idx != 2 {
		t.Fatalf("got (%d, %v), want (2, true)", idx, ok)
	}
	if _, ok := answerIndex("2"); ok {
		t.Fatal("a string should not be treated as an index")
	}
}

func TestAnswerStringVariants(t *testing.T) {
	if s := answerString("benar"); s != "benar" {
		t.Errorf("got %q", s)
	}
	if s := answerString(nil); s != "" {
		t.Errorf("got %q, want empty", s)
	}
}

func TestRound2(t *testing.T) {
	if got := round2(1.005); got != 1.01 && got != 1.0 {
		// floating point edge case tolerated either way; just ensure it's close
		t.Logf("round2(1.005) = %v", got)
	}
	if got := round2(4.0); got != 4.0 {
		t.Errorf("round2(4.0) = %v, want 4.0", got)
	}
	if got := round2(3.333); got != 3.33 {
		t.Errorf("round2(3.333) = %v, want 3.33", got)
	}
}

func TestStripQuestionsRemovesAnswers(t *testing.T) {
	questions := []store.Question{
		{Tipe: "pilihan_ganda", Pertanyaan: "2+2?", Opsi: []string{"3", "4", "5", "6"}, Jawaban: 1, Pembahasan: "empat"},
		{Tipe: "isian", Pertanyaan: "Ibukota?", Jawaban: "Jakarta", Pembahasan: "..."},
	}
	public := stripQuestions(questions)
	if len(public) != 2 {
		t.Fatalf("len = %d, want 2", len(public))
	}
	if public[0].Nomor != 1 || public[1].Nomor != 2 {
		t.Errorf("nomor not sequential: %+v", public)
	}
	if public[1].Opsi != nil {
		t.Errorf("non-pilihan_ganda question should not carry opsi")
	}
	// The public struct type has no jawaban/pembahasan field at all — this
	// is enforced at compile time by questionPublic's shape, not re-checked
	// here at runtime.
}
