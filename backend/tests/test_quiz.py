import copy
from datetime import datetime, timedelta, timezone

import httpx

import pytest
from fastapi.testclient import TestClient

from app.grade_config import get_config
from app.routers import quiz as quiz_router

QUESTIONS = [
    {
        "tipe": "pilihan_ganda",
        "pertanyaan": "Hasil 2 + 2 adalah?",
        "opsi": ["3", "4", "5", "6"],
        "jawaban": 1,
        "pembahasan": "2 + 2 = 4",
    },
    {
        "tipe": "benar_salah",
        "pertanyaan": "Air mendidih pada 100 derajat Celsius di tekanan 1 atm.",
        "jawaban": "benar",
        "pembahasan": "Titik didih air pada tekanan 1 atm adalah 100 derajat Celsius.",
    },
    {
        "tipe": "isian",
        "pertanyaan": "Ibukota Republik Indonesia adalah?",
        "jawaban": "Jakarta",
        "pembahasan": "Ibukota RI adalah Jakarta.",
    },
]

EXAM_TYPE_ID = "et-1"


def exam_type_fixture(sb):
    sb.tables.setdefault("exam_types", []).append(
        {"id": EXAM_TYPE_ID, "name": "Ujian Harian", "jumlah_soal": None, "durasi_menit": None}
    )


def insert_quiz(sb, quiz_id, started=False, expires_in_minutes=60, exam_type_id=EXAM_TYPE_ID, subject="Matematika", subject_id="sub-1"):
    row = {
        "id": quiz_id,
        "subject": subject,
        "subject_id": subject_id,
        "grade": 3,
        "exam_type_id": exam_type_id,
        "questions": copy.deepcopy(QUESTIONS),
        "batch_id": "batch-1",
        "durasi_menit": 60,
        "started": started,
        "expires_at": (datetime.now(timezone.utc) + timedelta(minutes=expires_in_minutes)).isoformat()
        if started
        else None,
    }
    sb.tables.setdefault("quizzes", []).append(row)
    return row


def make_generate_ok(monkeypatch):
    async def fake_generate(subject, grade, counts, material):
        return copy.deepcopy(QUESTIONS)

    monkeypatch.setattr(quiz_router, "generate_quiz", fake_generate)


def make_generate_fail(monkeypatch, message="gagal"):
    async def fake_generate(subject, grade, counts, material):
        raise quiz_router.LLMError(message)

    monkeypatch.setattr(quiz_router, "generate_quiz", fake_generate)


def make_grade_ok(monkeypatch, verdict="parsial", skor=0.5):
    async def fake_grade(items):
        return {i["index"]: {"verdict": verdict, "skor": skor, "umpan_balik": "Hampir tepat"} for i in items}

    monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)


class TestGradeConfig:
    def test_kelas_1_2(self):
        assert get_config(1) == {"jumlah_soal": 20, "durasi_menit": 60}
        assert get_config(2) == {"jumlah_soal": 20, "durasi_menit": 60}

    def test_kelas_3_4(self):
        assert get_config(3) == {"jumlah_soal": 25, "durasi_menit": 60}

    def test_kelas_5_6(self):
        assert get_config(5) == {"jumlah_soal": 30, "durasi_menit": 60}

    def test_kelas_7_keatas_default(self):
        assert get_config(7) == {"jumlah_soal": 30, "durasi_menit": 90}
        assert get_config(12) == {"jumlah_soal": 30, "durasi_menit": 90}


