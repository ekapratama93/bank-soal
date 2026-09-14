import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../api/client", async () => {
  const actual = await vi.importActual<typeof import("../api/client")>(
    "../api/client"
  );
  return {
    ...actual,
    submitQuiz: vi.fn(),
    getAttempts: vi.fn(),
  };
});

import { ApiError, getAttempts, submitQuiz } from "../api/client";
import { backoffMs, submitWithReconcile } from "./quizSubmit";

const submitMock = vi.mocked(submitQuiz);
const attemptsMock = vi.mocked(getAttempts);

const ANSWERS = { "0": 1 };

function storedAttempt(overrides: Record<string, unknown> = {}) {
  return {
    quiz_id: "q1",
    subject: "IPA",
    grade: 5,
    exam_type: "Ujian Harian",
    nilai: 80,
    per_question: [],
    expired: false,
    submitted_at: "2026-01-01T00:00:00Z",
    ...overrides,
  } as never;
}

beforeEach(() => {
  submitMock.mockReset();
  attemptsMock.mockReset();
});

describe("pengumpulan jawaban", () => {
  it("berhasil sekali jalan", async () => {
    submitMock.mockResolvedValue({ quiz_id: "q1", nilai: 90 } as never);
    const out = await submitWithReconcile("q1", ANSWERS, 0);
    expect(out.status).toBe("success");
    expect(submitMock).toHaveBeenCalledTimes(1);
    expect(attemptsMock).not.toHaveBeenCalled();
  });

  it("tidak mengirim ulang kalau ternyata sudah masuk ke server", async () => {
    // Respons hilang di jalan, padahal server sudah menyimpannya.
    submitMock.mockRejectedValue(new ApiError("Tidak ada koneksi ke server.", 0));
    attemptsMock.mockResolvedValue([storedAttempt()]);

    const out = await submitWithReconcile("q1", ANSWERS, 0);

    expect(out.status).toBe("already-submitted");
    if (out.status === "already-submitted") expect(out.result.nilai).toBe(80);
    // Inti pengamanan: POST kedua tidak pernah terjadi.
    expect(submitMock).toHaveBeenCalledTimes(1);
  });

  it("mengabaikan attempt paket lain saat rekonsiliasi", async () => {
    submitMock.mockRejectedValue(new ApiError("Tidak ada koneksi ke server.", 0));
    attemptsMock.mockResolvedValue([storedAttempt({ quiz_id: "paket-lain" })]);

    const out = await submitWithReconcile("q1", ANSWERS, 0);
    expect(out.status).toBe("retry");
  });

  it("mengabaikan attempt yang belum dikumpulkan", async () => {
    submitMock.mockRejectedValue(new ApiError("Server bermasalah.", 500));
    attemptsMock.mockResolvedValue([storedAttempt({ submitted_at: null })]);

    const out = await submitWithReconcile("q1", ANSWERS, 0);
    expect(out.status).toBe("retry");
  });

  it("menjadwalkan percobaan ulang saat jaringan putus", async () => {
    submitMock.mockRejectedValue(new ApiError("Tidak ada koneksi ke server.", 0));
    attemptsMock.mockResolvedValue([]);

    const out = await submitWithReconcile("q1", ANSWERS, 2);
    expect(out.status).toBe("retry");
    if (out.status === "retry") expect(out.retryInMs).toBe(4000);
  });

  it("tetap bisa dicoba ulang walau rekonsiliasi ikut gagal", async () => {
    submitMock.mockRejectedValue(new ApiError("Tidak ada koneksi ke server.", 0));
    attemptsMock.mockRejectedValue(new ApiError("Tidak ada koneksi.", 0));

    const out = await submitWithReconcile("q1", ANSWERS, 0);
    expect(out.status).toBe("retry");
    expect(submitMock).toHaveBeenCalledTimes(1);
  });

  it("menyerah pada galat 4xx yang tidak akan pulih sendiri", async () => {
    submitMock.mockRejectedValue(new ApiError("Kuis tidak ditemukan", 404));
    attemptsMock.mockResolvedValue([]);

    const out = await submitWithReconcile("q1", ANSWERS, 0);
    expect(out.status).toBe("fatal");
    if (out.status === "fatal") expect(out.error).toBe("Kuis tidak ditemukan");
  });

  it("mencoba ulang galat server 5xx", async () => {
    submitMock.mockRejectedValue(new ApiError("Gagal mengoreksi.", 502));
    attemptsMock.mockResolvedValue([]);

    const out = await submitWithReconcile("q1", ANSWERS, 0);
    expect(out.status).toBe("retry");
  });
});

describe("jeda antar percobaan", () => {
  it("naik dua kali lipat dan berhenti di 30 detik", () => {
    expect(backoffMs(0)).toBe(2000);
    expect(backoffMs(1)).toBe(2000);
    expect(backoffMs(2)).toBe(4000);
    expect(backoffMs(3)).toBe(8000);
    expect(backoffMs(10)).toBe(30000);
  });
});
