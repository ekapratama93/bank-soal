// Draf jawaban kuis disimpan di sisi klien (localStorage) supaya tidak hilang
// saat halaman di-reload atau siswa tak sengaja menavigasi menjauh.
// Timer tetap otoritatif dari server (attempt.expires_at), jadi draf yang
// sudah kedaluwarsa otomatis dibuang.

const DRAFTS_KEY = "bank-soal-quiz-drafts";
const MAX_DRAFTS = 5;

export interface QuizDraft {
  quizId: string;
  subject: string;
  answers: Record<string, number | string>;
  flagged: Record<string, boolean>;
  current: number;
  phase: "exam" | "review";
  expiresAt: number;
  savedAt: number;
}

function readAll(): Record<string, QuizDraft> {
  try {
    const raw = localStorage.getItem(DRAFTS_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? (parsed as Record<string, QuizDraft>) : {};
  } catch {
    return {};
  }
}

/** Hasil penyimpanan draf — pemanggil perlu tahu kalau jawaban tidak tersimpan. */
export type DraftSaveResult = "ok" | "quota" | "unavailable";

function isQuotaError(e: unknown): boolean {
  return (
    e instanceof DOMException &&
    (e.name === "QuotaExceededError" ||
      e.name === "NS_ERROR_DOM_QUOTA_REACHED" ||
      e.code === 22)
  );
}

function writeAll(drafts: Record<string, QuizDraft>): DraftSaveResult {
  try {
    localStorage.setItem(DRAFTS_KEY, JSON.stringify(drafts));
    return "ok";
  } catch (e) {
    return isQuotaError(e) ? "quota" : "unavailable";
  }
}

function prune(drafts: Record<string, QuizDraft>): Record<string, QuizDraft> {
  // Draf kedaluwarsa tetap disimpan: jawaban yang belum terkumpul tetap bisa
  // dipulihkan lewat auto-submit saat kuis dibuka ulang. Hanya batasi jumlah.
  const kept = Object.values(drafts)
    .filter((d) => d && d.quizId)
    .sort((a, b) => b.savedAt - a.savedAt)
    .slice(0, MAX_DRAFTS);
  return Object.fromEntries(kept.map((d) => [d.quizId, d]));
}

export function loadQuizDraft(quizId: string): QuizDraft | null {
  // Tidak membuang draf kedaluwarsa di sini: jika siswa membuka ulang kuis
  // yang waktunya sudah lewat, jawaban sebelumnya tetap dipulihkan agar
  // auto-submit tetap mengirim pekerjaan terakhir. Pemanggil lain yang butuh
  // draf aktif memakai getActiveDraft().
  const draft = readAll()[quizId];
  return draft ?? null;
}

export function saveQuizDraft(draft: QuizDraft): DraftSaveResult {
  const drafts = readAll();
  drafts[draft.quizId] = draft;
  const result = writeAll(prune(drafts));
  if (result !== "quota") return result;
  // Penyimpanan penuh: korbankan draf lain demi ujian yang sedang dikerjakan.
  return writeAll({ [draft.quizId]: draft });
}

export function clearQuizDraft(quizId: string): void {
  const drafts = readAll();
  if (!(quizId in drafts)) return;
  delete drafts[quizId];
  writeAll(drafts);
}

/** Draf terbaru yang masih aktif dan punya minimal satu jawaban terisi. */
export function getActiveDraft(): QuizDraft | null {
  const now = Date.now();
  const active = Object.values(prune(readAll())).filter(
    (d) =>
      d.expiresAt > now &&
      Object.values(d.answers).some(
        (v) => v !== undefined && v !== null && v !== ""
      )
  );
  if (!active.length) return null;
  return active.sort((a, b) => b.savedAt - a.savedAt)[0];
}

/** Seberapa cocok draf tersimpan dengan attempt yang sedang dilayani server. */
export type DraftMatch = "exact" | "near" | "stale";

export interface ReconciledDraft {
  match: DraftMatch;
  answers: Record<string, number | string>;
  flagged: Record<string, boolean>;
  current: number;
  phase: "exam" | "review";
}

/** Toleransi beda `expires_at` yang masih dianggap attempt yang sama. */
const NEAR_MS = 60_000;

/**
 * Bandingkan draf tersimpan dengan `expires_at` dari server dan bersihkan isinya.
 *
 * Draf yang tidak cocok TIDAK dibuang diam-diam: dikembalikan dengan
 * `match: "stale"` supaya siswa sendiri yang memutuskan memulihkannya.
 */
export function reconcileDraft(
  draft: QuizDraft | null,
  serverExpiresAt: number,
  totalQuestions: number
): ReconciledDraft | null {
  if (!draft) return null;

  const answers: Record<string, number | string> = {};
  for (const [k, v] of Object.entries(draft.answers ?? {})) {
    const idx = Number(k);
    if (
      Number.isInteger(idx) &&
      idx >= 0 &&
      idx < totalQuestions &&
      v !== "" &&
      v !== undefined &&
      v !== null
    ) {
      answers[k] = v;
    }
  }

  const diff = Math.abs(draft.expiresAt - serverExpiresAt);
  const match: DraftMatch = diff === 0 ? "exact" : diff <= NEAR_MS ? "near" : "stale";

  return {
    match,
    answers,
    flagged: draft.flagged ?? {},
    current: Math.max(0, Math.min(draft.current ?? 0, Math.max(0, totalQuestions - 1))),
    phase: draft.phase === "review" ? "review" : "exam",
  };
}
