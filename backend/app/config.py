from pydantic import field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    openrouter_api_key: str = ""
    openrouter_model: str = "z-ai/glm-4.5-air:free"
    # Model bergambar (image output) untuk soal yang butuh ilustrasi/diagram.
    openrouter_image_model: str = "google/gemini-2.5-flash-image-preview:free"
    supabase_url: str = ""
    supabase_service_key: str = ""
    frontend_origin: str = "http://localhost:5173"

    openrouter_url: str = "https://openrouter.ai/api/v1/chat/completions"

    @field_validator("supabase_url", "openrouter_url")
    @classmethod
    def _must_be_http_url_if_set(cls, v: str, info) -> str:
        # Kosong tetap boleh — itu ditangani terpisah (RuntimeError → 503,
        # lihat supabase_client.py) sebagai "belum dikonfigurasi". Tapi nilai
        # yang DIISI namun salah bentuk (typo .env) sebelumnya baru gagal
        # secara samar pada request pertama yang memakainya, lama setelah
        # deploy. Gagal cepat di sini, saat proses dimulai.
        if v and not (v.startswith("http://") or v.startswith("https://")):
            raise ValueError(
                f"{info.field_name} harus berupa URL http(s):// yang valid, dapat: {v!r}"
            )
        return v


settings = Settings()
