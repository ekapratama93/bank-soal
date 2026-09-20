package store

import (
	"encoding/json"
	"testing"
)

func TestQuestionJSONRoundTripPilihanGanda(t *testing.T) {
	q := Question{Tipe: "pilihan_ganda", Pertanyaan: "2+2?", Opsi: []string{"3", "4", "5", "6"}, Jawaban: 1, Pembahasan: "empat"}
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Question
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.JawabanIndex() != 1 {
		t.Errorf("JawabanIndex() = %d, want 1 (not collapsed to float64)", got.JawabanIndex())
	}
	if _, isInt := got.Jawaban.(int); !isInt {
		t.Errorf("Jawaban type = %T, want int", got.Jawaban)
	}
}

func TestQuestionJSONRoundTripTextTypes(t *testing.T) {
	for _, tipe := range []string{"benar_salah", "isian", "deskripsi"} {
		q := Question{Tipe: tipe, Pertanyaan: "q", Jawaban: "jawaban teks", Pembahasan: "p"}
		data, err := json.Marshal(q)
		if err != nil {
			t.Fatalf("marshal %s: %v", tipe, err)
		}
		var got Question
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", tipe, err)
		}
		if got.JawabanString() != "jawaban teks" {
			t.Errorf("%s: JawabanString() = %q", tipe, got.JawabanString())
		}
	}
}

func TestQuestionsArrayJSONRoundTrip(t *testing.T) {
	questions := []Question{
		{Tipe: "pilihan_ganda", Pertanyaan: "a", Opsi: []string{"1", "2", "3", "4"}, Jawaban: 2, Pembahasan: "x"},
		{Tipe: "isian", Pertanyaan: "b", Jawaban: "jkt", Pembahasan: "y"},
	}
	data, err := json.Marshal(questions)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got []Question
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 || got[0].JawabanIndex() != 2 || got[1].JawabanString() != "jkt" {
		t.Errorf("got %+v", got)
	}
}
