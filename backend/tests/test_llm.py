import asyncio
import json

import httpx
import pytest

from app.config import settings
from app.llm import (
    LLMError,
    _build_prompt,
    _chat,
    _validate_questions,
    generate_quiz,
    grade_short_answers,
)

VALID = {
    "questions": [
        {
            "tipe": "pilihan_ganda",
            "pertanyaan": "2+2?",
            "opsi": ["3", "4", "5", "6"],
            "jawaban": 1,
            "pembahasan": "2+2=4",
        },
        {
            "tipe": "benar_salah",
            "pertanyaan": "1=1?",
            "jawaban": "benar",
            "pembahasan": "ya",
        },
        {
            "tipe": "isian",
            "pertanyaan": "Ibukota RI?",
            "jawaban": "Jakarta",
            "pembahasan": "Jakarta",
        },
    ]
}

VALID_COUNTS = {"pilihan_ganda": 1, "benar_salah": 1, "isian": 1}


class TestBuildPrompt:
    def test_preserves_arabic_quotes_instruction(self):
        prompt = _build_prompt(
            "Pendidikan Agama Islam", 5, VALID_COUNTS, "Materi Hadis"
        )
        assert "PERTAHANKAN teks Arabnya" in prompt

    def test_counts_included_in_prompt(self):
        counts = {"pilihan_ganda": 10, "benar_salah": 0, "isian": 5, "deskripsi": 5}
        prompt = _build_prompt("IPA", 5, counts, None)
        assert "10 soal pilihan ganda" in prompt
        assert "5 soal isian singkat" in prompt
        assert "5 soal uraian/deskripsi" in prompt
        assert "benar/salah" not in prompt  # jumlah 0 tidak disebut


