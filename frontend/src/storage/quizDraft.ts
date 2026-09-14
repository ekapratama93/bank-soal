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

function writeAll(drafts: Record<string, QuizDraft>): void {
  try {
    localStorage.setItem(DRAFTS_KEY, JSON.stringify(drafts));
  } catch {
    // Storage penuh/tak tersedia — draf bersifat best-effort.
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

export function saveQuizDraft(draft: QuizDraft): void {
  const drafts = readAll();
  drafts[draft.quizId] = draft;
  writeAll(prune(drafts));
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