class TestGenerate:
    """Admin membuat batch paket via POST /api/quiz/generate."""

    def _generate(self, client, admin_headers, subject="Matematika", grade=3, jumlah_paket=3):
        return client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={
                "subject": subject,
                "grade": grade,
                "exam_type_id": EXAM_TYPE_ID,
                "jumlah_paket": jumlah_paket,
            },
        )

    def test_requires_admin(self, client, sb):
        exam_type_fixture(sb)
        res = client.post(
            "/api/quiz/generate",
            json={"subject": "Matematika", "grade": 3, "exam_type_id": EXAM_TYPE_ID},
        )
        assert res.status_code == 401

    def test_generate_creates_batch(self, client, sb, admin_auth, admin_headers, monkeypatch):
        exam_type_fixture(sb)
        make_generate_ok(monkeypatch)
        res = self._generate(client, admin_headers)
        assert res.status_code == 200
        body = res.json()
        assert body["generated"] == 3
        assert len(body["quiz_ids"]) == 3
        assert len(sb.tables["quizzes"]) == 3
        batch_ids = {q["batch_id"] for q in sb.tables["quizzes"]}
        assert len(batch_ids) == 1
        assert all(q["started"] is False and q["expires_at"] is None for q in sb.tables["quizzes"])

    def test_jumlah_paket_from_request_overrides_default(
        self, client, sb, admin_auth, admin_headers, monkeypatch
    ):
        exam_type_fixture(sb)
        sb.tables["materials"] = [
            {
                "subject": "Matematika",
                "subject_id": "sub-1",
                "grade": 3,
                "exam_type_id": EXAM_TYPE_ID,
                "title": "A",
                "content": "isi",
            },
        ]
        make_generate_ok(monkeypatch)
        res = self._generate(client, admin_headers, jumlah_paket=5)
        assert res.status_code == 200
        assert res.json()["generated"] == 5
        assert len(sb.tables["quizzes"]) == 5
        assert all(q["batch_id"] for q in sb.tables["quizzes"])
        assert all(q["started"] is False and q["expires_at"] is None for q in sb.tables["quizzes"])

    def test_jumlah_paket_out_of_range_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        res = self._generate(client, admin_headers, jumlah_paket=6)
        assert res.status_code == 422

    def test_material_context_passed_to_llm(self, client, sb, admin_auth, admin_headers, monkeypatch):
        exam_type_fixture(sb)
        sb.tables["materials"] = [
            {
                "subject": "Matematika",
                "subject_id": "sub-1",
                "grade": 3,
                "exam_type_id": EXAM_TYPE_ID,
                "title": "Perkalian",
                "content": "Perkalian adalah penjumlahan berulang.",
            }
        ]

        async def fake_generate(subject, grade, counts, material):
            assert material is not None
            assert "Perkalian adalah penjumlahan berulang." in material
            return copy.deepcopy(QUESTIONS)

        monkeypatch.setattr(quiz_router, "generate_quiz", fake_generate)
        res = self._generate(client, admin_headers)
        assert res.status_code == 200

    def test_generate_stores_full_answers(self, client, sb, admin_auth, admin_headers, monkeypatch):
        exam_type_fixture(sb)

        async def fake_generate(subject, grade, counts, material):
            assert subject == "Matematika"
            assert grade == 3
            assert sum(counts.values()) == 25
            # komposisi otomatis default: 40% PG, 20% B/S, sisanya isian
            assert counts["pilihan_ganda"] == 10
            assert counts["benar_salah"] == 5
            assert counts["isian"] == 10
            return copy.deepcopy(QUESTIONS)

        monkeypatch.setattr(quiz_router, "generate_quiz", fake_generate)
        res = self._generate(client, admin_headers)
        assert res.status_code == 200
        body = res.json()
        assert body["generated"] == 3

        stored = sb.tables["quizzes"][0]
        assert stored["questions"][0]["jawaban"] == 1
        assert stored["exam_type_id"] == EXAM_TYPE_ID

    def test_generate_with_subject_id(self, client, sb, admin_auth, admin_headers, monkeypatch):
        """Payload baru: subject_id menang — LLM tetap menerima NAMA mapel
        (prompt AI butuh nama, bukan id), dan baris tersimpan menulis keduanya."""
        exam_type_fixture(sb)

        async def fake_generate(subject, grade, counts, material):
            assert subject == "Matematika"  # nama, bukan "sub-1"
            return copy.deepcopy(QUESTIONS)

        monkeypatch.setattr(quiz_router, "generate_quiz", fake_generate)
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"subject_id": "sub-1", "grade": 3, "exam_type_id": EXAM_TYPE_ID},
        )
        assert res.status_code == 200
        assert res.json()["generated"] == 3
        for q in sb.tables["quizzes"]:
            assert q["subject_id"] == "sub-1"
            assert q["subject"] == "Matematika"

    def test_generate_both_fields_subject_id_wins(self, client, sb, admin_auth, admin_headers, monkeypatch):
        exam_type_fixture(sb)

        async def fake_generate(subject, grade, counts, material):
            assert subject == "Matematika"
            return copy.deepcopy(QUESTIONS)

        monkeypatch.setattr(quiz_router, "generate_quiz", fake_generate)
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"subject": "IPA", "subject_id": "sub-1", "grade": 3, "exam_type_id": EXAM_TYPE_ID},
        )
        assert res.status_code == 200
        assert sb.tables["quizzes"][0]["subject_id"] == "sub-1"

    def test_generate_neither_subject_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"grade": 3, "exam_type_id": EXAM_TYPE_ID},
        )
        assert res.status_code == 422
        assert "subject_id" in res.json()["detail"]

    def test_generate_unknown_subject_id_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"subject_id": "sub-tidak-ada", "grade": 3, "exam_type_id": EXAM_TYPE_ID},
        )
        assert res.status_code == 422

    def test_generate_legacy_subject_name_still_works(self, client, sb, admin_auth, admin_headers, monkeypatch):
        """Kompatibilitas masa transisi: payload legacy (nama) masih diterima
        dan baris yang dihasilkan membawa subject_id + subject (nama)."""
        exam_type_fixture(sb)
        make_generate_ok(monkeypatch)
        res = self._generate(client, admin_headers)  # subject: "Matematika"
        assert res.status_code == 200
        for q in sb.tables["quizzes"]:
            assert q["subject_id"] == "sub-1"
            assert q["subject"] == "Matematika"

    def test_exam_type_overrides_grade_config(self, client, sb, admin_auth, admin_headers, monkeypatch):
        exam_type_fixture(sb)
        sb.tables["exam_types"][0]["jumlah_soal"] = 10
        sb.tables["exam_types"][0]["durasi_menit"] = 30

        async def fake_generate(subject, grade, counts, material):
            assert sum(counts.values()) == 10
            return copy.deepcopy(QUESTIONS)

        monkeypatch.setattr(quiz_router, "generate_quiz", fake_generate)
        res = self._generate(client, admin_headers)
        assert res.status_code == 200
        assert sb.tables["quizzes"][0]["durasi_menit"] == 30

    def test_exam_type_tipe_soal_overrides_auto_split(
        self, client, sb, admin_auth, admin_headers, monkeypatch
    ):
        """Komposisi tipe soal dari tipe ujian dipakai apa adanya; total dihitung darinya."""
        exam_type_fixture(sb)
        sb.tables["exam_types"][0]["tipe_soal"] = {
            "pilihan_ganda": 10,
            "benar_salah": 0,
            "isian": 5,
            "deskripsi": 5,
        }

        async def fake_generate(subject, grade, counts, material):
            assert counts == {"pilihan_ganda": 10, "isian": 5, "deskripsi": 5}
            return copy.deepcopy(QUESTIONS)

        monkeypatch.setattr(quiz_router, "generate_quiz", fake_generate)
        res = self._generate(client, admin_headers)
        assert res.status_code == 200
        assert len(sb.tables["quizzes"]) == 3

    def test_generate_invalid_subject(self, client, sb, admin_auth, admin_headers):
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"subject": "Mama", "grade": 3, "exam_type_id": "x"},
        )
        assert res.status_code == 422

    def test_generate_invalid_grade(self, client, sb, admin_auth, admin_headers):
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"subject": "IPA", "grade": 13, "exam_type_id": "x"},
        )
        assert res.status_code == 422

    def test_generate_unknown_exam_type(self, client, sb, admin_auth, admin_headers):
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"subject": "IPA", "grade": 3, "exam_type_id": "tidak-ada"},
        )
        assert res.status_code == 422

    def test_generate_llm_error_returns_502(self, client, sb, admin_auth, admin_headers, monkeypatch):
        exam_type_fixture(sb)
        make_generate_fail(monkeypatch, "Gagal membuat soal setelah beberapa percobaan.")
        res = self._generate(client, admin_headers)
        assert res.status_code == 502
        assert "Gagal" in res.json()["detail"]


class TestQuizRequest:
    """Siswa menerima paket dari pool via POST /api/quiz/request."""

    def _request(self, client, served_ids=None):
        return client.post(
            "/api/quiz/request",
            json={
                "subject": "Matematika",
                "grade": 3,
                "exam_type_id": EXAM_TYPE_ID,
                "served_ids": served_ids or [],
            },
        )

    def test_serves_from_pool_no_llm_call(self, client, sb, monkeypatch):
        exam_type_fixture(sb)
        make_generate_fail(monkeypatch)  # LLM tidak boleh dipanggil
        insert_quiz(sb, "quiz-1")

        res = self._request(client)
        assert res.status_code == 200
        body = res.json()
        assert body["quiz_id"] == "quiz-1"
        assert body["repeat"] is False
        assert body["exam_type"] == "Ujian Harian"

    def test_respects_served_ids(self, client, sb, monkeypatch):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-1")
        insert_quiz(sb, "quiz-2")

        res = self._request(client, served_ids=["quiz-1"])
        assert res.status_code == 200
        assert res.json()["quiz_id"] == "quiz-2"

    def test_repeat_when_all_served(self, client, sb, monkeypatch):
        exam_type_fixture(sb)
        make_generate_fail(monkeypatch)
        insert_quiz(sb, "quiz-1")

        res = self._request(client, served_ids=["quiz-1"])
        assert res.status_code == 200
        body = res.json()
        assert body["quiz_id"] == "quiz-1"
        assert body["repeat"] is True

    def test_no_pool_404(self, client, sb):
        exam_type_fixture(sb)
        res = self._request(client)
        assert res.status_code == 404
        assert "Belum ada paket" in res.json()["detail"]

    def test_quiz_started_by_other_client_still_served(self, client, sb):
        """Paket yang sudah dibuka siswa lain tetap harus bisa didapat siswa baru —
        satu paket boleh dipakai banyak siswa."""
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-started", started=True)
        res = self._request(client)
        assert res.status_code == 200
        assert res.json()["quiz_id"] == "quiz-started"

    def test_own_attempted_quiz_not_served_again(self, client, sb):
        """Siswa yang sama tidak boleh mendapat paket yang sudah pernah dia buka."""
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-1")
        insert_quiz(sb, "quiz-2")
        opened = client.get("/api/quiz/quiz-1")
        assert opened.status_code == 200
        res = self._request(client)
        assert res.json()["quiz_id"] == "quiz-2"

    def test_unknown_exam_type_422(self, client, sb):
        res = self._request(client)
        assert res.status_code == 422

    def test_request_with_subject_id(self, client, sb, monkeypatch):
        """Payload baru: filter pool lewat subject_id."""
        exam_type_fixture(sb)
        make_generate_fail(monkeypatch)
        insert_quiz(sb, "quiz-1", subject="IPA", subject_id="sub-4")

        res = client.post(
            "/api/quiz/request",
            json={"subject_id": "sub-4", "grade": 3, "exam_type_id": EXAM_TYPE_ID},
        )
        assert res.status_code == 200
        body = res.json()
        assert body["quiz_id"] == "quiz-1"
        assert body["subject_id"] == "sub-4"
        assert body["subject"] == "IPA"

    def test_request_neither_subject_422(self, client, sb):
        exam_type_fixture(sb)
        res = client.post(
            "/api/quiz/request",
            json={"grade": 3, "exam_type_id": EXAM_TYPE_ID},
        )
        assert res.status_code == 422