class TestValidateQuestions:
    def test_valid_passes(self):
        assert len(_validate_questions(VALID, VALID_COUNTS)) == 3

    def test_wrong_count_fails(self):
        with pytest.raises(ValueError, match="Jumlah/komposisi soal"):
            _validate_questions(VALID, {"pilihan_ganda": 1, "benar_salah": 1, "isian": 3})

    def test_wrong_composition_fails(self):
        with pytest.raises(ValueError, match="komposisi"):
            _validate_questions(VALID, {"pilihan_ganda": 2, "isian": 1})

    def test_not_a_list_fails(self):
        with pytest.raises(ValueError):
            _validate_questions({"questions": "bukan"}, VALID_COUNTS)

    def test_wrong_option_count_fails(self):
        bad = json.loads(json.dumps(VALID))
        bad["questions"][0]["opsi"] = ["a", "b", "c"]
        with pytest.raises(ValueError, match="opsi"):
            _validate_questions(bad, VALID_COUNTS)

    def test_bad_answer_index_fails(self):
        bad = json.loads(json.dumps(VALID))
        bad["questions"][0]["jawaban"] = 4
        with pytest.raises(ValueError):
            _validate_questions(bad, VALID_COUNTS)

    def test_bad_tf_answer_fails(self):
        bad = json.loads(json.dumps(VALID))
        bad["questions"][1]["jawaban"] = "mungkin"
        with pytest.raises(ValueError):
            _validate_questions(bad, VALID_COUNTS)

    def test_empty_isian_fails(self):
        bad = json.loads(json.dumps(VALID))
        bad["questions"][2]["jawaban"] = "   "
        with pytest.raises(ValueError):
            _validate_questions(bad, VALID_COUNTS)

    def test_deskripsi_valid_passes(self):
        data = json.loads(json.dumps(VALID))
        data["questions"] = [
            {
                "tipe": "deskripsi",
                "pertanyaan": "Jelaskan siklus air!",
                "jawaban": "Siklus air adalah perputaran air dari laut ke awan lalu turun sebagai hujan, berulang terus menerus.",
                "pembahasan": "Evaporasi, kondensasi, presipitasi.",
            }
        ]
        counts = {"deskripsi": 1}
        assert len(_validate_questions(data, counts)) == 1

    def test_deskripsi_empty_jawaban_fails(self):
        data = json.loads(json.dumps(VALID))
        data["questions"] = [
            {
                "tipe": "deskripsi",
                "pertanyaan": "Jelaskan siklus air!",
                "jawaban": "  ",
                "pembahasan": "...",
            }
        ]
        with pytest.raises(ValueError, match="jawaban deskripsi"):
            _validate_questions(data, {"deskripsi": 1})

    def test_balanced_math_delimiters_pass(self):
        good = json.loads(json.dumps(VALID))
        good["questions"][0]["pertanyaan"] = "Berapa $x^2$ jika $x=2$?"
        good["questions"][0]["pembahasan"] = "$$x^2 = 4$$"
        assert len(_validate_questions(good, VALID_COUNTS)) == 3

    def test_unbalanced_math_delimiters_fail(self):
        bad = json.loads(json.dumps(VALID))
        bad["questions"][0]["pertanyaan"] = "Berapa $x^2 jika x=2?"
        with pytest.raises(ValueError, match=r"delimiter \$"):
            _validate_questions(bad, VALID_COUNTS)

    def test_unbalanced_math_delimiters_in_opsi_fail(self):
        bad = json.loads(json.dumps(VALID))
        bad["questions"][0]["opsi"] = ["3", "$4", "5", "6"]
        with pytest.raises(ValueError, match=r"delimiter \$"):
            _validate_questions(bad, VALID_COUNTS)

    def test_gambar_generated_valid_passes(self):
        good = json.loads(json.dumps(VALID))
        good["questions"][0]["gambar_tipe"] = "generated"
        good["questions"][0]["gambar_prompt"] = "diagram segitiga siku-siku"
        assert len(_validate_questions(good, VALID_COUNTS)) == 3

    def test_gambar_stock_valid_passes(self):
        good = json.loads(json.dumps(VALID))
        good["questions"][0]["gambar_tipe"] = "stock"
        good["questions"][0]["gambar_cari"] = "traditional Indonesian house"
        assert len(_validate_questions(good, VALID_COUNTS)) == 3

    def test_gambar_invalid_tipe_fails(self):
        bad = json.loads(json.dumps(VALID))
        bad["questions"][0]["gambar_tipe"] = "lainnya"
        with pytest.raises(ValueError, match="gambar_tipe"):
            _validate_questions(bad, VALID_COUNTS)

    def test_gambar_generated_missing_prompt_fails(self):
        bad = json.loads(json.dumps(VALID))
        bad["questions"][0]["gambar_tipe"] = "generated"
        with pytest.raises(ValueError, match="gambar_prompt"):
            _validate_questions(bad, VALID_COUNTS)

    def test_gambar_stock_missing_query_fails(self):
        bad = json.loads(json.dumps(VALID))
        bad["questions"][0]["gambar_tipe"] = "stock"
        bad["questions"][0]["gambar_cari"] = "   "
        with pytest.raises(ValueError, match="gambar_cari"):
            _validate_questions(bad, VALID_COUNTS)


class TestGenerateQuiz:
    async def test_success(self, monkeypatch):
        async def fake_chat(messages):
            return json.dumps(VALID, ensure_ascii=False)

        monkeypatch.setattr("app.llm._chat", fake_chat)
        questions = await generate_quiz("IPA", 5, VALID_COUNTS, None)
        assert len(questions) == 3

    async def test_retries_on_bad_json_then_succeeds(self, monkeypatch):
        calls = []

        async def fake_chat(messages):
            calls.append(messages)
            if len(calls) == 1:
                return "ini bukan JSON"
            return json.dumps(VALID, ensure_ascii=False)

        monkeypatch.setattr("app.llm._chat", fake_chat)
        questions = await generate_quiz("IPA", 5, VALID_COUNTS, None)
        assert len(questions) == 3
        # Pesan retry berisi alasan kegagalan
        assert "tidak valid" in calls[1][-1]["content"]

    async def test_fails_after_two_attempts(self, monkeypatch):
        async def fake_chat(messages):
            return "bukan JSON"

        monkeypatch.setattr("app.llm._chat", fake_chat)
        with pytest.raises(LLMError):
            await generate_quiz("IPA", 5, VALID_COUNTS, None)

    async def test_retries_on_unbalanced_math_then_succeeds(self, monkeypatch):
        unbalanced = json.loads(json.dumps(VALID))
        unbalanced["questions"][0]["pertanyaan"] = "Berapa $x^2 jika x=2?"
        calls = []

        async def fake_chat(messages):
            calls.append(messages)
            if len(calls) == 1:
                return json.dumps(unbalanced, ensure_ascii=False)
            return json.dumps(VALID, ensure_ascii=False)

        monkeypatch.setattr("app.llm._chat", fake_chat)
        questions = await generate_quiz("Matematika", 5, VALID_COUNTS, None)
        assert len(questions) == 3
        assert "delimiter" in calls[1][-1]["content"]


