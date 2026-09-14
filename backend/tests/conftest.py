import uuid

import pytest
from fastapi.testclient import TestClient
from postgrest.exceptions import APIError


class FakeResult:
    def __init__(self, data):
        self.data = data


class Query:
    """Meniru rantai query supabase-py: table().select().eq().execute()"""

    def __init__(self, sb, table):
        self.sb = sb
        self.table = table
        self.op = "select"
        self.filters = []
        self._single = False
        self._order = None

    def select(self, *args, **kwargs):
        return self

    def eq(self, col, val):
        self.filters.append((col, val))
        return self

    def order(self, col, desc=False):
        self._order = (col, desc)
        return self

    def single(self):
        self._single = True
        return self

    def insert(self, row):
        self.op = "insert"
        self.row = row
        return self

    def delete(self):
        self.op = "delete"
        return self

    def update(self, data):
        self.op = "update"
        self.updates = data
        return self

    def execute(self):
        rows = self.sb.tables.setdefault(self.table, [])
        if self.op == "insert":
            row = dict(self.row)
            row.setdefault("id", str(uuid.uuid4()))
            for cols in self.sb.unique_constraints.get(self.table, []):
                key = tuple(row.get(c) for c in cols)
                if any(key == tuple(r.get(c) for c in cols) for r in rows):
                    raise APIError(
                        {
                            "code": "23505",
                            "message": f"duplicate key value violates unique constraint on {cols}",
                        }
                    )
            rows.append(row)
            return FakeResult([row])
        if self.op == "delete":
            removed = [r for r in rows if all(r.get(c) == v for c, v in self.filters)]
            self.sb.tables[self.table] = [r for r in rows if r not in removed]
            for child_table, fk_col in self.sb.cascades.get(self.table, []):
                ids = {r.get("id") for r in removed}
                child_rows = self.sb.tables.setdefault(child_table, [])
                self.sb.tables[child_table] = [
                    r for r in child_rows if r.get(fk_col) not in ids
                ]
            return FakeResult(removed)
        if self.op == "update":
            updated = []
            for r in rows:
                if all(r.get(c) == v for c, v in self.filters):
                    r.update(self.updates)
                    updated.append(r)
            return FakeResult(updated)
        data = [r for r in rows if all(r.get(c) == v for c, v in self.filters)]
        if self._order:
            col, desc = self._order
            data = sorted(
                data,
                key=lambda r: (r.get(col) is None, r.get(col)),
                reverse=desc,
            )
        if self._single:
            # Meniru PostgREST: single() dengan 0 baris menghasilkan error PGRST116
            if not data:
                raise Exception(
                    "{'message': 'Cannot coerce the result to a single JSON object', "
                    "'code': 'PGRST116'}"
                )
            return FakeResult(data[0])
        return FakeResult(data)


class FakeUser:
    def __init__(self, id, email):
        self.id = id
        self.email = email


class FakeSession:
    def __init__(self, access_token):
        self.access_token = access_token


class FakeAuthResponse:
    def __init__(self, user, session):
        self.user = user
        self.session = session


class FakeAuth:
    def __init__(self):
        # email -> {"password", "id", "role"}
        self.accounts = {}
        self.jwt_user = None

    def sign_in_with_password(self, creds):
        acc = self.accounts.get(creds["email"])
        if not acc or acc["password"] != creds["password"]:
            raise Exception("Invalid login credentials")
        user = FakeUser(acc["id"], creds["email"])
        session = FakeSession(f"tok-{acc['id']}")
        return FakeAuthResponse(user, session)

    def get_user(self, jwt):
        if jwt and jwt.startswith("tok-") and self.jwt_user is not None:
            return FakeAuthResponse(self.jwt_user, None)
        raise Exception("Invalid JWT")


class FakeSupabase:
    def __init__(self, tables=None):
        self.tables = {k: list(v) for k, v in (tables or {}).items()}
        self.auth = FakeAuth()
        # Sebagian kecil constraint DB sungguhan ditegakkan di sini secara
        # opsional — cukup yang dipakai test untuk membuktikan kode menangani
        # error unique-violation (23505) dari PostgREST, bukan mengabaikannya.
        self.unique_constraints: dict[str, list[tuple[str, ...]]] = {
            "attempts": [("quiz_id", "client_id")],
        }
        # Meniru "on delete cascade" schema.sql: {tabel_induk: [(tabel_anak, kolom_fk)]}
        self.cascades: dict[str, list[tuple[str, str]]] = {
            "quizzes": [("attempts", "quiz_id")],
        }

    @property
    def accounts(self):
        return self.auth.accounts

    @property
    def jwt_user(self):
        return self.auth.jwt_user

    @jwt_user.setter
    def jwt_user(self, value):
        self.auth.jwt_user = value

    def table(self, name):
        return Query(self, name)


@pytest.fixture
def sb():
    from app.subjects import DEFAULT_SUBJECTS

    fake = FakeSupabase()
    fake.tables["subjects"] = [
        {"id": f"sub-{i}", "name": name}
        for i, name in enumerate(DEFAULT_SUBJECTS, start=1)
    ]
    return fake


@pytest.fixture
def client(sb, monkeypatch):
    from app.main import app
    from app.routers import auth as auth_router
    from app.routers import exam_types as exam_types_router
    from app.routers import materials as materials_router
    from app.routers import quiz as quiz_router
    from app.routers import subjects as subjects_router

    monkeypatch.setattr(auth_router, "get_supabase", lambda: sb)
    monkeypatch.setattr(auth_router, "get_fresh_client", lambda: sb)
    monkeypatch.setattr(exam_types_router, "get_supabase", lambda: sb)
    monkeypatch.setattr(materials_router, "get_supabase", lambda: sb)
    monkeypatch.setattr(quiz_router, "get_supabase", lambda: sb)
    monkeypatch.setattr(subjects_router, "get_supabase", lambda: sb)
    # Limiter login bersifat module-level (bertahan antar test) — bersihkan
    # supaya percobaan login di satu test tidak memengaruhi test lain.
    auth_router.limiter.reset()
    return TestClient(app)


@pytest.fixture
def admin_auth(sb):
    """Akun admin terdaftar + JWT valid di fake auth."""
    sb.accounts["admin@sekolah.id"] = {
        "password": "rahasia",
        "id": "00000000-0000-0000-0000-000000000001",
    }
    sb.jwt_user = FakeUser(
        "00000000-0000-0000-0000-000000000001", "admin@sekolah.id"
    )
    sb.tables["profiles"] = [
        {
            "id": "00000000-0000-0000-0000-000000000001",
            "email": "admin@sekolah.id",
            "role": "admin",
        }
    ]
    return sb


@pytest.fixture
def admin_headers():
    return {"Authorization": "Bearer tok-00000000-0000-0000-0000-000000000001"}