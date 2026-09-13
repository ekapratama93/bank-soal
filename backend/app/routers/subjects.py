from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from ..supabase_client import get_supabase
from .materials import require_admin

router = APIRouter(prefix="/api/subjects", tags=["subjects"])


class SubjectIn(BaseModel):
    name: str


@router.get("")
def list_subjects():
    sb = get_supabase()
    res = sb.table("subjects").select("*").order("name").execute()
    return res.data


@router.post("", status_code=201)
def create_subject(body: SubjectIn, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    name = body.name.strip()
    if not name:
        raise HTTPException(status_code=422, detail="Nama mata pelajaran kosong")
    existing = sb.table("subjects").select("id").eq("name", name).execute()
    if existing.data:
        raise HTTPException(status_code=409, detail="Mata pelajaran sudah ada")
    res = sb.table("subjects").insert({"name": name}).execute()
    return res.data[0]


@router.delete("/{subject_id}", status_code=204)
def delete_subject(subject_id: str, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    existing = sb.table("subjects").select("name").eq("id", subject_id).execute()
    if not existing.data:
        raise HTTPException(status_code=404, detail="Mata pelajaran tidak ditemukan")
    name = existing.data[0]["name"]

    used_materials = sb.table("materials").select("id").eq("subject", name).execute()
    if used_materials.data:
        raise HTTPException(
            status_code=409,
            detail="Mata pelajaran masih dipakai materi. Hapus materinya dulu.",
        )
    used_quizzes = sb.table("quizzes").select("id").eq("subject", name).execute()
    if used_quizzes.data:
        raise HTTPException(
            status_code=409,
            detail="Mata pelajaran masih dipakai kuis. Reset pool dulu sebelum menghapus.",
        )
    sb.table("subjects").delete().eq("id", subject_id).execute()
    return None