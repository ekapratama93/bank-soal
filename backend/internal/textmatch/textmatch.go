// Package textmatch grades short-answer ("isian") questions locally via
// normalized string/number comparison, without calling an LLM. Ported
// line-for-line from the Python backend's app/text_match.py — see that
// file's docstring for the full rationale.
package textmatch

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// MatchThreshold is the minimum SequenceMatcher-style similarity ratio
// (0-1) for a text answer to be considered correct — loose enough to
// tolerate 1-2 typos, tight enough to reject genuinely different answers.
const MatchThreshold = 0.85

var (
	fractionRE    = regexp.MustCompile(`^-?\d+\s*/\s*\d+$`)
	numericRE     = regexp.MustCompile(`^-?[\d.,]+$`)
	punctuationRE = regexp.MustCompile("[.,!?;:'\"()\\[\\]{}‘’“”\\-–—]")
	whitespaceRE  = regexp.MustCompile(`\s+`)
	fold          = cases.Fold()
)

func normalize(text string) string {
	text = norm.NFKC.String(text)
	text = strings.TrimSpace(text)
	text = fold.String(text)
	text = punctuationRE.ReplaceAllString(text, "")
	text = whitespaceRE.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// tryParseNumber tries to read text as a number — a simple fraction
// ("1/2") or a decimal using either Indonesian (dot=thousands,
// comma=decimal) or international (dot=decimal) notation. Returns
// (value, true) or (0, false) if text isn't purely numeric.
//
// This must run BEFORE normalize()/string matching, because normalize()
// strips '.'/','/'-' as ordinary punctuation — correct for text, but wrong
// for numbers ("-0,2" would lose both its sign and its decimal point).
func tryParseNumber(text string) (float64, bool) {
	text = strings.TrimSpace(norm.NFKC.String(text))
	if text == "" {
		return 0, false
	}

	if fractionRE.MatchString(text) {
		parts := strings.SplitN(text, "/", 2)
		numStr := strings.TrimSpace(parts[0])
		denStr := strings.TrimSpace(parts[1])
		num, err1 := strconv.Atoi(numStr)
		den, err2 := strconv.Atoi(denStr)
		if err1 != nil || err2 != nil || den == 0 {
			return 0, false
		}
		return float64(num) / float64(den), true
	}

	if !numericRE.MatchString(text) {
		return 0, false // not purely numeric (e.g. contains letters) — text path
	}

	hasComma := strings.Contains(text, ",")
	hasDot := strings.Contains(text, ".")
	var normalized string
	switch {
	case hasComma && hasDot:
		// Whichever separator appears LAST is the decimal point; the other is
		// a thousands separator. "1.234,56" (ID) -> comma last -> decimal.
		// "1,234.56" (US) -> dot last -> decimal.
		if strings.LastIndex(text, ",") > strings.LastIndex(text, ".") {
			normalized = strings.ReplaceAll(text, ".", "")
			normalized = strings.ReplaceAll(normalized, ",", ".")
		} else {
			normalized = strings.ReplaceAll(text, ",", "")
		}
	case hasComma:
		// Comma only: Indonesian convention uses '.' for thousands, so a
		// lone comma is almost always a decimal point ("-0,2" -> -0.2).
		normalized = strings.ReplaceAll(text, ",", ".")
	case hasDot:
		// Dot only: ambiguous between thousands ("1.000" Indonesian) and
		// decimal ("0.25" international). Heuristic: starts with "0." or
		// "-0." (nobody writes a thousands separator starting with a leading
		// zero), or the part after the dot isn't exactly 3 digits (a
		// thousands group), or there's more than one dot — treat as decimal;
		// otherwise ("1.000", "12.345") treat as thousands.
		lastDot := strings.LastIndex(text, ".")
		lastGroup := text[lastDot+1:]
		looksDecimal := strings.HasPrefix(text, "0.") || strings.HasPrefix(text, "-0.") || len(lastGroup) != 3
		if looksDecimal {
			normalized = text
		} else {
			normalized = strings.ReplaceAll(text, ".", "")
		}
	default:
		normalized = text
	}

	v, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func isClose(a, b, relTol, absTol float64) bool {
	diff := math.Abs(a - b)
	return diff <= math.Max(relTol*math.Max(math.Abs(a), math.Abs(b)), absTol)
}

// GradeIsian compares a student's short-answer response against the model
// answer. It returns (verdict, skor); there is no partial credit here —
// "isian" is a short factual recall, not something reasonably scored as
// "half right" purely from string similarity (unlike "deskripsi").
func GradeIsian(jawabanSiswa, jawabanBenar string) (string, float64) {
	siswaNum, siswaIsNum := tryParseNumber(jawabanSiswa)
	benarNum, benarIsNum := tryParseNumber(jawabanBenar)
	if siswaIsNum && benarIsNum {
		// Numbers compare by value, not string — "1/2", "0.5", and "0,5"
		// must all be equal. The tolerance is only for floating-point
		// precision (e.g. a fraction's division), NOT for rounding by the
		// student — 0.24 is still wrong if the correct answer is 0.25.
		if isClose(siswaNum, benarNum, 1e-9, 1e-9) {
			return "benar", 1.0
		}
		return "salah", 0.0
	}

	a := normalize(jawabanSiswa)
	b := normalize(jawabanBenar)
	if a == "" {
		return "salah", 0.0
	}
	if a == b {
		return "benar", 1.0
	}
	if ratio(a, b) >= MatchThreshold {
		return "benar", 1.0
	}
	return "salah", 0.0
}
