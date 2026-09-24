import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  ApiError,
  bulkDeleteQuizPackages,
  generateBatch,
  getAttempts,
  getQuiz,
  getSubjects,
  renameSubject,
  submitQuiz,
  updateExamType,
} from "./client";
import { getClockOffset, resetServerTime } from "../lib/serverTime";

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

beforeEach(() => {
  resetServerTime();
});

describe("pembungkus fetch", () => {
  it("mengembalikan body JSON saat sukses", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse([{ quiz_id: "q1" }])));
    await expect(getAttempts()).resolves.toEqual([{ quiz_id: "q1" }]);
  });

  it("memetakan kegagalan jaringan ke status 0", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("failed")));
    const err = await getQuiz("q1").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(0);
    expect(err.message).toBe("Tidak ada koneksi ke server.");
  });

  it("meneruskan status dan pesan galat HTTP", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse({ detail: "Kuis tidak ditemukan" }, { status: 404 })
      )
    );
    const err = await getQuiz("q1").catch((e) => e);
    expect(err.status).toBe(404);
    expect(err.message).toBe("Kuis tidak ditemukan");
  });

  it("menolak respons 200 yang bukan JSON (mis. SPA fallback saat proxy /api hilang)", async () => {
    // Sebelum diperketat, body non-JSON diam-diam dianggap null dan meledak
    // di halaman ("can't access property length, S is null").
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response("<!doctype html><html></html>", {
          status: 200,
          headers: { "Content-Type": "text/html" },
        })
      )
    );
    const err = await getSubjects().catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe("Server mengirim respons yang tidak valid.");
  });

  it("membatalkan permintaan yang menggantung dan menandainya bisa dicoba ulang", async () => {
    vi.useFakeTimers();
    // fetch yang tidak pernah selesai sampai sinyal abort dipicu.
    vi.stubGlobal(
      "fetch",
      vi.fn(
        (_url: string, init: RequestInit) =>
          new Promise((_resolve, reject) => {
            init.signal?.addEventListener("abort", () =>
              reject(new DOMException("aborted", "AbortError"))
            );
          })
      )
    );

    const pending = getQuiz("q1").catch((e) => e);
    await vi.advanceTimersByTimeAsync(21_000);
    const err = await pending;

    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(0);
    expect(err.message).toBe("Server terlalu lama merespons.");
  });

  it("mencatat jam server dari header Date", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-01-01T00:00:00Z"));
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse([], {
          headers: { Date: "Thu, 01 Jan 2026 01:00:00 GMT" },
        })
      )
    );

    await getAttempts();
    // Perangkat tertinggal ~1 jam dari server.
    expect(getClockOffset()).toBeGreaterThan(3_599_000);
  });
});

describe("endpoint yang memanggil AI di server memakai tenggang lebih panjang", () => {
  function hangingFetch() {
    return vi.fn(
      (_url: string, init: RequestInit) =>
        new Promise((_resolve, reject) => {
          init.signal?.addEventListener("abort", () =>
            reject(new DOMException("aborted", "AbortError"))
          );
        })
    );
  }

  it("submitQuiz tidak terpotong oleh tenggang default (20s)", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", hangingFetch());

    let settled = false;
    const pending = submitQuiz("q1", {}).catch((e) => e).finally(() => {
      settled = true;
    });
    await vi.advanceTimersByTimeAsync(21_000);
    // Lewat tenggang default (20s) tapi belum dibatalkan — submit pakai
    // tenggang yang jauh lebih panjang.
    expect(settled).toBe(false);

    await vi.advanceTimersByTimeAsync(600_000);
    const err = await pending;
    expect(settled).toBe(true);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe("Server terlalu lama merespons.");
  });

  it("generateBatch tidak terpotong oleh tenggang default (20s)", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", hangingFetch());

    let settled = false;
    const pending = generateBatch("tok", {
      subject_id: "sub-4",
      grade: 5,
      exam_type_id: "et-1",
      jumlah_paket: 3,
    })
      .catch((e) => e)
      .finally(() => {
        settled = true;
      });
    await vi.advanceTimersByTimeAsync(21_000);
    expect(settled).toBe(false);

    await vi.advanceTimersByTimeAsync(600_000);
    const err = await pending;
    expect(settled).toBe(true);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe("Server terlalu lama merespons.");
  });
});

describe("endpoint admin baru", () => {
  it("renameSubject memanggil PATCH /api/subjects/{id} dengan nama baru", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({ id: "sub-4", name: "Ilmu Pengetahuan Alam" })
    );
    vi.stubGlobal("fetch", fetchMock);
    await expect(
      renameSubject("tok", "sub-4", "Ilmu Pengetahuan Alam")
    ).resolves.toEqual({ id: "sub-4", name: "Ilmu Pengetahuan Alam" });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/subjects/sub-4");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body)).toEqual({ name: "Ilmu Pengetahuan Alam" });
    expect(init.headers.Authorization).toBe("Bearer tok");
  });

  it("updateExamType memanggil PATCH /api/exam-types/{id} dengan payload penuh", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({ id: "et-1", name: "Ujian Harian" })
    );
    vi.stubGlobal("fetch", fetchMock);
    await updateExamType("tok", "et-1", {
      name: "Ujian Harian",
      jumlah_soal: null,
      durasi_menit: 30,
      tipe_soal: null,
      poin_per_tipe: { deskripsi: 10 },
    });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/exam-types/et-1");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body)).toEqual({
      name: "Ujian Harian",
      jumlah_soal: null,
      durasi_menit: 30,
      tipe_soal: null,
      poin_per_tipe: { deskripsi: 10 },
    });
  });

  it("bulkDeleteQuizPackages memanggil bulk-delete dengan daftar id", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ deleted: 2 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(
      bulkDeleteQuizPackages("tok", ["q-1", "q-2"])
    ).resolves.toEqual({ deleted: 2 });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/quiz/admin/quizzes/bulk-delete");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ ids: ["q-1", "q-2"] });
  });
});