class TestGetQuiz:
    def test_first_get_starts_timer(self, client, sb):
        exam_type_fixture(sb)
        quiz = insert_quiz(sb, "quiz-123", started=False)
        assert quiz["expires_at"] is None

        res = client.get("/api/quiz/quiz-123")
        assert res.status_code == 200
        body = res.json()
        assert body["exam_type"] == "Ujian Harian"
        assert body["durasi_menit"] == 60
        assert body["expires_at"]

        assert sb.tables["quizzes"][0]["started"] is True
        attempt = sb.tables["attempts"][0]
        assert attempt["expires_at"] == body["expires_at"]
        expires = datetime.fromisoformat(attempt["expires_at"])
        remaining = expires - datetime.now(timezone.utc)
        assert 59 < remaining.total_seconds() / 60 <= 60

    def test_second_get_returns_same_expires(self, client, sb):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123", started=False)
        first = client.get("/api/quiz/quiz-123").json()
        second = client.get("/api/quiz/quiz-123").json()
        assert first["expires_at"] == second["expires_at"]

    def test_get_quiz_started_by_other_client_gets_own_fresh_timer(self, client, sb):
        """quiz.started=True secara global (siswa lain sudah membuka) — siswa
        baru tetap mendapat timernya sendiri yang baru, bukan waktu yang lewat."""
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123", started=True)
        res = client.get("/api/quiz/quiz-123")
        assert res.status_code == 200
        expires = datetime.fromisoformat(res.json()["expires_at"])
        remaining = expires - datetime.now(timezone.utc)
        assert remaining.total_seconds() > 0

    def test_get_quiz_resumes_own_existing_attempt(self, client, sb):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123", started=False)
        client.get("/api/quiz/quiz-123")
        past = datetime.now(timezone.utc) - timedelta(minutes=10)
        sb.tables["attempts"][0]["expires_at"] = past.isoformat()
        res = client.get("/api/quiz/quiz-123")
        assert res.json()["expires_at"] == past.isoformat()

    def test_get_quiz_strips_answers(self, client, sb):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.get("/api/quiz/quiz-123")
        for q in res.json()["questions"]:
            assert "jawaban" not in q
            assert "pembahasan" not in q

    def test_get_quiz_not_found(self, client, sb):
        res = client.get("/api/quiz/tidak-ada")
        assert res.status_code == 404

    def test_concurrent_attempt_creation_does_not_500(self, sb):
        """Dua request nyaris bersamaan untuk (quiz, client) yang sama bisa
        lolos _get_attempt() yang sama-sama kosong, lalu berlomba insert.
        Yang kalah harus mendapat baris si pemenang, bukan exception mentah
        dari unique-violation (idx_attempts_quiz_client)."""
        exam_type_fixture(sb)
        quiz = insert_quiz(sb, "quiz-123", started=False)

        first = quiz_router._create_attempt(sb, quiz, "client-a")
        second = quiz_router._create_attempt(sb, quiz, "client-a")

        assert second["id"] == first["id"]
        assert len(sb.tables["attempts"]) == 1

    def test_concurrent_attempt_creation_different_clients_both_succeed(self, sb):
        exam_type_fixture(sb)
        quiz = insert_quiz(sb, "quiz-123", started=False)

        first = quiz_router._create_attempt(sb, quiz, "client-a")
        second = quiz_router._create_attempt(sb, quiz, "client-b")

        assert first["id"] != second["id"]
        assert len(sb.tables["attempts"]) == 2