class TestGradeShortAnswers:
    async def test_success(self, monkeypatch):
        items = [
            {"index": 0, "pertanyaan": "q", "jawaban_model": "Jakarta", "jawaban_siswa": "jakarta"}
        ]

        async def fake_chat(messages):
            return json.dumps(
                {
                    "hasil": [
                        {"index": 0, "verdict": "benar", "skor": 1.0, "umpan_balik": "Tepat"}
                    ]
                },
                ensure_ascii=False,
            )

        monkeypatch.setattr("app.llm._chat", fake_chat)
        result = await grade_short_answers(items)
        assert result[0]["verdict"] == "benar"
        assert result[0]["skor"] == 1.0

    async def test_deskripsi_items_and_koreksi_instruction_in_prompt(self, monkeypatch):
        items = [
            {
                "index": 0,
                "tipe": "deskripsi",
                "pertanyaan": "Jelaskan siklus air!",
                "jawaban_model": "Air menguap lalu turun sebagai hujan.",
                "jawaban_siswa": "Air berputar dari laut ke awan.",
            }
        ]
        captured = {}

        async def fake_chat(messages):
            captured["prompt"] = messages[1]["content"]
            return json.dumps(
                {
                    "hasil": [
                        {
                            "index": 0,
                            "verdict": "parsial",
                            "skor": 0.6,
                            "umpan_balik": "Poin evaporasi sudah tepat, kondensasi belum disebut.",
                        }
                    ]
                },
                ensure_ascii=False,
            )

        monkeypatch.setattr("app.llm._chat", fake_chat)
        result = await grade_short_answers(items)
        prompt = captured["prompt"]
        # tipe item dikirim ke AI agar koreksi sesuai jenis soal
        assert '"tipe": "deskripsi"' in prompt
        # instruksi koreksi AI untuk uraian ada di prompt
        assert "koreksi AI" in prompt
        assert result[0]["skor"] == 0.6
        assert "kondensasi" in result[0]["umpan_balik"]

    async def test_skor_clamped(self, monkeypatch):
        items = [{"index": 0, "pertanyaan": "q", "jawaban_model": "a", "jawaban_siswa": "b"}]

        async def fake_chat(messages):
            return json.dumps(
                {"hasil": [{"index": 0, "verdict": "parsial", "skor": 1.7, "umpan_balik": "x"}]}
            )

        monkeypatch.setattr("app.llm._chat", fake_chat)
        result = await grade_short_answers(items)
        assert result[0]["skor"] == 1.0

    async def test_missing_index_fails_after_retry(self, monkeypatch):
        items = [{"index": 0, "pertanyaan": "q", "jawaban_model": "a", "jawaban_siswa": "b"}]

        async def fake_chat(messages):
            return json.dumps({"hasil": []})

        monkeypatch.setattr("app.llm._chat", fake_chat)
        with pytest.raises(LLMError):
            await grade_short_answers(items)

    async def test_empty_items_no_llm_call(self, monkeypatch):
        async def fail_chat(messages):
            raise AssertionError("tidak boleh memanggil LLM")

        monkeypatch.setattr("app.llm._chat", fail_chat)
        assert await grade_short_answers([]) == {}

