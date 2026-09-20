package textmatch

import "testing"

func check(t *testing.T, siswa, benar, wantVerdict string, wantSkor float64) {
	t.Helper()
	verdict, skor := GradeIsian(siswa, benar)
	if verdict != wantVerdict || skor != wantSkor {
		t.Errorf("GradeIsian(%q, %q) = (%q, %v), want (%q, %v)", siswa, benar, verdict, skor, wantVerdict, wantSkor)
	}
}

func TestExactMatchIsBenar(t *testing.T) {
	check(t, "Jakarta", "Jakarta", "benar", 1.0)
}

func TestCaseAndWhitespaceInsensitive(t *testing.T) {
	check(t, "  jakarta  ", "Jakarta", "benar", 1.0)
}

func TestSmallTypoStillBenar(t *testing.T) {
	check(t, "jakrta", "Jakarta", "benar", 1.0)
}

func TestWrongWordIsSalah(t *testing.T) {
	check(t, "Bandung", "Jakarta", "salah", 0.0)
}

func TestCompletelyDifferentShortWordsAreSalah(t *testing.T) {
	check(t, "air", "api", "salah", 0.0)
}

func TestEmptyAnswerIsSalah(t *testing.T) {
	check(t, "", "Jakarta", "salah", 0.0)
	check(t, "   ", "Jakarta", "salah", 0.0)
}

func TestTrailingPunctuationIgnored(t *testing.T) {
	check(t, "Jakarta.", "Jakarta", "benar", 1.0)
}

func TestNumericAnswerExactMatch(t *testing.T) {
	check(t, "10", "10", "benar", 1.0)
}

func TestNumericAnswerMismatch(t *testing.T) {
	check(t, "3", "2", "salah", 0.0)
}

func TestComposedAndDecomposedUnicodeAccentsMatch(t *testing.T) {
	composed := "café"
	decomposed := "café" // e + combining acute accent
	if composed == decomposed {
		t.Fatal("expected composed and decomposed forms to differ byte-for-byte")
	}
	check(t, decomposed, composed, "benar", 1.0)
}

func TestSmartQuotesFromMobileKeyboardAreNormalized(t *testing.T) {
	check(t, "tidak’apa", "tidak'apa", "benar", 1.0)
}

func TestSmartDashesAreNormalized(t *testing.T) {
	check(t, "kereta–api", "kereta-api", "benar", 1.0)
}

func TestFullwidthDigitsNormalizedViaNFKC(t *testing.T) {
	check(t, "１０", "10", "benar", 1.0)
}

func TestNumericIndonesianThousandsSeparator(t *testing.T) {
	check(t, "1.000", "1000", "benar", 1.0)
	check(t, "1000", "1.000", "benar", 1.0)
}

func TestNumericNegativeDecimalCommaVsDot(t *testing.T) {
	check(t, "-0,2", "-0.2", "benar", 1.0)
}

func TestNumericNegativeSignIsNotLost(t *testing.T) {
	check(t, "-0,2", "0,2", "salah", 0.0)
}

func TestNumericInternationalVsIndonesianDecimalNotation(t *testing.T) {
	check(t, "0.25", "0,25", "benar", 1.0)
	check(t, "0,25", "0.25", "benar", 1.0)
}

func TestNumericFractionVsDecimal(t *testing.T) {
	check(t, "1/2", "0.5", "benar", 1.0)
	check(t, "1/2", "0,5", "benar", 1.0)
	check(t, "0.5", "1/2", "benar", 1.0)
}

func TestNumericUnsimplifiedFractionSameValue(t *testing.T) {
	check(t, "2/4", "0.5", "benar", 1.0)
}

func TestNumericDifferentValuesAreSalah(t *testing.T) {
	check(t, "1.000", "1000000", "salah", 0.0)
	check(t, "0.24", "0.25", "salah", 0.0)
	check(t, "1/3", "0.5", "salah", 0.0)
}

func TestNumericZeroDenominatorFractionDoesNotCrash(t *testing.T) {
	verdict, _ := GradeIsian("1/0", "1/0")
	if verdict != "benar" && verdict != "salah" {
		t.Errorf("unexpected verdict %q", verdict)
	}
}

func TestNumericNonNumericTextUnaffected(t *testing.T) {
	check(t, "Jakarta", "Jakarta", "benar", 1.0)
	check(t, "jakrta", "Jakarta", "benar", 1.0)
}

func TestNumericMixedNumericAndTextFallsBackToTextMatching(t *testing.T) {
	verdict, _ := GradeIsian("sekitar 100 orang", "100")
	if verdict != "salah" {
		t.Errorf("verdict = %q, want salah", verdict)
	}
}

func TestNumericRepeatedThousandsGroups(t *testing.T) {
	check(t, "1.234.567", "1234567", "benar", 1.0)
}

func TestNumericDecimalWithTwoDigitFractionNotMistakenForThousands(t *testing.T) {
	check(t, "3.14", "3.14", "benar", 1.0)
	check(t, "3.14", "3140", "salah", 0.0)
}
