import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  clearQuizDraft,
  getActiveDraft,
  loadQuizDraft,
  reconcileDraft,
  saveQuizDraft,
  type QuizDraft,
} from "./quizDraft";

const NOW = Date.now();

function makeDraft(overrides: Partial<QuizDraft> = {}): QuizDraft {
  return {
    quizId: "q1",
    subject: "Matematika",
    answers: { "0": 1 },
    flagged: { "1": true },
    current: 2,
    phase: "exam",
    expiresAt: NOW + 30 * 60 * 1000,
    savedAt: NOW,
    ...overrides,
  };
}

beforeEach(() => {
  localStorage.clear();
});

describe("draf jawaban kuis", () => {
  it("menyimpan dan memuat draf", () => {
    saveQuizDraft(makeDraft());
    const draft = loadQuizDraft("q1");
    expect(draft?.answers).toEqual({ "0": 1 });
    expect(draft?.flagged).toEqual({ "1": true });
    expect(draft?.current).toBe(2);
    expect(draft?.phase).toBe("exam");
  });

  it("mengembalikan null jika draf tidak ada", () => {
    expect(loadQuizDraft("tidak-ada")).toBeNull();
  });

  it("memuat draf kedaluwarsa agar auto-submit tetap mengirim jawaban", () => {
    saveQuizDraft(makeDraft({ expiresAt: NOW - 1000 }));
    expect(loadQuizDraft("q1")?.answers).toEqual({ "0": 1 });
  });

  it("menghapus draf setelah dikumpulkan", () => {
    saveQuizDraft(makeDraft());
    clearQuizDraft("q1");
    expect(loadQuizDraft("q1")).toBeNull();
  });

  it("getActiveDraft hanya mengembalikan draf dengan jawaban terisi", () => {
    saveQuizDraft(makeDraft({ answers: {} }));
    expect(getActiveDraft()).toBeNull();
    saveQuizDraft(makeDraft({ answers: { "2": "isian singkat" } }));
    expect(getActiveDraft()?.quizId).toBe("q1");
  });

  it("getActiveDraft mengabaikan jawaban kosong dan draf kedaluwarsa", () => {
    saveQuizDraft(makeDraft({ answers: { "0": "" } }));
    expect(getActiveDraft()).toBeNull();
    saveQuizDraft(makeDraft({ expiresAt: NOW - 1000, answers: { "0": 1 } }));
    expect(getActiveDraft()).toBeNull();
  });

  it("getActiveDraft memilih draf terbaru", () => {
    saveQuizDraft(makeDraft({ quizId: "q1", savedAt: 1000 }));
    saveQuizDraft(makeDraft({ quizId: "q2", savedAt: 2000 }));
    expect(getActiveDraft()?.quizId).toBe("q2");
  });

  it("membatasi jumlah draf yang disimpan", () => {
    for (let i = 0; i < 8; i++) {
      saveQuizDraft(makeDraft({ quizId: `q-${i}`, savedAt: 1000 + i }));
    }
    expect(loadQuizDraft("q-0")).toBeNull();
    expect(loadQuizDraft("q-2")).toBeNull();
    expect(loadQuizDraft("q-3")).not.toBeNull();
    expect(loadQuizDraft("q-7")).not.toBeNull();
  });
});
describe("penyimpanan penuh", () => {
  it("melaporkan ok saat penyimpanan normal", () => {
    expect(saveQuizDraft(makeDraft())).toBe("ok");
  });

  it("mengorbankan draf lain demi ujian yang sedang dikerjakan", () => {
    saveQuizDraft(makeDraft({ quizId: "lama", savedAt: NOW - 1000 }));
    const real = localStorage.setItem.bind(localStorage);
    let calls = 0;
    vi.spyOn(Storage.prototype, "setItem").mockImplementation((k, v) => {
      calls += 1;
      // Penulisan pertama (berisi semua draf) gagal, penulisan kedua lolos.
      if (calls === 1) {
        throw new DOMException("penuh", "QuotaExceededError");
      }
      real(k, v);
    });

    expect(saveQuizDraft(makeDraft({ quizId: "baru" }))).toBe("ok");
    vi.restoreAllMocks();
    expect(loadQuizDraft("baru")).not.toBeNull();
    expect(loadQuizDraft("lama")).toBeNull();
  });

  it("melaporkan quota kalau tetap gagal", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("penuh", "QuotaExceededError");
    });
    expect(saveQuizDraft(makeDraft())).toBe("quota");
  });
});

describe("mencocokkan draf dengan attempt server", () => {
  const draft = makeDraft({
    answers: { "0": 1, "1": "benar", "5": "di luar jangkauan", "x": 3, "2": "" },
    current: 99,
  });

  it("mengembalikan null kalau tidak ada draf", () => {
    expect(reconcileDraft(null, NOW, 3)).toBeNull();
  });

  it("membuang jawaban di luar jangkauan dan yang kosong", () => {
    const r = reconcileDraft(draft, draft.expiresAt, 3);
    expect(r?.answers).toEqual({ "0": 1, "1": "benar" });
  });

  it("membatasi nomor soal aktif", () => {
    expect(reconcileDraft(draft, draft.expiresAt, 3)?.current).toBe(2);
  });

  it("menandai cocok persis", () => {
    expect(reconcileDraft(draft, draft.expiresAt, 3)?.match).toBe("exact");
  });

  it("mentoleransi selisih waktu kecil", () => {
    expect(reconcileDraft(draft, draft.expiresAt + 30_000, 3)?.match).toBe("near");
  });

  it("menandai draf dari attempt lain sebagai stale, bukan membuangnya", () => {
    const r = reconcileDraft(draft, draft.expiresAt + 10 * 60_000, 3);
    expect(r?.match).toBe("stale");
    // Jawabannya tetap ada supaya bisa ditawarkan ke siswa.
    expect(r?.answers).toEqual({ "0": 1, "1": "benar" });
  });
});
