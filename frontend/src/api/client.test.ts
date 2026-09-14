import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, getAttempts, getQuiz } from "./client";
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