class TestChat:
    """_chat() sendiri — percobaan ulang untuk gangguan sesaat, bukan untuk
    galat yang pasti gagal lagi."""

    @pytest.fixture(autouse=True)
    def _fake_api_key(self, monkeypatch):
        monkeypatch.setattr(settings, "openrouter_api_key", "fake-key-for-tests")

    async def test_retries_once_on_connection_error_then_succeeds(self, monkeypatch):
        calls = {"n": 0}

        async def fake_post(self, url, headers=None, json=None):
            calls["n"] += 1
            if calls["n"] == 1:
                raise httpx.ConnectError("koneksi gagal")
            return httpx.Response(
                200,
                json={"choices": [{"message": {"content": "hasil"}}]},
                request=httpx.Request("POST", url),
            )

        monkeypatch.setattr(httpx.AsyncClient, "post", fake_post)
        monkeypatch.setattr("app.llm._CHAT_RETRY_DELAY_SECONDS", 0)

        content = await _chat([{"role": "user", "content": "x"}])
        assert content == "hasil"
        assert calls["n"] == 2

    async def test_retries_once_on_timeout_then_succeeds(self, monkeypatch):
        calls = {"n": 0}

        async def fake_post(self, url, headers=None, json=None):
            calls["n"] += 1
            if calls["n"] == 1:
                raise httpx.ReadTimeout("timeout")
            return httpx.Response(
                200,
                json={"choices": [{"message": {"content": "hasil"}}]},
                request=httpx.Request("POST", url),
            )

        monkeypatch.setattr(httpx.AsyncClient, "post", fake_post)
        monkeypatch.setattr("app.llm._CHAT_RETRY_DELAY_SECONDS", 0)

        content = await _chat([{"role": "user", "content": "x"}])
        assert content == "hasil"
        assert calls["n"] == 2

    async def test_gives_up_after_sustained_connection_failure(self, monkeypatch):
        async def fake_post(self, url, headers=None, json=None):
            raise httpx.ConnectError("selalu gagal")

        monkeypatch.setattr(httpx.AsyncClient, "post", fake_post)
        monkeypatch.setattr("app.llm._CHAT_RETRY_DELAY_SECONDS", 0)

        with pytest.raises(LLMError):
            await _chat([{"role": "user", "content": "x"}])

    async def test_retries_transient_5xx_then_succeeds(self, monkeypatch):
        calls = {"n": 0}

        async def fake_post(self, url, headers=None, json=None):
            calls["n"] += 1
            if calls["n"] == 1:
                return httpx.Response(503, text="unavailable", request=httpx.Request("POST", url))
            return httpx.Response(
                200,
                json={"choices": [{"message": {"content": "hasil"}}]},
                request=httpx.Request("POST", url),
            )

        monkeypatch.setattr(httpx.AsyncClient, "post", fake_post)
        monkeypatch.setattr("app.llm._CHAT_RETRY_DELAY_SECONDS", 0)

        content = await _chat([{"role": "user", "content": "x"}])
        assert content == "hasil"
        assert calls["n"] == 2

    async def test_does_not_retry_non_transient_4xx(self, monkeypatch):
        calls = {"n": 0}

        async def fake_post(self, url, headers=None, json=None):
            calls["n"] += 1
            return httpx.Response(401, text="invalid key", request=httpx.Request("POST", url))

        monkeypatch.setattr(httpx.AsyncClient, "post", fake_post)
        monkeypatch.setattr("app.llm._CHAT_RETRY_DELAY_SECONDS", 0)

        with pytest.raises(LLMError):
            await _chat([{"role": "user", "content": "x"}])
        # tidak dicoba ulang — 401 tidak akan berhasil walau diulang
        assert calls["n"] == 1


