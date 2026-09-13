import uuid

from supabase import Client

from .supabase_client import QUESTION_IMAGES_BUCKET, ensure_bucket

_EXT_BY_CONTENT_TYPE = {
    "image/png": "png",
    "image/jpeg": "jpg",
    "image/jpg": "jpg",
    "image/webp": "webp",
}


def upload_image_bytes(sb: Client, data: bytes, content_type: str) -> str:
    """Unggah gambar ke bucket Supabase Storage dan kembalikan URL publiknya."""
    ensure_bucket(sb)
    ext = _EXT_BY_CONTENT_TYPE.get(content_type, "jpg")
    path = f"{uuid.uuid4()}.{ext}"
    sb.storage.from_(QUESTION_IMAGES_BUCKET).upload(
        path, data, file_options={"content-type": content_type}
    )
    return sb.storage.from_(QUESTION_IMAGES_BUCKET).get_public_url(path)