class TestSubmit:
    def _open(self, client, quiz_id):
        res = client.get(f"/api/quiz/{quiz_id}")
        assert res.status_code == 200
        return res

    def test_submit_autograde_pg_bs_and_local_isian(self, client, sb, monkeypatch):
        """Isian dinilai lokal (text_match.py) — quiz tanpa soal deskripsi
        tidak boleh memanggil AI sama sekali. Ini yang membuat pengumpulan
        cepat: sebelumnya SETIAP isian ikut satu panggilan AI bersama."""
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        self._open(client, "quiz-123")

        async def fail_if_called(items):
            raise AssertionError(f"AI tidak boleh dipanggil untuk isian: {items}")

        monkeypatch.setattr(quiz_router, "grade_short_answers", fail_if_called)

        res = client.post(
            "/api/quiz/quiz-123/submit",
            # "jakarta" (huruf kecil) — toleransi kecil ejaan, tetap "benar".
            json={"answers": {"0": 1, "1": "benar", "2": "jakarta"}},
        )
        assert res.status_code == 200
        body = res.json()
        assert body["nilai"] == 100
        assert body["expired"] is False
        assert body["exam_type"] == "Ujian Harian"
        verdicts = [pq["verdict"] for pq in body["per_question"]]
        assert verdicts == ["benar", "benar", "benar"]

    def test_submit_isian_wrong_answer_graded_locally(self, client, sb, monkeypatch):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        self._open(client, "quiz-123")

        async def fail_if_called(items):
            raise AssertionError("AI tidak boleh dipanggil untuk isian")

        monkeypatch.setattr(quiz_router, "grade_short_answers", fail_if_called)
        res = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 1, "1": "benar", "2": "Bandung"}},
        )
        assert res.status_code == 200
        body = res.json()
        pq_isian = body["per_question"][2]
        assert pq_isian["verdict"] == "salah"
        assert pq_isian["skor"] == 0.0
        assert pq_isian["jawaban_benar"] == "Jakarta"
        assert sb.tables["attempts"][0]["quiz_id"] == "quiz-123"

    def test_submit_deskripsi_graded_by_ai(self, client, sb, monkeypatch):
        exam_type_fixture(sb)
        quiz = insert_quiz(sb, "quiz-desc")
        quiz["questions"] = [
            {
                "tipe": "deskripsi",
                "pertanyaan": "Jelaskan siklus air!",
                "jawaban": "Air menguap, mengembun jadi awan, lalu turun sebagai hujan.",
                "pembahasan": "Evaporasi, kondensasi, presipitasi.",
            }
        ]
        self._open(client, "quiz-desc")

        async def fake_grade(items):
            assert len(items) == 1
            assert items[0]["tipe"] == "deskripsi"
            assert items[0]["jawaban_model"].startswith("Air menguap")
            return {0: {"verdict": "parsial", "skor": 0.7, "umpan_balik": "Kurang lengkap"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post(
            "/api/quiz/quiz-desc/submit",
            json={"answers": {"0": "Air berputar dari laut ke awan lalu hujan"}},
        )
        assert res.status_code == 200
        body = res.json()
        assert body["nilai"] == 70
        assert body["per_question"][0]["tipe"] == "deskripsi"
        assert body["per_question"][0]["jawaban_benar"].startswith("Air menguap")
        # tanpa konfigurasi poin: default 1 poin per soal
        assert body["per_question"][0]["poin"] == 0.7
        assert body["per_question"][0]["poin_maks"] == 1
        assert body["poin"] == 0.7
        assert body["poin_maks"] == 1

    def test_submit_poin_weighted(self, client, sb, monkeypatch):
        """Nilai dihitung dari poin per tipe: PG 2 poin, isian 5, deskripsi 10."""
        exam_type_fixture(sb)
        sb.tables["exam_types"][0]["poin_per_tipe"] = {
            "pilihan_ganda": 2,
            "isian": 5,
            "deskripsi": 10,
        }
        quiz = insert_quiz(sb, "quiz-poin")
        quiz["questions"] = [
            {
                "tipe": "pilihan_ganda",
                "pertanyaan": "2+2?",
                "opsi": ["3", "4", "5", "6"],
                "jawaban": 1,
                "pembahasan": "4",
            },
            {
                "tipe": "deskripsi",
                "pertanyaan": "Jelaskan siklus air!",
                "jawaban": "Air menguap, mengembun, lalu turun sebagai hujan.",
                "pembahasan": "Evaporasi, kondensasi, presipitasi.",
            },
        ]
        self._open(client, "quiz-poin")

        async def fake_grade(items):
            assert len(items) == 1
            assert items[0]["tipe"] == "deskripsi"
            # AI memberi skor 0.8 → poin = 0.8 × 10 = 8
            return {1: {"verdict": "parsial", "skor": 0.8, "umpan_balik": "Bagus"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post(
            "/api/quiz/quiz-poin/submit",
            json={"answers": {"0": 1, "1": "air menguap lalu turun hujan"}},
        )
        assert res.status_code == 200
        body = res.json()
        # total dapat = 2 (PG benar) + 8 (deskripsi 0.8 × 10) = 10 dari 12 poin
        assert body["poin"] == 10
        assert body["poin_maks"] == 12
        assert body["nilai"] == 83
        pg = body["per_question"][0]
        assert pg["poin"] == 2 and pg["poin_maks"] == 2
        desc = body["per_question"][1]
        assert desc["poin"] == 8 and desc["poin_maks"] == 10
        stored_score = sb.tables["attempts"][0]["score"]
        assert stored_score["poin"] == 10
        assert stored_score["poin_maks"] == 12

    def test_submit_deskripsi_poin_parsial(self, client, sb, monkeypatch):
        """Contoh: jawaban deskripsi 80% benar pada soal berpoin 20 → 16 poin."""
        exam_type_fixture(sb)
        sb.tables["exam_types"][0]["poin_per_tipe"] = {"deskripsi": 20}
        quiz = insert_quiz(sb, "quiz-poin-parsial-uraian")
        quiz["questions"] = [
            {
                "tipe": "deskripsi",
                "pertanyaan": "Jelaskan proses fotosintesis!",
                "jawaban": "Tumbuhan mengubah air dan CO2 menjadi glukosa dan oksigen dengan bantuan cahaya matahari.",
                "pembahasan": "Fotosintesis membutuhkan cahaya, air, dan CO2.",
            }
        ]
        self._open(client, "quiz-poin-parsial-uraian")

        async def fake_grade(items):
            # AI menilai jawaban siswa benar 80%
            return {0: {"verdict": "parsial", "skor": 0.8, "umpan_balik": "Sebagian besar poin tepat"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post(
            "/api/quiz/quiz-poin-parsial-uraian/submit",
            json={"answers": {"0": "Tumbuhan membuat makanan sendiri memakai cahaya matahari"}},
        )
        assert res.status_code == 200
        body = res.json()
        assert body["poin"] == 16
        assert body["poin_maks"] == 20
        assert body["nilai"] == 80
        assert body["per_question"][0]["poin"] == 16
        assert body["per_question"][0]["poin_maks"] == 20

    def test_submit_poin_partial_unconfigured_type_defaults_to_one(self, client, sb, monkeypatch):
        """Tipe yang tidak disebut di poin_per_tipe bernilai 1 poin."""
        exam_type_fixture(sb)
        sb.tables["exam_types"][0]["poin_per_tipe"] = {"deskripsi": 10}
        quiz = insert_quiz(sb, "quiz-poin-parsial")
        quiz["questions"] = [
            {
                "tipe": "benar_salah",
                "pertanyaan": "Air mendidih pada 100 derajat Celsius.",
                "jawaban": "benar",
                "pembahasan": "Ya.",
            },
        ]
        self._open(client, "quiz-poin-parsial")

        async def fake_grade(items):
            return {}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post(
            "/api/quiz/quiz-poin-parsial/submit",
            json={"answers": {"0": "benar"}},
        )
        body = res.json()
        assert body["poin"] == 1
        assert body["poin_maks"] == 1
        assert body["nilai"] == 100

    def test_submit_wrong_answers(self, client, sb, monkeypatch):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        self._open(client, "quiz-123")

        async def fake_grade(items):
            return {2: {"verdict": "salah", "skor": 0.0, "umpan_balik": "-"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 3, "1": "salah", "2": "Bandung"}},
        )
        body = res.json()
        assert body["nilai"] == 0
        assert [pq["verdict"] for pq in body["per_question"]] == ["salah", "salah", "salah"]

    def test_submit_expired_flag(self, client, sb, monkeypatch):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        self._open(client, "quiz-123")
        sb.tables["attempts"][0]["expires_at"] = (
            datetime.now(timezone.utc) - timedelta(minutes=5)
        ).isoformat()

        async def fake_grade(items):
            return {2: {"verdict": "benar", "skor": 1.0, "umpan_balik": "Benar"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 1, "1": "benar", "2": "Jakarta"}},
        )
        body = res.json()
        assert body["expired"] is True
        assert sb.tables["attempts"][0]["expired"] is True

    def test_submit_never_opened_quiz(self, client, sb, monkeypatch):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123", started=False)

        async def fake_grade(items):
            return {2: {"verdict": "benar", "skor": 1.0, "umpan_balik": "Benar"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 1, "1": "benar", "2": "Jakarta"}},
        )
        assert res.status_code == 200
        body = res.json()
        assert body["expired"] is True  # kedaluwarsa langsung saat mulai=submit
        stored = sb.tables["quizzes"][0]
        assert stored["started"] is True
        assert sb.tables["attempts"][0]["expired"] is True

    def test_submit_quiz_not_found(self, client, sb):
        res = client.post("/api/quiz/tidak-ada/submit", json={"answers": {}})
        assert res.status_code == 404

    def test_submit_rejects_too_many_answer_keys(self, client, sb):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        answers = {str(i): "x" for i in range(quiz_router.MAX_ANSWER_KEYS + 1)}
        res = client.post("/api/quiz/quiz-123/submit", json={"answers": answers})
        assert res.status_code == 422

    def test_submit_rejects_oversized_answer_value(self, client, sb):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": "x" * (quiz_router.MAX_ANSWER_VALUE_LENGTH + 1)}},
        )
        assert res.status_code == 422

    def test_submit_rejects_nested_answer_value(self, client, sb):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": {"nested": "object"}}},
        )
        assert res.status_code == 422

    def test_submit_accepts_answers_within_bounds(self, client, sb, monkeypatch):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")

        async def fake_grade(items):
            return {2: {"verdict": "benar", "skor": 1.0, "umpan_balik": "Benar"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 1, "1": "benar", "2": "Jakarta"}},
        )
        assert res.status_code == 200

    def test_submit_llm_error_returns_502(self, client, sb, monkeypatch):
        """Isian tidak lagi memanggil AI (dinilai lokal) — pakai soal
        deskripsi supaya galat AI sungguh teruji di jalur yang masih
        memakainya."""
        exam_type_fixture(sb)
        quiz = insert_quiz(sb, "quiz-123")
        quiz["questions"][2] = {
            "tipe": "deskripsi",
            "pertanyaan": "Jelaskan proses fotosintesis.",
            "jawaban": "Tumbuhan mengubah cahaya matahari menjadi energi kimia.",
            "pembahasan": "Fotosintesis mengubah CO2 dan air menjadi glukosa dan oksigen.",
        }
        self._open(client, "quiz-123")

        async def fake_grade(items):
            raise quiz_router.LLMError("Gagal mengoreksi jawaban uraian.")

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post("/api/quiz/quiz-123/submit", json={"answers": {"2": "x"}})
        assert res.status_code == 502

    def test_submit_twice_is_idempotent(self, client, sb, monkeypatch):
        """Percobaan ulang (respons pertama hilang di jaringan) mengembalikan
        hasil tersimpan tanpa menilai ulang dan tanpa menimpa nilai. Pakai
        soal deskripsi supaya jalur AI (yang harus dipanggil hanya sekali)
        sungguh teruji — isian sendiri kini dinilai lokal."""
        exam_type_fixture(sb)
        quiz = insert_quiz(sb, "quiz-123")
        quiz["questions"][2] = {
            "tipe": "deskripsi",
            "pertanyaan": "Jelaskan proses fotosintesis.",
            "jawaban": "Tumbuhan mengubah cahaya matahari menjadi energi kimia.",
            "pembahasan": "Fotosintesis mengubah CO2 dan air menjadi glukosa dan oksigen.",
        }
        self._open(client, "quiz-123")

        calls = []

        async def fake_grade(items):
            calls.append(items)
            return {2: {"verdict": "parsial", "skor": 0.5, "umpan_balik": "Hampir tepat"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)

        first = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 1, "1": "benar", "2": "Tumbuhan mengubah cahaya jadi energi"}},
        )
        assert first.status_code == 200
        submitted_at = sb.tables["attempts"][0]["submitted_at"]

        second = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 0, "1": "salah", "2": ""}},
        )
        assert second.status_code == 200
        assert second.json() == first.json()
        # AI hanya dipanggil sekali, dan jawaban/waktu asli tidak berubah
        assert len(calls) == 1
        assert sb.tables["attempts"][0]["submitted_at"] == submitted_at
        assert sb.tables["attempts"][0]["answers"] == {
            "0": 1,
            "1": "benar",
            "2": "Tumbuhan mengubah cahaya jadi energi",
        }