class TestGradeShortAnswersBatching:
    """Item deskripsi lebih dari GRADE_BATCH_SIZE dibagi ke beberapa batch
    dan dinilai PARALEL (asyncio.gather), bukan satu panggilan AI besar
    berurutan — lihat grade_short_answers() di llm.py."""

    def _item(self, i):
        return {
            "index": i,
            "tipe": "deskripsi",
            "pertanyaan": f"Soal {i}",
            "jawaban_model": f"jawaban model {i}",
            "jawaban_siswa": f"jawaban siswa {i}",
        }

    def _items_in_prompt(self, messages):
        """Baca kembali daftar item yang sungguh dikirim di prompt — jangan
        cocokkan substring pada "index": N, karena "index": 1 adalah
        substring dari "index": 10 (dan seterusnya)."""
        content = messages[-1]["content"]
        start = content.index("Item jawaban:\n") + len("Item jawaban:\n")
        end = content.index("\n\nBalas HANYA JSON valid:")
        return json.loads(content[start:end])

    def _hasil_for(self, items):
        return json.dumps(
            {
                "hasil": [
                    {
                        "index": it["index"],
                        "verdict": "benar",
                        "skor": 1.0,
                        "umpan_balik": "ok",
                    }
                    for it in items
                ]
            }
        )

    async def test_small_batch_uses_single_call(self, monkeypatch):
        items = [self._item(i) for i in range(4)]  # == GRADE_BATCH_SIZE
        calls = []

        async def fake_chat(messages):
            calls.append(messages)
            return self._hasil_for(items)

        monkeypatch.setattr("app.llm._chat", fake_chat)
        result = await grade_short_answers(items)
        assert len(calls) == 1
        assert len(result) == 4

    async def test_large_batch_splits_into_multiple_calls(self, monkeypatch):
        items = [self._item(i) for i in range(10)]  # > GRADE_BATCH_SIZE (4)
        batches_seen = []

        async def fake_chat(messages):
            batch_items = self._items_in_prompt(messages)
            batches_seen.append(batch_items)
            return self._hasil_for(batch_items)

        monkeypatch.setattr("app.llm._chat", fake_chat)
        result = await grade_short_answers(items)

        # 10 item / 4 per batch -> 3 panggilan (4, 4, 2), bukan 1 panggilan besar.
        assert len(batches_seen) == 3
        assert sorted(len(b) for b in batches_seen) == [2, 4, 4]
        # Semua index tetap lengkap setelah digabung dari beberapa batch.
        assert set(result.keys()) == {i for i in range(10)}

    async def test_batches_run_concurrently_not_sequentially(self, monkeypatch):
        items = [self._item(i) for i in range(8)]  # -> 2 batch
        in_flight = 0
        max_in_flight = 0

        async def fake_chat(messages):
            nonlocal in_flight, max_in_flight
            in_flight += 1
            max_in_flight = max(max_in_flight, in_flight)
            await asyncio.sleep(0.05)  # beri waktu supaya batch lain sempat mulai
            batch_items = self._items_in_prompt(messages)
            in_flight -= 1
            return self._hasil_for(batch_items)

        monkeypatch.setattr("app.llm._chat", fake_chat)
        await grade_short_answers(items)

        # Kalau berurutan, max_in_flight akan selalu 1. Paralel -> keduanya
        # sempat berjalan bersamaan.
        assert max_in_flight == 2

    async def test_one_failing_batch_raises_llm_error(self, monkeypatch):
        items = [self._item(i) for i in range(8)]  # -> 2 batch

        async def fake_chat(messages):
            batch_items = self._items_in_prompt(messages)
            if any(it["index"] == 0 for it in batch_items):
                raise LLMError("batch pertama gagal")
            return self._hasil_for(batch_items)

        monkeypatch.setattr("app.llm._chat", fake_chat)
        with pytest.raises(LLMError):
            await grade_short_answers(items)

    async def test_merged_results_preserve_correct_scores_per_item(self, monkeypatch):
        items = [self._item(i) for i in range(6)]  # -> 2 batch (4, 2)

        async def fake_chat(messages):
            batch_items = self._items_in_prompt(messages)
            return json.dumps(
                {
                    "hasil": [
                        {
                            "index": it["index"],
                            "verdict": "parsial",
                            "skor": it["index"] / 10,
                            "umpan_balik": f"catatan {it['index']}",
                        }
                        for it in batch_items
                    ]
                }
            )

        monkeypatch.setattr("app.llm._chat", fake_chat)
        result = await grade_short_answers(items)
        for i in range(6):
            assert result[i]["skor"] == i / 10
            assert result[i]["umpan_balik"] == f"catatan {i}"
