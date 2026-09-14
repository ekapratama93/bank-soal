import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from .config import settings
from .routers import auth, exam_types, materials, quiz, subjects
from .supabase_client import close_http_client

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    yield
    close_http_client()


app = FastAPI(title="Bank Soal API", lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=[settings.frontend_origin],
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(auth.router)
app.include_router(exam_types.router)
app.include_router(materials.router)
app.include_router(quiz.router)
app.include_router(subjects.router)


@app.exception_handler(RuntimeError)
async def runtime_error_handler(request: Request, exc: RuntimeError):
    return JSONResponse(
        status_code=503,
        content={"detail": "Server belum dikonfigurasi dengan benar. Hubungi admin."},
    )


@app.exception_handler(Exception)
async def unhandled_exception_handler(request: Request, exc: Exception):
    """Jaring pengaman terakhir: setiap galat tak terduga dicatat lengkap
    dengan traceback (sebelumnya hilang begitu saja ke stderr uvicorn tanpa
    konteks) dan direspons dengan body generik yang aman, bukan bocoran
    internal."""
    logger.exception("Unhandled error on %s %s", request.method, request.url.path)
    return JSONResponse(
        status_code=500,
        content={"detail": "Terjadi kesalahan pada server."},
    )


@app.get("/api/health")
def health():
    return {"status": "ok"}
