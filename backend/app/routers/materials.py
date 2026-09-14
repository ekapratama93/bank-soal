from fastapi import APIRouter, Depends, File, Form, Header, HTTPException, UploadFile
from pydantic import BaseModel, Field
from supabase_auth.errors import AuthRetryableError

from ..file_extract import MAX_FILE_BYTES, FileExtractError, extract_text
from ..supabase_client import get_supabase

router = APIRouter(prefix="/api/materials", tags=["materials"])


def require_admin(authorization: str | None = Header(default=None)) -> dict:
    if not authorization or not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="Harus login sebagai admin")
    token = authorization.removeprefix("Bearer ").strip()
    sb = get_supabase()
    try:
        user = sb.auth.get_user(token).user
    except AuthRetryableError:
        # Gangguan jaringan/server Supabase — bukan token invalid. Biarkan
        # jadi 5xx (handler galat umum di main.py) alih-alih salah melaporkan
        # "sesi tidak valid" dan menyuruh admin login ulang tanpa gunanya.
        raise
    except Exception:
        raise HTTPException(status_code=401, detail="Sesi tidak valid, silakan login ulang")
    if not user:
        raise HTTPException(status_code=401, detail="Sesi tidak valid, silakan login ulang")
    profile = sb.table("profiles").select("role").eq("id", user.id).execute()
    if not profile.data or profile.data[0].get("role") != "admin":
        raise HTTPException(status_code=403, detail="Akun ini bukan admin")
    return {"id": user.id, "email": user.email}


def _validate_subject(sb, subject: str):
    res = sb.table("subjects").select("id").eq("name", subject).execute()
    if not res.data:
        raise HTTPException(status_code=422, detail="Mata pelajaran tidak valid")


def _validate_exam_type(sb, exam_type_id: str):
    res = sb.table("exam_types").select("id").eq("id", exam_type_id).execute()
    if not res.data:
        raise HTTPException(status_code=422, detail="Tipe ujian tidak valid")
    return exam_type_id


class MaterialIn(BaseModel):
    subject: str
    grade: int = Field(ge=1, le=12)
    exam_type_id: str
    title: str
    content: str


class MaterialUpdate(BaseModel):
    subject: str | None = None
    grade: int | None = Field(default=None, ge=1, le=12)
    exam_type_id: str | None = None
    title: str | None = None
    content: str | None = None


def _invalidate_pool(sb, subject: str, grade: int, exam_type_id: str) -> int:
    """Hapus paket yang belum dimulai agar batch berikutnya dibuat ulang
    dari materi terbaru (materi lama tetap dipakai sebagai konteks)."""
    removed = (
        sb.table("quizzes")
        .delete()
        .eq("subject", subject)
        .eq("grade", grade)
        .eq("exam_type_id", exam_type_id)
        .eq("started", False)
        .execute()
    )
    return len(removed.data)


@router.get("")
def list_materials(
    subject: str | None = None,
    grade: int | None = None,
    admin: dict = Depends(require_admin),
):
    sb = get_supabase()
    query = sb.table("materials").select("*, exam_types(name)").order("created_at", desc=True)
    if subject:
        query = query.eq("subject", subject)
    if grade is not None:
        query = query.eq("grade", grade)
    res = query.execute()
    return res.data


