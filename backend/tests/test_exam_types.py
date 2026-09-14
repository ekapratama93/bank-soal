import pytest


class TestExamTypes:
    def test_public_list(self, client, sb):
        sb.tables["exam_types"] = [
            {"id": "et-2", "name": "Ujian Semester", "jumlah_soal": None, "durasi_menit": None},
            {"id": "et-1", "name": "Ujian Harian", "jumlah_soal": None, "durasi_menit": None},
        ]
        res = client.get("/api/exam-types")
        assert res.status_code == 200
        names = [t["name"] for t in res.json()]
        assert names == ["Ujian Harian", "Ujian Semester"]

    def test_create(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Ujian Harian", "jumlah_soal": 10, "durasi_menit": 30},
            headers=admin_headers,
        )
        assert res.status_code == 201
        body = res.json()
        assert body["name"] == "Ujian Harian"
        assert body["jumlah_soal"] == 10

    def test_create_invalid_range(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Coba", "jumlah_soal": 3, "durasi_menit": None},
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_create_with_tipe_soal(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={
                "name": "Ujian Campuran",
                "tipe_soal": {"pilihan_ganda": 10, "isian": 5, "deskripsi": 5},
            },
            headers=admin_headers,
        )
        assert res.status_code == 201
        body = res.json()
        # tipe berjumlah 0 dibuang (dinormalisasi)
        assert body["tipe_soal"] == {"pilihan_ganda": 10, "isian": 5, "deskripsi": 5}

    def test_create_tipe_soal_invalid_type_name(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Coba", "tipe_soal": {"esai": 10}},
            headers=admin_headers,
        )
        assert res.status_code == 422
        assert "Tipe soal tidak valid" in res.json()["detail"]

    def test_create_tipe_soal_total_too_small(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Coba", "tipe_soal": {"pilihan_ganda": 2, "deskripsi": 1}},
            headers=admin_headers,
        )
        assert res.status_code == 422
        assert "5-50" in res.json()["detail"]

    def test_create_tipe_soal_total_too_big(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Coba", "tipe_soal": {"pilihan_ganda": 40, "isian": 20}},
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_create_tipe_soal_negative_count(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Coba", "tipe_soal": {"pilihan_ganda": -1, "isian": 10}},
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_create_with_poin_per_tipe(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={
                "name": "Ujian Berbobot",
                "poin_per_tipe": {"pilihan_ganda": 2, "isian": 5, "deskripsi": 10},
            },
            headers=admin_headers,
        )
        assert res.status_code == 201
        body = res.json()
        # tipe yang tidak disebut tidak ditambahkan otomatis (default 1 poin di submit)
        assert body["poin_per_tipe"] == {
            "pilihan_ganda": 2,
            "isian": 5,
            "deskripsi": 10,
        }

    def test_create_poin_per_tipe_partial(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Coba", "poin_per_tipe": {"deskripsi": 10}},
            headers=admin_headers,
        )
        assert res.status_code == 201
        assert res.json()["poin_per_tipe"] == {"deskripsi": 10}

    def test_create_poin_per_tipe_invalid_key(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Coba", "poin_per_tipe": {"esai": 10}},
            headers=admin_headers,
        )
        assert res.status_code == 422
        assert "Tipe soal tidak valid" in res.json()["detail"]

    def test_create_poin_per_tipe_zero_fails(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Coba", "poin_per_tipe": {"pilihan_ganda": 0}},
            headers=admin_headers,
        )
        assert res.status_code == 422
        assert "1-100" in res.json()["detail"]

    def test_create_poin_per_tipe_too_big(self, client, admin_auth, admin_headers):
        res = client.post(
            "/api/exam-types",
            json={"name": "Coba", "poin_per_tipe": {"pilihan_ganda": 101}},
            headers=admin_headers,
        )
        assert res.status_code == 422

    def test_update_poin_per_tipe(self, client, admin_auth, admin_headers):
        sb = admin_auth
        sb.tables["exam_types"] = [
            {"id": "et-1", "name": "Lama", "jumlah_soal": None, "durasi_menit": None}
        ]
        res = client.patch(
            "/api/exam-types/et-1",
            json={"name": "Lama", "poin_per_tipe": {"deskripsi": 10}},
            headers=admin_headers,
        )
        assert res.status_code == 200
        assert sb.tables["exam_types"][0]["poin_per_tipe"] == {"deskripsi": 10}

    def test_update(self, client, admin_auth, admin_headers):
        sb = admin_auth
        sb.tables["exam_types"] = [
            {"id": "et-1", "name": "Lama", "jumlah_soal": None, "durasi_menit": None}
        ]
        res = client.patch(
            "/api/exam-types/et-1",
            json={"name": "Baru", "jumlah_soal": 20, "durasi_menit": 45},
            headers=admin_headers,
        )
        assert res.status_code == 200
        assert res.json()["name"] == "Baru"
        assert sb.tables["exam_types"][0]["jumlah_soal"] == 20

    def test_delete_free(self, client, admin_auth, admin_headers):
        sb = admin_auth
        sb.tables["exam_types"] = [
            {"id": "et-1", "name": "Lama", "jumlah_soal": None, "durasi_menit": None}
        ]
        res = client.delete("/api/exam-types/et-1", headers=admin_headers)
        assert res.status_code == 204
        assert sb.tables["exam_types"] == []

    def test_delete_blocked_by_material(self, client, admin_auth, admin_headers):
        sb = admin_auth
        sb.tables["exam_types"] = [
            {"id": "et-1", "name": "Lama", "jumlah_soal": None, "durasi_menit": None}
        ]
        sb.tables["materials"] = [{"id": "m-1", "exam_type_id": "et-1"}]
        res = client.delete("/api/exam-types/et-1", headers=admin_headers)
        assert res.status_code == 409
        assert "materi" in res.json()["detail"]

    def test_delete_blocked_by_quiz(self, client, admin_auth, admin_headers):
        sb = admin_auth
        sb.tables["exam_types"] = [
            {"id": "et-1", "name": "Lama", "jumlah_soal": None, "durasi_menit": None}
        ]
        sb.tables["quizzes"] = [{"id": "q-1", "exam_type_id": "et-1"}]
        res = client.delete("/api/exam-types/et-1", headers=admin_headers)
        assert res.status_code == 409
        assert "kuis" in res.json()["detail"]

    def test_delete_not_found(self, client, admin_auth, admin_headers):
        res = client.delete("/api/exam-types/tidak-ada", headers=admin_headers)
        assert res.status_code == 404

    def test_delete_race_fk_violation_becomes_409(
        self, client, admin_auth, admin_headers, monkeypatch
    ):
        """Sesuatu mulai memakai exam_type di celah antara pengecekan dan
        delete (mis. materi baru disisipkan tepat setelah pre-check lolos).
        DB menolak lewat foreign key — harus jadi 409 yang ramah, bukan 500."""
        from postgrest.exceptions import APIError

        from conftest import Query

        sb = admin_auth
        sb.tables["exam_types"] = [
            {"id": "et-1", "name": "Lama", "jumlah_soal": None, "durasi_menit": None}
        ]

        real_execute = Query.execute

        def flaky_execute(self):
            if self.table == "exam_types" and self.op == "delete":
                raise APIError({"code": "23503", "message": "foreign key violation"})
            return real_execute(self)

        monkeypatch.setattr(Query, "execute", flaky_execute)

        res = client.delete("/api/exam-types/et-1", headers=admin_headers)
        assert res.status_code == 409
        # Baris tidak boleh hilang — delete-nya sungguhan gagal di DB.
        assert sb.tables["exam_types"]

    def test_requires_admin(self, client, sb):
        res = client.get("/api/exam-types")  # publik
        assert res.status_code == 200
        res = client.post("/api/exam-types", json={"name": "X"})
        assert res.status_code == 401