class TestAvailable:
    def test_aggregates_combinations(self, client, sb):
        sb.tables["exam_types"] = [
            {"id": "et-1", "name": "Ujian Harian", "jumlah_soal": None, "durasi_menit": None},
            {"id": "et-2", "name": "Ujian Semester", "jumlah_soal": None, "durasi_menit": None},
        ]
        sb.tables.setdefault("quizzes", []).extend(
            [
                {"subject": "IPA", "subject_id": "sub-4", "grade": 5, "exam_type_id": "et-1", "started": False},
                {"subject": "IPA", "subject_id": "sub-4", "grade": 5, "exam_type_id": "et-1", "started": True},
                {"subject": "IPA", "subject_id": "sub-4", "grade": 5, "exam_type_id": "et-2", "started": False},
                {"subject": "Matematika", "subject_id": "sub-1", "grade": 3, "exam_type_id": "et-1", "started": True},
            ]
        )
        res = client.get("/api/quiz/available")
        assert res.status_code == 200
        combos = res.json()
        assert len(combos) == 3
        et1_ipa5 = next(
            c for c in combos if c["subject"] == "IPA" and c["exam_type_id"] == "et-1"
        )
        assert et1_ipa5["grade"] == 5
        assert et1_ipa5["exam_type"] == "Ujian Harian"
        assert et1_ipa5["unstarted"] == 1
        assert et1_ipa5["total"] == 2
        mat = next(c for c in combos if c["subject"] == "Matematika")
        assert mat["unstarted"] == 0
        assert mat["total"] == 1
        # sorted by subject, grade, exam_type_id
        assert combos == sorted(combos, key=lambda c: (c["subject"], c["grade"], c["exam_type_id"]))

    def test_empty(self, client, sb):
        res = client.get("/api/quiz/available")
        assert res.status_code == 200
        assert res.json() == []


