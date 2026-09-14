import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { RouterProvider, createMemoryRouter } from "react-router-dom";

vi.mock("../api/client", async () => {
  const actual = await vi.importActual<typeof import("../api/client")>(
    "../api/client"
  );
  return {
    ...actual,
    getQuiz: vi.fn(),
    submitQuiz: vi.fn(),
    getAttempts: vi.fn(),
  };
});

import { ApiError, getAttempts, getQuiz, submitQuiz } from "../api/client";
import Quiz from "./Quiz";

const getQuizMock = vi.mocked(getQuiz);
const submitMock = vi.mocked(submitQuiz);
const attemptsMock = vi.mocked(getAttempts);

function quizResponse(expiresInMs = 30 * 60_000) {
  return {
    quiz_id: "q1",
    subject: "IPA",
    grade: 5,
    exam_type: "Ujian Harian",
    durasi_menit: 30,
    expires_at: new Date(Date.now() + expiresInMs).toISOString(),
    repeat: false,
    questions: [
      {
        nomor: 1,
        tipe: "isian" as const,
        pertanyaan: "Ibu kota Indonesia?",
      },
    ],
  };
}

function renderQuiz() {
  const router = createMemoryRouter(
    [
      { path: "/quiz/:quizId", element: <Quiz /> },
      { path: "/result/:quizId", element: <div>HALAMAN HASIL</div> },
      { path: "/", element: <div>HALAMAN BERANDA</div> },
    ],
    { initialEntries: ["/quiz/q1"] }
  );
  return { ...render(<RouterProvider router={router} />), router };
}

async function answerAndOpenReview() {
  const input = await screen.findByPlaceholderText("Tulis jawaban singkat di sini");
  fireEvent.change(input, { target: { value: "Jakarta" } });
  fireEvent.click(screen.getByText("Selesai & Tinjau"));
}

beforeEach(() => {
  getQuizMock.mockReset();
  submitMock.mockReset();
  attemptsMock.mockReset();
  localStorage.clear();
});

describe("halaman ujian", () => {
  it("menampilkan soal setelah paket dimuat", async () => {
    getQuizMock.mockResolvedValue(quizResponse());
    renderQuiz();
    expect(await screen.findByText("Ibu kota Indonesia?")).toBeDefined();
  });

  it("pindah ke halaman hasil setelah berhasil dikumpulkan", async () => {
    getQuizMock.mockResolvedValue(quizResponse());
    submitMock.mockResolvedValue({ quiz_id: "q1", nilai: 100 } as never);

    renderQuiz();
    await answerAndOpenReview();
    fireEvent.click(screen.getByText("Konfirmasi & Kumpulkan"));

    expect(await screen.findByText("HALAMAN HASIL")).toBeDefined();
  });

  it("tidak mengirim ulang kalau jawaban ternyata sudah masuk", async () => {
    getQuizMock.mockResolvedValue(quizResponse());
    submitMock.mockRejectedValue(new ApiError("Tidak ada koneksi ke server.", 0));
    attemptsMock.mockResolvedValue([
      {
        quiz_id: "q1",
        subject: "IPA",
        grade: 5,
        exam_type: "Ujian Harian",
        nilai: 80,
        per_question: [],
        expired: false,
        submitted_at: "2026-01-01T00:00:00Z",
      } as never,
    ]);

    renderQuiz();
    await answerAndOpenReview();
    fireEvent.click(screen.getByText("Konfirmasi & Kumpulkan"));

    expect(await screen.findByText("HALAMAN HASIL")).toBeDefined();
    expect(submitMock).toHaveBeenCalledTimes(1);
  });

  it("menawarkan percobaan ulang saat pengumpulan gagal", async () => {
    getQuizMock.mockResolvedValue(quizResponse());
    submitMock.mockRejectedValue(new ApiError("Tidak ada koneksi ke server.", 0));
    attemptsMock.mockResolvedValue([]);

    renderQuiz();
    await answerAndOpenReview();
    fireEvent.click(screen.getByText("Konfirmasi & Kumpulkan"));

    expect(await screen.findByText("Coba Sekarang")).toBeDefined();
    // Draf jawaban tetap tersimpan di perangkat.
    await waitFor(() =>
      expect(localStorage.getItem("bank-soal-quiz-drafts")).toContain("Jakarta")
    );
  });

  it("menahan navigasi keluar saat ujian masih berjalan", async () => {
    getQuizMock.mockResolvedValue(quizResponse());
    const { router } = renderQuiz();

    const input = await screen.findByPlaceholderText(
      "Tulis jawaban singkat di sini"
    );
    fireEvent.change(input, { target: { value: "Jakarta" } });

    // Siswa menekan "Beranda" di navigasi aplikasi.
    await router.navigate("/");

    expect(await screen.findByText("Ujian masih berjalan")).toBeDefined();
    expect(screen.queryByText("HALAMAN BERANDA")).toBeNull();

    // Memilih "Keluar" baru benar-benar meninggalkan ujian.
    fireEvent.click(screen.getByText("Keluar"));
    expect(await screen.findByText("HALAMAN BERANDA")).toBeDefined();
  });

  it("tidak menahan navigasi kalau belum ada jawaban", async () => {
    getQuizMock.mockResolvedValue(quizResponse());
    const { router } = renderQuiz();
    await screen.findByText("Ibu kota Indonesia?");

    await router.navigate("/");
    expect(await screen.findByText("HALAMAN BERANDA")).toBeDefined();
  });
});
