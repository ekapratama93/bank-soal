package httpapi

import "testing"

func TestValidateTipeSoalNil(t *testing.T) {
	out, err := validateTipeSoal(nil)
	if err != nil || out != nil {
		t.Fatalf("got (%v, %v), want (nil, nil)", out, err)
	}
}

func TestValidateTipeSoalInvalidTypeName(t *testing.T) {
	if _, err := validateTipeSoal(map[string]int{"esai": 10}); err == nil {
		t.Fatal("expected error for invalid type name")
	}
}

func TestValidateTipeSoalTotalTooSmall(t *testing.T) {
	if _, err := validateTipeSoal(map[string]int{"pilihan_ganda": 2, "deskripsi": 1}); err == nil {
		t.Fatal("expected error: total below 5")
	}
}

func TestValidateTipeSoalTotalTooBig(t *testing.T) {
	if _, err := validateTipeSoal(map[string]int{"pilihan_ganda": 40, "isian": 20}); err == nil {
		t.Fatal("expected error: total above 50")
	}
}

func TestValidateTipeSoalNegativeCount(t *testing.T) {
	if _, err := validateTipeSoal(map[string]int{"pilihan_ganda": -1, "isian": 10}); err == nil {
		t.Fatal("expected error for negative count")
	}
}

func TestValidateTipeSoalDropsZeroCounts(t *testing.T) {
	out, err := validateTipeSoal(map[string]int{"pilihan_ganda": 10, "isian": 5, "deskripsi": 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := out["deskripsi"]; ok {
		t.Errorf("zero-count type should be dropped, got %v", out)
	}
	if out["pilihan_ganda"] != 10 || out["isian"] != 5 {
		t.Errorf("unexpected output: %v", out)
	}
}

func TestValidatePoinPerTipeNil(t *testing.T) {
	out, err := validatePoinPerTipe(nil)
	if err != nil || out != nil {
		t.Fatalf("got (%v, %v), want (nil, nil)", out, err)
	}
}

func TestValidatePoinPerTipePartial(t *testing.T) {
	out, err := validatePoinPerTipe(map[string]int{"deskripsi": 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["deskripsi"] != 10 || len(out) != 1 {
		t.Errorf("unexpected output: %v", out)
	}
}

func TestValidatePoinPerTipeInvalidKey(t *testing.T) {
	if _, err := validatePoinPerTipe(map[string]int{"esai": 10}); err == nil {
		t.Fatal("expected error for invalid type name")
	}
}

func TestValidatePoinPerTipeZeroFails(t *testing.T) {
	if _, err := validatePoinPerTipe(map[string]int{"pilihan_ganda": 0}); err == nil {
		t.Fatal("expected error: poin must be >= 1")
	}
}

func TestValidatePoinPerTipeTooBigFails(t *testing.T) {
	if _, err := validatePoinPerTipe(map[string]int{"pilihan_ganda": 101}); err == nil {
		t.Fatal("expected error: poin must be <= 100")
	}
}
