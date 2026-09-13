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


settings = Settings()