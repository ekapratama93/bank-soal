"""Isian dinilai lokal (perbandingan string), tanpa AI — lihat
app/text_match.py dan submit() di app/routers/quiz.py."""

from app.text_match import grade_isian


def test_exact_match_is_benar():
    assert grade_isian("Jakarta", "Jakarta") == ("benar", 1.0)


def test_case_and_whitespace_insensitive():
    assert grade_isian("  jakarta  ", "Jakarta") == ("benar", 1.0)


def test_small_typo_still_benar():
    """Toleransi kecil pada salah ketik/ejaan — sesuai desain prompt AI yang
    lama, sekarang ditegakkan secara lokal."""
    assert grade_isian("jakrta", "Jakarta") == ("benar", 1.0)


def test_wrong_word_is_salah():
    assert grade_isian("Bandung", "Jakarta") == ("salah", 0.0)


def test_completely_different_short_words_are_salah():
    assert grade_isian("air", "api") == ("salah", 0.0)


def test_empty_answer_is_salah():
    assert grade_isian("", "Jakarta") == ("salah", 0.0)
    assert grade_isian("   ", "Jakarta") == ("salah", 0.0)


def test_trailing_punctuation_ignored():
    assert grade_isian("Jakarta.", "Jakarta") == ("benar", 1.0)


def test_numeric_answer_exact_match():
    assert grade_isian("10", "10") == ("benar", 1.0)


def test_numeric_answer_mismatch():
    assert grade_isian("3", "2") == ("salah", 0.0)


def test_composed_and_decomposed_unicode_accents_match():
    """'é' sebagai satu kode vs 'e' + aksen terpisah — tampak identik tapi
    beda secara byte tanpa normalisasi Unicode (NFKC)."""
    composed = "café"
    decomposed = "café"  # e + combining acute accent (U+0301)
    assert composed != decomposed  # buktikan memang beda representasi
    assert grade_isian(decomposed, composed) == ("benar", 1.0)


def test_smart_quotes_from_mobile_keyboard_are_normalized():
    assert grade_isian("tidak’apa", "tidak'apa") == ("benar", 1.0)


def test_smart_dashes_are_normalized():
    assert grade_isian("kereta–api", "kereta-api") == ("benar", 1.0)


def test_fullwidth_digits_normalized_via_nfkc():
    # Sebagian keyboard mobile (mode input tertentu) bisa menghasilkan
    # digit lebar-penuh yang tampak sama tapi kode Unicode-nya berbeda.
    assert grade_isian("１０", "10") == ("benar", 1.0)


class TestNumericAnswers:
    """Angka dibandingkan sebagai nilai, bukan string — sebelum perbaikan ini,
    _normalize() membuang titik/koma/minus sebagai tanda baca biasa dan
    merusak makna angka ("-0,2" -> "02", tanda minus & desimal hilang)."""

    def test_indonesian_thousands_separator(self):
        assert grade_isian("1.000", "1000") == ("benar", 1.0)
        assert grade_isian("1000", "1.000") == ("benar", 1.0)

    def test_negative_decimal_comma_vs_dot(self):
        assert grade_isian("-0,2", "-0.2") == ("benar", 1.0)

    def test_negative_sign_is_not_lost(self):
        """Bug yang ditemukan: tanda minus dulu ikut terbuang sebagai
        'tanda baca', membuat -0,2 dan 0,2 tak terbedakan."""
        assert grade_isian("-0,2", "0,2") == ("salah", 0.0)

    def test_international_vs_indonesian_decimal_notation(self):
        assert grade_isian("0.25", "0,25") == ("benar", 1.0)
        assert grade_isian("0,25", "0.25") == ("benar", 1.0)

    def test_fraction_vs_decimal(self):
        assert grade_isian("1/2", "0.5") == ("benar", 1.0)
        assert grade_isian("1/2", "0,5") == ("benar", 1.0)
        assert grade_isian("0.5", "1/2") == ("benar", 1.0)

    def test_unsimplified_fraction_same_value(self):
        assert grade_isian("2/4", "0.5") == ("benar", 1.0)

    def test_different_values_are_salah(self):
        assert grade_isian("1.000", "1000000") == ("salah", 0.0)
        assert grade_isian("0.24", "0.25") == ("salah", 0.0)
        assert grade_isian("1/3", "0.5") == ("salah", 0.0)

    def test_zero_denominator_fraction_does_not_crash(self):
        # Bukan angka valid -> jatuh ke jalur teks, bukan ZeroDivisionError.
        verdict, skor = grade_isian("1/0", "1/0")
        assert verdict in ("benar", "salah")

    def test_non_numeric_text_unaffected(self):
        assert grade_isian("Jakarta", "Jakarta") == ("benar", 1.0)
        assert grade_isian("jakrta", "Jakarta") == ("benar", 1.0)

    def test_mixed_numeric_and_text_falls_back_to_text_matching(self):
        # Bukan murni angka (ada kata) -> tetap lewat perbandingan string.
        verdict, _ = grade_isian("sekitar 100 orang", "100")
        assert verdict == "salah"

    def test_repeated_thousands_groups(self):
        assert grade_isian("1.234.567", "1234567") == ("benar", 1.0)

    def test_decimal_with_two_digit_fraction_not_mistaken_for_thousands(self):
        assert grade_isian("3.14", "3.14") == ("benar", 1.0)
        assert grade_isian("3.14", "3140") == ("salah", 0.0)
