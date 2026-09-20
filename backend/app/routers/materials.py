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


def _validate_exam_type(sb, exam_type_id: str):
    res = sb.table("exam_types").select("id").eq("id", exam_type_id).execute()
    if not res.data:
        raise HTTPException(status_code=422, detail="Tipe ujian tidak valid")
    return exam_type_id


def resolve_subject(sb, *, subject_id=None, subject=None) -> dict:
    """Terima subject_id (baru) ATAU subject (nama, kompatibel kode lama) —
    kembalikan baris subjects. Bila keduanya diisi, subject_id yang menang.
    422 bila keduanya kosong, atau tidak ditemukan."""
    if subject_id:
        res = sb.table("subjects").select("*").eq("id", subject_id).execute()
        if res.data:
            return res.data[0]
        raise HTTPException(status_code=422, detail="Mata pelajaran tidak valid")
    if subject:
        res = sb.table("subjects").select("*").eq("name", subject).execute()
        if res.data:
            return res.data[0]
        raise HTTPException(status_code=422, detail="Mata pelajaran tidak valid")
    raise HTTPException(
        status_code=422, detail="subject atau subject_id wajib diisi"
    )


def subject_columns(sub: dict) -> dict:
    """Kolom subject yang ditulis saat insert/update materi & kuis selama masa
    transisi: FK subject_id + teks subject (nama, dibaca kode lama).
    Fase 2: hapus key "subject" di sini, deploy, lalu jalankan SQL fase 2."""
    return {"subject_id": sub["id"], "subject": sub["name"]}


def subject_names_map(sb) -> dict[str, str]:
    """Peta {id: nama} subjects untuk mendekorasi respons — pengganti embed
    PostgREST supaya ramah dengan fake Supabase di test."""
    res = sb.table("subjects").select("id, name").execute()
    return {s["id"]: s["name"] for s in (res.data or [])}


def subject_display_name(
    subject_id: str | None, row_subject: str | None, names: dict[str, str]
) -> str:
    """Nama tampilan mapel — peta subjects adalah sumber kebenaran (rename
    langsung tampil); teks pada baris hanya fallback untuk baris lawas tanpa
    subject_id (baris yang ditulis backend lama selama jendela deploy)."""
    return names.get(subject_id, "") or (row_subject or "")


def decorate_subject(sb, row: dict, names: dict[str, str] | None = None) -> dict:
    """Pastikan respons selalu membawa subject (nama tampil) + subject_id.
    Nama diambil dari peta subjects supaya rename langsung tampil; teks baris
    dipakai hanya bila baris belum punya subject_id."""
    out = dict(row)
    if names is None:
        names = subject_names_map(sb)
    out["subject"] = subject_display_name(
        out.get("subject_id"), out.get("subject"), names
    )
    return out


def _row_subject_id(sb, row: dict) -> str | None:
    """subject_id dari baris materi/kuis — baris yang ditulis backend lama
    selama jendela deploy hanya membawa teks subject, jadi diresolvakan."""
    if row.get("subject_id"):
        return row["subject_id"]
    name = row.get("subject")
    if not name:
        return None
    try:
        return resolve_subject(sb, subject=name)["id"]
    except HTTPException:
        return None


class MaterialIn(BaseModel):
    subject_id: str | None = None
    subject: str | None = None
    grade: int = Field(ge=1, le=12)
    exam_type_id: str
    title: str
    content: str


class MaterialUpdate(BaseModel):
    subject_id: str | None = None
    subject: str | None = None
    grade: int | None = Field(default=None, ge=1, le=12)
    exam_type_id: str | None = None
    title: str | None = None
    content: str | None = None


def _invalidate_pool(sb, subject_id: str, grade: int, exam_type_id: str) -> int:
    """Hapus paket yang belum dimulai agar batch berikutnya dibuat ulang
    dari materi terbaru (materi lama tetap dipakai sebagai konteks)."""
    removed = (
        sb.table("quizzes")
        .delete()
        .eq("subject_id", subject_id)
        .eq("grade", grade)
        .eq("exam_type_id", exam_type_id)
        .eq("started", False)
        .execute()
    )
    return len(removed.data)


