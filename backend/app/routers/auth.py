from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from limits import parse as parse_limit
from slowapi import Limiter
from slowapi.util import get_remote_address
from supabase_auth.errors import AuthRetryableError

from ..supabase_client import get_fresh_client, get_supabase

router = APIRouter(prefix="/api/auth", tags=["auth"])

# key_func di sini tidak dipakai secara langsung (lihat login() di bawah — kita
# mengecek limit manual per email, bukan lewat dekorator @limiter.limit, karena
# key_func slowapi dipanggil sinkron dan tidak bisa membaca body request yang
# baru terparse async). Limiter tetap dipakai untuk mesin hitung/penyimpanannya.
limiter = Limiter(key_func=get_remote_address)
LOGIN_LIMIT = parse_limit("5/15 minutes")


class LoginRequest(BaseModel):
    email: str
    password: str


@router.post("/login")
def login(body: LoginRequest):
    email_key = body.email.strip().lower()
    if not limiter.limiter.hit(LOGIN_LIMIT, email_key):
        raise HTTPException(
            status_code=429,
            detail="Terlalu banyak percobaan login. Coba lagi dalam beberapa menit.",
            headers={"Retry-After": "900"},
        )

    auth_sb = get_fresh_client()
    try:
        res = auth_sb.auth.sign_in_with_password(
            {"email": body.email, "password": body.password}
        )
    except AuthRetryableError:
        # Gangguan jaringan/server Supabase — bukan kredensial salah. Biarkan
        # jadi 5xx (lewat handler umum di main.py) alih-alih disalahartikan
        # sebagai "email atau password salah".
        raise
    except Exception:
        raise HTTPException(status_code=401, detail="Email atau password salah")
    user = res.user
    session = res.session
    if not user or not session:
        raise HTTPException(status_code=401, detail="Email atau password salah")

    # Cek role memakai klien service key (bukan klien yang baru login)
    sb = get_supabase()
    profile = (
        sb.table("profiles").select("role").eq("id", user.id).execute()
    )
    role = profile.data[0].get("role") if profile.data else None
    if role != "admin":
        raise HTTPException(status_code=403, detail="Akun ini bukan admin")
    return {
        "access_token": session.access_token,
        "email": user.email,
        "role": role,
    }