class TestClientCorrelation:
    def _setup(self, sb):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123", started=True)

    def test_submit_records_client_id_from_cookie(self, client, sb, monkeypatch):
        self._setup(sb)

        async def fake_grade(items):
            return {2: {"verdict": "benar", "skor": 1.0, "umpan_balik": "-"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        client.cookies.set("client_id", "11111111-1111-1111-1111-111111111111")
        res = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 1, "1": "benar", "2": "Jakarta"}},
        )
        assert res.status_code == 200
        assert sb.tables["attempts"][0]["client_id"] == "11111111-1111-1111-1111-111111111111"

        # Riwayat untuk cookie tersebut berisi attempt tadi
        res = client.get("/api/quiz/attempts")
        rows = res.json()
        assert len(rows) == 1
        assert rows[0]["quiz_id"] == "quiz-123"
        assert rows[0]["subject"] == "Matematika"
        assert rows[0]["exam_type"] == "Ujian Harian"
        assert rows[0]["nilai"] == 100

    def test_new_client_gets_own_cookie_and_empty_history(self, client, sb, monkeypatch):
        self._setup(sb)

        async def fake_grade(items):
            return {2: {"verdict": "benar", "skor": 1.0, "umpan_balik": "-"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        client.cookies.set("client_id", "22222222-2222-2222-2222-222222222222")
        client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 1, "1": "benar", "2": "Jakarta"}},
        )
        # Klien lain (cookie berbeda) tidak melihat riwayat tersebut
        other = TestClient(client.app)
        other.cookies.set("client_id", "33333333-3333-3333-3333-333333333333")
        assert other.get("/api/quiz/attempts").json() == []

    def test_request_sets_cookie(self, client, sb):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-1")
        res = client.post(
            "/api/quiz/request",
            json={"subject": "Matematika", "grade": 3, "exam_type_id": EXAM_TYPE_ID},
        )
        assert res.status_code == 200
        assert "client_id" in res.cookies

    def test_submit_generates_client_id_if_missing(self, client, sb, monkeypatch):
        self._setup(sb)

        async def fake_grade(items):
            return {2: {"verdict": "benar", "skor": 1.0, "umpan_balik": "-"}}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        res = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 1, "1": "benar", "2": "Jakarta"}},
        )
        assert res.status_code == 200
        assert "client_id" in res.cookies
        stored = sb.tables["attempts"][0]["client_id"]
        assert stored  # uuid dibuat server-side
        assert stored == res.cookies.get("client_id")


class TestResolveImages:
    def _question(self, **overrides):
        q = copy.deepcopy(QUESTIONS[0])
        q.update(overrides)
        return q

    async def test_generated_image_resolved(self, monkeypatch, sb):
        async def fake_generate_image(prompt):
            assert prompt == "diagram segitiga"
            return b"bytes", "image/png"

        monkeypatch.setattr(quiz_router, "generate_image", fake_generate_image)
        monkeypatch.setattr(
            quiz_router, "upload_image_bytes", lambda sb, data, ct: "http://img/1.png"
        )
        q = self._question(gambar_tipe="generated", gambar_prompt="diagram segitiga")
        await quiz_router._resolve_images(sb, [q])
        assert q["gambar"] == "http://img/1.png"
        assert "gambar_tipe" not in q
        assert "gambar_prompt" not in q

    async def test_stock_image_resolved(self, monkeypatch, sb):
        async def fake_search(query):
            assert query == "rumah adat"
            return b"bytes"

        monkeypatch.setattr(quiz_router, "search_stock_image", fake_search)
        monkeypatch.setattr(
            quiz_router, "upload_image_bytes", lambda sb, data, ct: "http://img/2.jpg"
        )
        q = self._question(gambar_tipe="stock", gambar_cari="rumah adat")
        await quiz_router._resolve_images(sb, [q])
        assert q["gambar"] == "http://img/2.jpg"
        assert "gambar_cari" not in q

    async def test_stock_no_result_leaves_no_image(self, monkeypatch, sb):
        async def fake_search(query):
            return None

        monkeypatch.setattr(quiz_router, "search_stock_image", fake_search)
        q = self._question(gambar_tipe="stock", gambar_cari="rumah adat")
        await quiz_router._resolve_images(sb, [q])
        assert "gambar" not in q

    async def test_generation_error_skips_gracefully(self, monkeypatch, sb):
        async def fake_generate_image(prompt):
            raise quiz_router.ImageGenError("gagal")

        monkeypatch.setattr(quiz_router, "generate_image", fake_generate_image)
        q = self._question(gambar_tipe="generated", gambar_prompt="diagram")
        await quiz_router._resolve_images(sb, [q])
        assert "gambar" not in q

    async def test_no_gambar_tipe_untouched(self, sb):
        q = self._question()
        await quiz_router._resolve_images(sb, [q])
        assert "gambar" not in q

    async def test_network_failure_skips_gracefully_not_whole_batch(self, monkeypatch, sb):
        """Kegagalan koneksi sungguhan (bukan generate_image yang di-stub)
        pada satu soal tidak boleh menggagalkan pembuatan gambar soal lain
        dalam batch yang sama — menguji generate_image() + _resolve_images()
        bersama-sama, bukan cuma penanganan ImageGenError yang sudah teruji
        di test_generation_error_skips_gracefully."""
        from app.config import settings

        monkeypatch.setattr(settings, "openrouter_api_key", "fake-key-for-tests")

        async def flaky_post(self, url, headers=None, json=None):
            if json["messages"][0]["content"] == "gambar-bermasalah":
                raise httpx.ConnectError("koneksi gagal")
            return httpx.Response(
                200,
                json={
                    "choices": [
                        {
                            "message": {
                                "images": [
                                    {"image_url": {"url": "data:image/png;base64,YQ=="}}
                                ]
                            }
                        }
                    ]
                },
                request=httpx.Request("POST", url),
            )

        monkeypatch.setattr(httpx.AsyncClient, "post", flaky_post)
        monkeypatch.setattr(
            quiz_router, "upload_image_bytes", lambda sb, data, ct: "http://img/ok.png"
        )
        bad = self._question(gambar_tipe="generated", gambar_prompt="gambar-bermasalah")
        good = self._question(gambar_tipe="generated", gambar_prompt="gambar-oke")
        await quiz_router._resolve_images(sb, [bad, good])
        assert "gambar" not in bad
        assert good["gambar"] == "http://img/ok.png"

    async def test_cap_limits_number_of_images_per_paket(self, monkeypatch, sb):
        calls = []

        async def fake_generate_image(prompt):
            calls.append(prompt)
            return b"bytes", "image/png"

        monkeypatch.setattr(quiz_router, "generate_image", fake_generate_image)
        monkeypatch.setattr(
            quiz_router, "upload_image_bytes", lambda sb, data, ct: "http://img/x.png"
        )
        questions = [
            self._question(gambar_tipe="generated", gambar_prompt=f"gambar {i}")
            for i in range(quiz_router.IMAGE_CAP_PER_PAKET + 2)
        ]
        await quiz_router._resolve_images(sb, questions)
        assert len(calls) == quiz_router.IMAGE_CAP_PER_PAKET
        with_image = [q for q in questions if "gambar" in q]
        assert len(with_image) == quiz_router.IMAGE_CAP_PER_PAKET


