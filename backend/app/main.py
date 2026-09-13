import logging

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from .config import settings
from .routers import auth, exam_types, materials, quiz, subjects

logging.basicConfig(level=logging.INFO)

app = FastAPI(title="Bank Soal API")

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


@app.get("/api/health")
def health():
    return {"status": "ok"}