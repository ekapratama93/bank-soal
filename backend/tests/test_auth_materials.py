import pytest


class TestLogin:
    def test_admin_login_success(self, client, sb):
        sb.accounts["admin@sekolah.id"] = {
            "password": "rahasia",
            "id": "00000000-0000-0000-0000-000000000001",
        }
        sb.tables["profiles"] = [
            {
                "id": "00000000-0000-0000-0000-000000000001",
                "email": "admin@sekolah.id",
                "role": "admin",
            }
        ]
        res = client.post(
            "/api/auth/login", json={"email": "admin@sekolah.id", "password": "rahasia"}
        )
        assert res.status_code == 200
        body = res.json()
        assert body["access_token"].startswith("tok-")
        assert body["role"] == "admin"

    def test_student_rejected_403(self, client, sb):
        sb.accounts["siswa@sekolah.id"] = {
            "password": "rahasia",
            "id": "00000000-0000-0000-0000-000000000002",
        }
        sb.tables["profiles"] = [
            {
                "id": "00000000-0000-0000-0000-000000000002",
                "email": "siswa@sekolah.id",
                "role": "student",
            }
        ]
        res = client.post(
            "/api/auth/login", json={"email": "siswa@sekolah.id", "password": "rahasia"}
        )
        assert res.status_code == 403

    def test_wrong_password_401(self, client, sb):
        sb.accounts["admin@sekolah.id"] = {
            "password": "rahasia",
            "id": "00000000-0000-0000-0000-000000000001",
        }
        res = client.post(
            "/api/auth/login", json={"email": "admin@sekolah.id", "password": "salah"}
        )
        assert res.status_code == 401

    def test_rate_limited_after_five_attempts(self, client, sb):
        sb.accounts["admin@sekolah.id"] = {
            "password": "rahasia",
            "id": "00000000-0000-0000-0000-000000000001",
        }
        for _ in range(5):
            res = client.post(
                "/api/auth/login",
                json={"email": "admin@sekolah.id", "password": "salah"},
            )
            assert res.status_code == 401

        res = client.post(
            "/api/auth/login",
            json={"email": "admin@sekolah.id", "password": "salah"},
        )
        assert res.status_code == 429
        assert res.headers.get("retry-after") == "900"

    def test_rate_limit_is_per_email(self, client, sb):
        sb.accounts["admin@sekolah.id"] = {
            "password": "rahasia",
            "id": "00000000-0000-0000-0000-000000000001",
        }
        for _ in range(5):
            client.post(
                "/api/auth/login",
                json={"email": "admin@sekolah.id", "password": "salah"},
            )
        # Email lain tidak terpengaruh oleh limit email di atas.
        res = client.post(
            "/api/auth/login",
            json={"email": "lain@sekolah.id", "password": "salah"},
        )
        assert res.status_code == 401  # bukan 429

    def test_rate_limit_key_is_case_insensitive(self, client, sb):
        sb.accounts["admin@sekolah.id"] = {
            "password": "rahasia",
            "id": "00000000-0000-0000-0000-000000000001",
        }
        for _ in range(5):
            client.post(
                "/api/auth/login",
                json={"email": "Admin@Sekolah.id", "password": "salah"},
            )
        res = client.post(
            "/api/auth/login",
            json={"email": "ADMIN@SEKOLAH.ID", "password": "salah"},
        )
        assert res.status_code == 429

    def test_login_infra_failure_is_not_reported_as_wrong_password(
        self, sb, monkeypatch
    ):
        from fastapi.testclient import TestClient
        from supabase_auth.errors import AuthRetryableError

        from app.main import app
        from app.routers import auth as auth_router

        def boom(creds):
            raise AuthRetryableError("gangguan jaringan", 0)

        monkeypatch.setattr(auth_router, "get_supabase", lambda: sb)
        monkeypatch.setattr(auth_router, "get_fresh_client", lambda: sb)
        monkeypatch.setattr(sb.auth, "sign_in_with_password", boom)
        auth_router.limiter.reset()

        no_raise_client = TestClient(app, raise_server_exceptions=False)
        res = no_raise_client.post(
            "/api/auth/login",
            json={"email": "admin@sekolah.id", "password": "rahasia"},
        )
        assert res.status_code == 500