class TestQuizManagement:
    """Admin mengelola paket soal: daftar, detail lengkap, hapus."""

    def test_list_requires_admin(self, client, sb):
        res = client.get("/api/quiz/admin/list")
        assert res.status_code == 401

    def test_list_returns_metadata(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-2")
        insert_quiz(sb, "quiz-1")
        sb.tables["quizzes"][0]["created_at"] = "2026-01-02T00:00:00+00:00"
        sb.tables["quizzes"][1]["created_at"] = "2026-01-01T00:00:00+00:00"
        res = client.get("/api/quiz/admin/list", headers=admin_headers)
        assert res.status_code == 200
        rows = res.json()
        assert [r["id"] for r in rows] == ["quiz-2", "quiz-1"]
        assert rows[0]["subject"] == "Matematika"
        assert rows[0]["grade"] == 3
        assert rows[0]["exam_type"] == "Ujian Harian"
        assert rows[0]["jumlah_soal"] == 3
        assert rows[0]["started"] is False
        assert rows[0]["durasi_menit"] == 60
        # metadata saja — isi soal tidak dikirim di daftar
        assert "questions" not in rows[0]

    def test_list_filters(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        sb.tables["exam_types"].append(
            {"id": "et-2", "name": "Ujian Semester", "jumlah_soal": None, "durasi_menit": None}
        )
        insert_quiz(sb, "quiz-1")
        sb.tables["quizzes"][0]["created_at"] = "2026-01-01T00:00:00+00:00"
        insert_quiz(sb, "quiz-2", exam_type_id="et-2")
        sb.tables["quizzes"][1]["created_at"] = "2026-01-02T00:00:00+00:00"
        sb.tables["quizzes"][1]["subject"] = "IPA"
        sb.tables["quizzes"][1]["subject_id"] = "sub-4"
        sb.tables["quizzes"][1]["grade"] = 5

        res = client.get("/api/quiz/admin/list", headers=admin_headers)
        assert len(res.json()) == 2

        res = client.get("/api/quiz/admin/list?subject=IPA", headers=admin_headers)
        assert [r["id"] for r in res.json()] == ["quiz-2"]

        res = client.get("/api/quiz/admin/list?grade=3", headers=admin_headers)
        assert [r["id"] for r in res.json()] == ["quiz-1"]

        res = client.get(
            "/api/quiz/admin/list?exam_type_id=et-2", headers=admin_headers
        )
        assert [r["id"] for r in res.json()] == ["quiz-2"]

    def test_list_filter_by_subject_id(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-1")  # Matematika sub-1
        sb.tables["quizzes"][0]["created_at"] = "2026-01-01T00:00:00+00:00"
        insert_quiz(sb, "quiz-2", subject="IPA", subject_id="sub-4")
        sb.tables["quizzes"][1]["created_at"] = "2026-01-02T00:00:00+00:00"

        res = client.get(
            "/api/quiz/admin/list?subject_id=sub-4", headers=admin_headers
        )
        assert [r["id"] for r in res.json()] == ["quiz-2"]

    def test_list_response_carries_subject_id_and_name(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-1")
        res = client.get("/api/quiz/admin/list", headers=admin_headers)
        row = res.json()[0]
        assert row["subject"] == "Matematika"
        assert row["subject_id"] == "sub-1"

    def test_detail_response_carries_subject_id_and_name(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.get("/api/quiz/admin/quizzes/quiz-123", headers=admin_headers)
        body = res.json()
        assert body["subject"] == "Matematika"
        assert body["subject_id"] == "sub-1"

    def test_detail_includes_answers(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.get("/api/quiz/admin/quizzes/quiz-123", headers=admin_headers)
        assert res.status_code == 200
        body = res.json()
        assert body["exam_type"] == "Ujian Harian"
        assert body["subject"] == "Matematika"
        qs = body["questions"]
        assert [q["nomor"] for q in qs] == [1, 2, 3]
        assert qs[0]["jawaban"] == 1
        assert qs[0]["opsi"] == ["3", "4", "5", "6"]
        assert qs[0]["pembahasan"] == "2 + 2 = 4"
        assert qs[1]["jawaban"] == "benar"
        assert qs[2]["jawaban"] == "Jakarta"
        assert "jawaban" in qs[0] and "pembahasan" in qs[0]

    def test_detail_requires_admin(self, client, sb):
        res = client.get("/api/quiz/admin/quizzes/quiz-1")
        assert res.status_code == 401

    def test_detail_not_found(self, client, sb, admin_auth, admin_headers):
        res = client.get("/api/quiz/admin/quizzes/tidak-ada", headers=admin_headers)
        assert res.status_code == 404

    def test_delete_removes_quiz_and_attempts(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123", started=True)
        opened = client.get("/api/quiz/quiz-123")
        assert opened.status_code == 200
        assert sb.tables["attempts"]

        res = client.delete(
            "/api/quiz/admin/quizzes/quiz-123", headers=admin_headers
        )
        assert res.status_code == 204
        assert sb.tables["quizzes"] == []
        assert sb.tables["attempts"] == []

    def test_delete_unstarted_quiz(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-1")
        res = client.delete(
            "/api/quiz/admin/quizzes/quiz-1", headers=admin_headers
        )
        assert res.status_code == 204
        assert sb.tables["quizzes"] == []
        assert sb.tables.get("attempts", []) == []

    def test_delete_not_found(self, client, sb, admin_auth, admin_headers):
        res = client.delete(
            "/api/quiz/admin/quizzes/tidak-ada", headers=admin_headers
        )
        assert res.status_code == 404

    def test_delete_requires_admin(self, client, sb):
        res = client.delete("/api/quiz/admin/quizzes/quiz-1")
        assert res.status_code == 401

    def test_edit_replaces_questions(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        questions = [
            {
                "tipe": "pilihan_ganda",
                "pertanyaan": "Hasil 3 × 3 adalah?",
                "opsi": ["6", "8", "9", "12"],
                "jawaban": 2,
                "pembahasan": "3 × 3 = 9.",
            },
            {
                "tipe": "isian",
                "pertanyaan": "Proklamasi RI dilakukan tahun?",
                "jawaban": "1945",
                "pembahasan": "Proklamasi 17 Agustus 1945.",
            },
        ]
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={"questions": questions},
            headers=admin_headers,
        )
        assert res.status_code == 200
        body = res.json()
        assert [q["nomor"] for q in body["questions"]] == [1, 2]
        stored = sb.tables["quizzes"][0]["questions"]
        assert stored[0]["jawaban"] == 2
        assert stored[1]["jawaban"] == "1945"  # string angka tetap string
        assert "opsi" not in stored[1]
        assert "nomor" not in stored[0]

    def test_edit_updates_list_jumlah_soal(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        questions = [
            {
                "tipe": "benar_salah",
                "pertanyaan": "Air mendidih pada 100 derajat Celsius.",
                "jawaban": "benar",
                "pembahasan": "Ya.",
            }
        ]
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={"questions": questions},
            headers=admin_headers,
        )
        assert res.status_code == 200
        listing = client.get("/api/quiz/admin/list", headers=admin_headers).json()
        assert listing[0]["jumlah_soal"] == 1

    def test_edit_keeps_gambar(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        quiz = insert_quiz(sb, "quiz-123")
        quiz["questions"][0]["gambar"] = "http://img/x.png"
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={"questions": copy.deepcopy(quiz["questions"])},
            headers=admin_headers,
        )
        assert res.status_code == 200
        assert res.json()["questions"][0]["gambar"] == "http://img/x.png"
        assert sb.tables["quizzes"][0]["questions"][0]["gambar"] == "http://img/x.png"

    def test_edit_requires_admin(self, client, sb):
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-1", json={"questions": []}
        )
        assert res.status_code == 401

    def test_edit_not_found(self, client, sb, admin_auth, admin_headers):
        res = client.patch(
            "/api/quiz/admin/quizzes/tidak-ada",
            json={"questions": []},
            headers=admin_headers,
        )
        assert res.status_code == 404

    def test_edit_empty_questions_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={"questions": []},
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_edit_invalid_tipe_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={"questions": [{"tipe": "aneh", "pertanyaan": "?", "jawaban": "x", "pembahasan": "y"}]},
            headers=admin_headers,
        )
        assert res.status_code == 422
        assert "tipe tidak valid" in res.json()["detail"]

    def test_edit_pg_bad_index_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={
                "questions": [
                    {
                        "tipe": "pilihan_ganda",
                        "pertanyaan": "2+2?",
                        "opsi": ["3", "4", "5", "6"],
                        "jawaban": 7,
                        "pembahasan": "4",
                    }
                ]
            },
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_edit_pg_wrong_opsi_count_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={
                "questions": [
                    {
                        "tipe": "pilihan_ganda",
                        "pertanyaan": "2+2?",
                        "opsi": ["3", "4"],
                        "jawaban": 1,
                        "pembahasan": "4",
                    }
                ]
            },
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_edit_pg_empty_option_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={
                "questions": [
                    {
                        "tipe": "pilihan_ganda",
                        "pertanyaan": "2+2?",
                        "opsi": ["3", "4", " ", "6"],
                        "jawaban": 1,
                        "pembahasan": "4",
                    }
                ]
            },
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_edit_bs_invalid_jawaban_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={
                "questions": [
                    {
                        "tipe": "benar_salah",
                        "pertanyaan": "Air mendidih pada 100 derajat.",
                        "jawaban": "mungkin",
                        "pembahasan": "Ya.",
                    }
                ]
            },
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_edit_isian_empty_jawaban_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={
                "questions": [
                    {
                        "tipe": "isian",
                        "pertanyaan": "Ibukota RI?",
                        "jawaban": "   ",
                        "pembahasan": "Jakarta.",
                    }
                ]
            },
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_edit_empty_pertanyaan_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={
                "questions": [
                    {
                        "tipe": "isian",
                        "pertanyaan": " ",
                        "jawaban": "Jakarta",
                        "pembahasan": "Jakarta.",
                    }
                ]
            },
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_edit_empty_pembahasan_422(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123")
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={
                "questions": [
                    {
                        "tipe": "isian",
                        "pertanyaan": "Ibukota RI?",
                        "jawaban": "Jakarta",
                        "pembahasan": "",
                    }
                ]
            },
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_edited_questions_still_gradeable(self, client, sb, admin_auth, admin_headers, monkeypatch):
        """Soal hasil edit tetap bisa dikerjakan & dinilai siswa."""
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-123", started=True)
        client.get("/api/quiz/quiz-123")
        new_questions = [
            {
                "tipe": "pilihan_ganda",
                "pertanyaan": "Hasil 5 × 5 adalah?",
                "opsi": ["10", "20", "25", "30"],
                "jawaban": 2,
                "pembahasan": "5 × 5 = 25.",
            }
        ]
        res = client.patch(
            "/api/quiz/admin/quizzes/quiz-123",
            json={"questions": new_questions},
            headers=admin_headers,
        )
        assert res.status_code == 200

        async def fake_grade(items):
            return {}

        monkeypatch.setattr(quiz_router, "grade_short_answers", fake_grade)
        submit = client.post(
            "/api/quiz/quiz-123/submit",
            json={"answers": {"0": 2}},
        )
        assert submit.status_code == 200
        assert submit.json()["nilai"] == 100


class TestPoolReset:
    def test_reset_deletes_only_unstarted(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-unstarted-1")
        insert_quiz(sb, "quiz-unstarted-2")
        insert_quiz(sb, "quiz-started", started=True)

        res = client.post(
            "/api/quiz/pool/reset",
            json={"subject": "Matematika", "grade": 3, "exam_type_id": EXAM_TYPE_ID},
            headers=admin_headers,
        )
        assert res.status_code == 200
        assert res.json()["deleted"] == 2
        ids = {q["id"] for q in sb.tables["quizzes"]}
        assert ids == {"quiz-started"}

    def test_reset_requires_admin(self, client, sb):
        res = client.post(
            "/api/quiz/pool/reset",
            json={"subject": "Matematika", "grade": 3, "exam_type_id": "et-1"},
        )
        assert res.status_code == 401

    def test_reset_invalid_exam_type(self, client, sb, admin_auth, admin_headers):
        res = client.post(
            "/api/quiz/pool/reset",
            json={"subject": "Matematika", "grade": 3, "exam_type_id": "tidak-ada"},
            headers=admin_headers,
        )
        assert res.status_code == 422


class TestBulkDelete:
    """POST /api/quiz/admin/quizzes/bulk-delete — hapus banyak paket sekaligus."""

    def _bulk(self, client, admin_headers, ids):
        return client.post(
            "/api/quiz/admin/quizzes/bulk-delete",
            json={"ids": ids},
            headers=admin_headers,
        )

    def test_bulk_delete_mixed_started_and_unstarted(self, client, sb, admin_auth, admin_headers):
        """Paket yang sudah pernah dibuka siswa ikut terhapus; riwayat
        (attempts) terhapus lewat cascade."""
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-a")
        insert_quiz(sb, "quiz-b", started=True)
        insert_quiz(sb, "quiz-keep")
        opened = client.get("/api/quiz/quiz-b")
        assert opened.status_code == 200
        assert sb.tables["attempts"]  # attempt quiz-b ada

        res = self._bulk(client, admin_headers, ["quiz-a", "quiz-b"])
        assert res.status_code == 200
        assert res.json() == {"deleted": 2}
        ids = {q["id"] for q in sb.tables["quizzes"]}
        assert ids == {"quiz-keep"}
        # Riwayat quiz-b ikut terhapus (cascade), quiz tidak disinggung tetap aman
        assert sb.tables["attempts"] == []

    def test_bulk_delete_count_and_dedupe(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-a")
        insert_quiz(sb, "quiz-b")
        res = self._bulk(client, admin_headers, ["quiz-a", "quiz-a", "quiz-b"])
        assert res.status_code == 200
        assert res.json()["deleted"] == 2
        assert sb.tables["quizzes"] == []

    def test_bulk_delete_unknown_ids_ignored(self, client, sb, admin_auth, admin_headers):
        exam_type_fixture(sb)
        insert_quiz(sb, "quiz-a")
        res = self._bulk(client, admin_headers, ["quiz-a", "tidak-ada"])
        assert res.status_code == 200
        assert res.json()["deleted"] == 1

    def test_bulk_delete_empty_422(self, client, sb, admin_auth, admin_headers):
        res = self._bulk(client, admin_headers, [])
        assert res.status_code == 422

    def test_bulk_delete_more_than_200_422(self, client, sb, admin_auth, admin_headers):
        res = self._bulk(client, admin_headers, [f"q{i}" for i in range(201)])
        assert res.status_code == 422

    def test_bulk_delete_requires_admin(self, client, sb):
        res = client.post("/api/quiz/admin/quizzes/bulk-delete", json={"ids": ["q-1"]})
        assert res.status_code == 401