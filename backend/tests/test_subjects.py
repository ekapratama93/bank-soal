import pytest

from app.routers import quiz as quiz_router


class TestSubjects:
    def test_public_list(self, client, sb):
        res = client.get("/api/subjects")
        assert res.status_code == 200
        names = [s["name"] for s in res.json()]
        assert "Matematika" in names
        assert names == sorted(names)

    def test_create(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/subjects",
            headers=admin_headers,
            json={"name": "Sejarah"},
        )
        assert res.status_code == 201
        assert res.json()["name"] == "Sejarah"

    def test_create_duplicate_409(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/subjects",
            headers=admin_headers,
            json={"name": "Matematika"},
        )
        assert res.status_code == 409
        assert "sudah ada" in res.json()["detail"]

    def test_create_empty_name(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/subjects",
            headers=admin_headers,
            json={"name": "   "},
        )
        assert res.status_code == 422

    def test_delete_free(self, client, admin_auth, admin_headers):
        created = client.post(
            "/api/subjects", headers=admin_headers, json={"name": "Sejarah"}
        ).json()
        res = client.delete(f"/api/subjects/{created['id']}", headers=admin_headers)
        assert res.status_code == 204

    def test_delete_blocked_by_material(self, client, admin_auth, admin_headers):
        sub = next(
            s for s in client.get("/api/subjects").json() if s["name"] == "IPA"
        )
        admin_auth.tables["materials"] = [
            {"id": "m-1", "subject": "IPA", "subject_id": sub["id"]}
        ]
        res = client.delete(f"/api/subjects/{sub['id']}", headers=admin_headers)
        assert res.status_code == 409
        assert "materi" in res.json()["detail"]

    def test_delete_blocked_by_material_legacy_text_row(self, client, admin_auth, admin_headers):
        """Baris yang ditulis backend lama selama jendela deploy hanya membawa
        teks subject (subject_id belum ter-backfill) — tetap harus memblokir."""
        sub = next(
            s for s in client.get("/api/subjects").json() if s["name"] == "IPA"
        )
        admin_auth.tables["materials"] = [{"id": "m-1", "subject": "IPA"}]
        res = client.delete(f"/api/subjects/{sub['id']}", headers=admin_headers)
        assert res.status_code == 409

    def test_delete_blocked_by_quiz(self, client, admin_auth, admin_headers):
        sub = next(
            s for s in client.get("/api/subjects").json() if s["name"] == "IPA"
        )
        admin_auth.tables["quizzes"] = [
            {"id": "q-1", "subject": "IPA", "subject_id": sub["id"]}
        ]
        res = client.delete(f"/api/subjects/{sub['id']}", headers=admin_headers)
        assert res.status_code == 409
        assert "kuis" in res.json()["detail"]

    def test_rename(self, client, admin_auth, admin_headers):
        sub = next(
            s for s in client.get("/api/subjects").json() if s["name"] == "IPA"
        )
        res = client.patch(
            f"/api/subjects/{sub['id']}",
            headers=admin_headers,
            json={"name": "Ilmu Pengetahuan Alam"},
        )
        assert res.status_code == 200
        body = res.json()
        assert body["id"] == sub["id"]
        assert body["name"] == "Ilmu Pengetahuan Alam"
        names = [s["name"] for s in client.get("/api/subjects").json()]
        assert "Ilmu Pengetahuan Alam" in names
        assert "IPA" not in names

    def test_rename_not_found(self, client, admin_auth, admin_headers):
        res = client.patch(
            "/api/subjects/tidak-ada", headers=admin_headers, json={"name": "X"}
        )
        assert res.status_code == 404

    def test_rename_empty_422(self, client, admin_auth, admin_headers):
        sub = next(
            s for s in client.get("/api/subjects").json() if s["name"] == "IPA"
        )
        res = client.patch(
            f"/api/subjects/{sub['id']}", headers=admin_headers, json={"name": "   "}
        )
        assert res.status_code == 422

    def test_rename_duplicate_409(self, client, admin_auth, admin_headers):
        sub = next(
            s for s in client.get("/api/subjects").json() if s["name"] == "IPA"
        )
        res = client.patch(
            f"/api/subjects/{sub['id']}",
            headers=admin_headers,
            json={"name": "Matematika"},
        )
        assert res.status_code == 409
        assert "sudah ada" in res.json()["detail"]

    def test_rename_race_unique_violation_becomes_409(
        self, client, admin_auth, admin_headers, monkeypatch
    ):
        """Balapan antara pre-check nama dan update: DB menolak lewat unique
        constraint — harus 409 ramah, bukan 500."""
        from postgrest.exceptions import APIError

        from conftest import Query

        sub = next(
            s for s in client.get("/api/subjects").json() if s["name"] == "IPA"
        )

        real_execute = Query.execute

        def flaky_execute(self):
            if self.table == "subjects" and self.op == "update":
                raise APIError({"code": "23505", "message": "duplicate key"})
            return real_execute(self)

        monkeypatch.setattr(Query, "execute", flaky_execute)
        res = client.patch(
            f"/api/subjects/{sub['id']}",
            headers=admin_headers,
            json={"name": "Baru Banget"},
        )
        assert res.status_code == 409

    def test_rename_requires_admin(self, client, sb):
        sub = next(s for s in client.get("/api/subjects").json() if s["name"] == "IPA")
        res = client.patch(f"/api/subjects/{sub['id']}", json={"name": "X"})
        assert res.status_code == 401

    def test_delete_not_found(self, client, admin_auth, admin_headers):
        res = client.delete("/api/subjects/tidak-ada", headers=admin_headers)
        assert res.status_code == 404

    def test_requires_admin_to_modify(self, client, sb):
        assert client.get("/api/subjects").status_code == 200
        assert client.post("/api/subjects", json={"name": "X"}).status_code == 401

    def test_generate_rejects_unknown_subject(self, client, sb, admin_auth, admin_headers, monkeypatch):
        sb.tables["exam_types"] = [
            {"id": "et-1", "name": "Ujian Harian", "jumlah_soal": None, "durasi_menit": None}
        ]
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"subject": "Mama", "grade": 3, "exam_type_id": "et-1"},
        )
        assert res.status_code == 422
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"subject": "Sejarah Baru", "grade": 3, "exam_type_id": "et-1"},
        )
        assert res.status_code == 422

        async def fake_generate(subject, grade, total, material):
            raise quiz_router.LLMError("gagal LLM")

        monkeypatch.setattr(quiz_router, "generate_quiz", fake_generate)
        # Setelah admin menambahkan mapel baru, generate diterima (gagal di LLM mock)
        client.post("/api/subjects", headers=admin_headers, json={"name": "Sejarah Baru"})
        res = client.post(
            "/api/quiz/generate",
            headers=admin_headers,
            json={"subject": "Sejarah Baru", "grade": 3, "exam_type_id": "et-1"},
        )
        assert res.status_code == 502