@router.post("", status_code=201)
def create_material(body: MaterialIn, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    _validate_subject(sb, body.subject)
    _validate_exam_type(sb, body.exam_type_id)
    res = (
        sb.table("materials")
        .insert(
            {
                "subject": body.subject,
                "grade": body.grade,
                "exam_type_id": body.exam_type_id,
                "title": body.title,
                "content": body.content,
                "created_by": admin["email"],
            }
        )
        .execute()
    )
    _invalidate_pool(sb, body.subject, body.grade, body.exam_type_id)
    return res.data[0]


@router.post("/upload", status_code=201)
async def upload_material(
    file: UploadFile | None = File(default=None),
    subject: str = Form(...),
    grade: int = Form(...),
    exam_type_id: str = Form(...),
    title: str = Form(default=""),
    content: str = Form(default=""),
    admin: dict = Depends(require_admin),
):
    if not 1 <= grade <= 12:
        raise HTTPException(status_code=422, detail="Kelas tidak valid")
    sb = get_supabase()
    _validate_subject(sb, subject)
    _validate_exam_type(sb, exam_type_id)

    parts = []
    file_name = None
    if file is not None and file.filename:
        # Baca terbatas, bukan file.read() tanpa batas — supaya unggahan raksasa
        # tidak dulu masuk seluruhnya ke memori sebelum pengecekan ukuran di
        # extract_text() sempat menolaknya.
        data = await file.read(MAX_FILE_BYTES + 1)
        if len(data) > MAX_FILE_BYTES:
            raise HTTPException(
                status_code=422,
                detail=f"Ukuran file melebihi {MAX_FILE_BYTES // (1024 * 1024)} MB.",
            )
        try:
            parts.append(extract_text(file.filename, data))
            file_name = file.filename
        except FileExtractError as e:
            # File tidak terbaca (mis. PDF hasil scan): diteruskan hanya jika ada isi teks
            if not content.strip():
                raise HTTPException(status_code=422, detail=str(e))

    if content.strip():
        parts.append(content.strip())

    if not parts:
        raise HTTPException(
            status_code=422,
            detail="Isi materi kosong. Unggah file atau tulis isi materi.",
        )
    full_content = "\n\n".join(parts)

    final_title = title.strip() or (file_name or "materi").rsplit(".", 1)[0]
    res = (
        sb.table("materials")
        .insert(
            {
                "subject": subject,
                "grade": grade,
                "exam_type_id": exam_type_id,
                "title": final_title,
                "content": full_content,
                "file_name": file_name,
                "created_by": admin["email"],
            }
        )
        .execute()
    )
    _invalidate_pool(sb, subject, grade, exam_type_id)
    return res.data[0]


@router.patch("/{material_id}")
def update_material(
    material_id: str, body: MaterialUpdate, admin: dict = Depends(require_admin)
):
    sb = get_supabase()
    existing = sb.table("materials").select("*").eq("id", material_id).execute()
    if not existing.data:
        raise HTTPException(status_code=404, detail="Materi tidak ditemukan")
    current = existing.data[0]

    updates: dict = {}
    if body.subject is not None and body.subject != current["subject"]:
        _validate_subject(sb, body.subject)
        updates["subject"] = body.subject
    if body.grade is not None and body.grade != current["grade"]:
        updates["grade"] = body.grade
    if body.exam_type_id is not None and body.exam_type_id != current["exam_type_id"]:
        _validate_exam_type(sb, body.exam_type_id)
        updates["exam_type_id"] = body.exam_type_id
    if body.title is not None:
        if not body.title.strip():
            raise HTTPException(status_code=422, detail="Judul materi kosong")
        updates["title"] = body.title.strip()
    if body.content is not None:
        if not body.content.strip():
            raise HTTPException(status_code=422, detail="Isi materi kosong")
        updates["content"] = body.content

    if updates:
        sb.table("materials").update(updates).eq("id", material_id).execute()
        _invalidate_pool(
            sb,
            updates.get("subject", current["subject"]),
            updates.get("grade", current["grade"]),
            updates.get("exam_type_id", current["exam_type_id"]),
        )
        if updates.get("subject") or updates.get("grade") or updates.get("exam_type_id"):
            # Jika pindah kombinasi, bersihkan juga pool kombinasi lama
            _invalidate_pool(sb, current["subject"], current["grade"], current["exam_type_id"])

    res = sb.table("materials").select("*").eq("id", material_id).execute()
    return res.data[0] if res.data else None


@router.delete("/{material_id}", status_code=204)
def delete_material(material_id: str, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    existing = sb.table("materials").select("*").eq("id", material_id).execute()
    if not existing.data:
        raise HTTPException(status_code=404, detail="Materi tidak ditemukan")
    current = existing.data[0]
    sb.table("materials").delete().eq("id", material_id).execute()
    _invalidate_pool(sb, current["subject"], current["grade"], current["exam_type_id"])
    return None