@router.get("")
def list_materials(
    subject_id: str | None = None,
    subject: str | None = None,
    grade: int | None = None,
    admin: dict = Depends(require_admin),
):
    sb = get_supabase()
    query = sb.table("materials").select("*, exam_types(name)").order("created_at", desc=True)
    if subject_id:
        query = query.eq("subject_id", subject_id)
    elif subject:
        # Filter legacy (nama) — diresolvakan ke subject_id supaya memakai
        # indeks dan tetap bekerja setelah kolom teks dihapus di fase 2.
        sub = resolve_subject(sb, subject=subject)
        query = query.eq("subject_id", sub["id"])
    if grade is not None:
        query = query.eq("grade", grade)
    res = query.execute()
    names = subject_names_map(sb)
    return [decorate_subject(sb, row, names) for row in res.data]


@router.post("", status_code=201)
def create_material(body: MaterialIn, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    sub = resolve_subject(sb, subject_id=body.subject_id, subject=body.subject)
    _validate_exam_type(sb, body.exam_type_id)
    res = (
        sb.table("materials")
        .insert(
            {
                **subject_columns(sub),
                "grade": body.grade,
                "exam_type_id": body.exam_type_id,
                "title": body.title,
                "content": body.content,
                "created_by": admin["email"],
            }
        )
        .execute()
    )
    _invalidate_pool(sb, sub["id"], body.grade, body.exam_type_id)
    return decorate_subject(sb, res.data[0])


@router.post("/upload", status_code=201)
async def upload_material(
    file: UploadFile | None = File(default=None),
    subject_id: str = Form(default=""),
    subject: str = Form(default=""),
    grade: int = Form(...),
    exam_type_id: str = Form(...),
    title: str = Form(default=""),
    content: str = Form(default=""),
    admin: dict = Depends(require_admin),
):
    if not 1 <= grade <= 12:
        raise HTTPException(status_code=422, detail="Kelas tidak valid")
    sb = get_supabase()
    sub = resolve_subject(sb, subject_id=subject_id or None, subject=subject or None)
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
                **subject_columns(sub),
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
    _invalidate_pool(sb, sub["id"], grade, exam_type_id)
    return decorate_subject(sb, res.data[0])


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
    if body.subject_id is not None or body.subject is not None:
        sub = resolve_subject(sb, subject_id=body.subject_id, subject=body.subject)
        if sub["id"] != current.get("subject_id"):
            updates.update(subject_columns(sub))
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
        current_subject_id = _row_subject_id(sb, current)
        new_subject_id = (
            updates["subject_id"]
            if "subject_id" in updates
            else current_subject_id
        )
        if new_subject_id:
            _invalidate_pool(sb, new_subject_id, updates.get("grade", current["grade"]), updates.get("exam_type_id", current["exam_type_id"]))
        if updates.get("subject_id") or updates.get("grade") or updates.get("exam_type_id"):
            # Jika pindah kombinasi, bersihkan juga pool kombinasi lama
            if current_subject_id:
                _invalidate_pool(sb, current_subject_id, current["grade"], current["exam_type_id"])

    res = (
        sb.table("materials")
        .select("*, exam_types(name)")
        .eq("id", material_id)
        .execute()
    )
    return decorate_subject(sb, res.data[0]) if res.data else None


@router.delete("/{material_id}", status_code=204)
def delete_material(material_id: str, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    existing = sb.table("materials").select("*").eq("id", material_id).execute()
    if not existing.data:
        raise HTTPException(status_code=404, detail="Materi tidak ditemukan")
    current = existing.data[0]
    sb.table("materials").delete().eq("id", material_id).execute()
    subject_id = _row_subject_id(sb, current)
    if subject_id:
        _invalidate_pool(sb, subject_id, current["grade"], current["exam_type_id"])
    return None