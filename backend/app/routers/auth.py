from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from ..supabase_client import get_fresh_client, get_supabase

router = APIRouter(prefix="/api/auth", tags=["auth"])


class LoginRequest(BaseModel):
    email: str
    password: str


@router.post("/login")
def login(body: LoginRequest):
    auth_sb = get_fresh_client()
    try:
        res = auth_sb.auth.sign_in_with_password(
            {"email": body.email, "password": body.password}
        )
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