class TestMaterials:
    def test_requires_token(self, client):
        res = client.get("/api/materials")
        assert res.status_code == 401

    def test_get_user_infra_failure_is_not_reported_as_invalid_session(
        self, sb, monkeypatch
    ):
        """Gangguan jaringan/server saat verifikasi token harus jadi 5xx
        (galat umum), bukan disalahartikan sebagai token tidak valid (401) —
        admin tidak perlu login ulang untuk masalah yang bukan salahnya."""
        from fastapi.testclient import TestClient
        from supabase_auth.errors import AuthRetryableError

        from app.main import app
        from app.routers import materials as materials_router

        def boom(token):
            raise AuthRetryableError("gangguan jaringan", 0)

        monkeypatch.setattr(materials_router, "get_supabase", lambda: sb)
        monkeypatch.setattr(sb.auth, "get_user", boom)

        no_raise_client = TestClient(app, raise_server_exceptions=False)
        res = no_raise_client.get(
            "/api/materials", headers={"Authorization": "Bearer tok-apa-saja"}
        )
        assert res.status_code == 500

    def test_requires_admin_role(self, client, sb, monkeypatch):
        student_id = "00000000-0000-0000-0000-000000000002"
        sb.jwt_user = type("U", (), {"id": student_id, "email": "siswa@sekolah.id"})()
        sb.tables["profiles"] = [
            {"id": student_id, "email": "siswa@sekolah.id", "role": "student"}
        ]
        res = client.get(
            "/api/materials", headers={"Authorization": f"Bearer tok-{student_id}"}
        )
        assert res.status_code == 403

    def test_create_and_list(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        res = client.post(
            "/api/materials",
            headers=admin_headers,
            json={
                "subject": "IPA",
                "grade": 5,
                "exam_type_id": exam_type_id,
                "title": "Fotosintesis",
                "content": "Fotosintesis adalah proses tumbuhan membuat makanan.",
            },
        )
        assert res.status_code == 201
        created = res.json()
        assert created["title"] == "Fotosintesis"
        assert created["created_by"] == "admin@sekolah.id"

        res = client.get("/api/materials", headers=admin_headers)
        assert res.status_code == 200
        assert len(res.json()) == 1

    def test_create_invalid_payload(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        res = client.post(
            "/api/materials",
            headers=admin_headers,
            json={"subject": "IPA", "grade": 13, "exam_type_id": exam_type_id, "title": "x", "content": "y"},
        )
        assert res.status_code == 422

    def test_delete(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        created = client.post(
            "/api/materials",
            headers=admin_headers,
            json={
                "subject": "IPA",
                "grade": 5,
                "exam_type_id": exam_type_id,
                "title": "x",
                "content": "y",
            },
        ).json()
        res = client.delete(f"/api/materials/{created['id']}", headers=admin_headers)
        assert res.status_code == 204
        assert client.get("/api/materials", headers=admin_headers).json() == []

    def test_delete_not_found(self, client, admin_auth, admin_headers):
        res = client.delete("/api/materials/tidak-ada", headers=admin_headers)
        assert res.status_code == 404

    def _exam_type(self, sb, exam_type_id="et-1"):
        sb.tables.setdefault("exam_types", []).append(
            {"id": exam_type_id, "name": "Ujian Harian", "jumlah_soal": None, "durasi_menit": None}
        )
        return exam_type_id

    def test_upload_txt(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        res = client.post(
            "/api/materials/upload",
            headers=admin_headers,
            files={"file": ("materi.txt", b"Perkalian adalah penjumlahan berulang.", "text/plain")},
            data={
                "subject": "Matematika",
                "grade": "3",
                "exam_type_id": exam_type_id,
            },
        )
        assert res.status_code == 201
        body = res.json()
        assert body["title"] == "materi"
        assert body["file_name"] == "materi.txt"
        assert "Perkalian" in body["content"]

    def test_upload_unsupported_extension(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        res = client.post(
            "/api/materials/upload",
            headers=admin_headers,
            files={"file": ("materi.rtf", b"isi", "application/octet-stream")},
            data={"subject": "IPA", "grade": "5", "exam_type_id": exam_type_id},
        )
        assert res.status_code == 422
        assert ".pdf, .docx, atau .txt" in res.json()["detail"]

    def test_upload_size_limit(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        res = client.post(
            "/api/materials/upload",
            headers=admin_headers,
            files={"file": ("besar.txt", b"x" * (2 * 1024 * 1024 + 1), "text/plain")},
            data={"subject": "IPA", "grade": "5", "exam_type_id": exam_type_id},
        )
        assert res.status_code == 422
        assert "2 MB" in res.json()["detail"]

    def test_upload_invalid_exam_type(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/materials/upload",
            headers=admin_headers,
            files={"file": ("materi.txt", b"isi materi", "text/plain")},
            data={"subject": "IPA", "grade": "5", "exam_type_id": "tidak-ada"},
        )
        assert res.status_code == 422

    def test_upload_file_plus_text_combined(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        res = client.post(
            "/api/materials/upload",
            headers=admin_headers,
            files={"file": ("materi.txt", b"Isi dari file.", "text/plain")},
            data={
                "subject": "IPA",
                "grade": "5",
                "exam_type_id": exam_type_id,
                "content": "Tambahan teks manual.",
            },
        )
        assert res.status_code == 201
        body = res.json()
        assert "Isi dari file." in body["content"]
        assert "Tambahan teks manual." in body["content"]
        assert body["file_name"] == "materi.txt"

    def test_upload_unreadable_file_with_text_still_succeeds(
        self, client, admin_auth, admin_headers
    ):
        exam_type_id = self._exam_type(admin_auth)
        res = client.post(
            "/api/materials/upload",
            headers=admin_headers,
            files={"file": ("scan.pdf", b"%PDF-korup", "application/pdf")},
            data={
                "subject": "IPA",
                "grade": "5",
                "exam_type_id": exam_type_id,
                "content": "Teks manual sebagai pengganti.",
            },
        )
        assert res.status_code == 201
        assert "Teks manual" in res.json()["content"]

    def test_update_material(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        created = client.post(
            "/api/materials",
            headers=admin_headers,
            json={
                "subject": "IPA",
                "grade": 5,
                "exam_type_id": exam_type_id,
                "title": "Lama",
                "content": "isi lama",
            },
        ).json()
        res = client.patch(
            f"/api/materials/{created['id']}",
            headers=admin_headers,
            json={"title": "Baru", "content": "isi baru"},
        )
        assert res.status_code == 200
        body = res.json()
        assert body["title"] == "Baru"
        assert body["content"] == "isi baru"

    def test_update_empty_content_422(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        created = client.post(
            "/api/materials",
            headers=admin_headers,
            json={
                "subject": "IPA",
                "grade": 5,
                "exam_type_id": exam_type_id,
                "title": "x",
                "content": "isi",
            },
        ).json()
        res = client.patch(
            f"/api/materials/{created['id']}",
            headers=admin_headers,
            json={"content": "   "},
        )
        assert res.status_code == 422

    def _pool_quiz(self, sb, exam_type_id, quiz_id, started):
        sb.tables.setdefault("quizzes", []).append(
            {
                "id": quiz_id,
                "subject": "IPA",
                "grade": 5,
                "exam_type_id": exam_type_id,
                "questions": [],
                "started": started,
                "expires_at": None,
            }
        )

    def test_create_material_invalidates_pool(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        self._pool_quiz(admin_auth, exam_type_id, "q-unstarted", started=False)
        self._pool_quiz(admin_auth, exam_type_id, "q-started", started=True)
        res = client.post(
            "/api/materials",
            headers=admin_headers,
            json={
                "subject": "IPA",
                "grade": 5,
                "exam_type_id": exam_type_id,
                "title": "Baru",
                "content": "isi",
            },
        )
        assert res.status_code == 201
        ids = {q["id"] for q in admin_auth.tables["quizzes"]}
        assert ids == {"q-started"}

    def test_update_material_invalidates_pool(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        created = client.post(
            "/api/materials",
            headers=admin_headers,
            json={
                "subject": "IPA",
                "grade": 5,
                "exam_type_id": exam_type_id,
                "title": "x",
                "content": "isi",
            },
        ).json()
        self._pool_quiz(admin_auth, exam_type_id, "q-unstarted", started=False)
        self._pool_quiz(admin_auth, exam_type_id, "q-started", started=True)
        res = client.patch(
            f"/api/materials/{created['id']}",
            headers=admin_headers,
            json={"content": "isi baru"},
        )
        assert res.status_code == 200
        ids = {q["id"] for q in admin_auth.tables["quizzes"]}
        assert ids == {"q-started"}

    def test_delete_material_invalidates_pool(self, client, admin_auth, admin_headers):
        exam_type_id = self._exam_type(admin_auth)
        created = client.post(
            "/api/materials",
            headers=admin_headers,
            json={
                "subject": "IPA",
                "grade": 5,
                "exam_type_id": exam_type_id,
                "title": "x",
                "content": "isi",
            },
        ).json()
        self._pool_quiz(admin_auth, exam_type_id, "q-unstarted", started=False)
        self._pool_quiz(admin_auth, exam_type_id, "q-started", started=True)
        res = client.delete(f"/api/materials/{created['id']}", headers=admin_headers)
        assert res.status_code == 204
        ids = {q["id"] for q in admin_auth.tables["quizzes"]}
        assert ids == {"q-started"}