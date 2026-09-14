import {
  ApiError,
  getAttempts,
  submitQuiz,
  type AttemptResult,
  type SubmitResponse,
} from "../api/client";

/** Jeda antar percobaan otomatis: 1s, 2s, 4s, … maksimal 30s. */
export function backoffMs(attempt: number): number {
  return Math.min(30_000, 1000 * 2 ** Math.max(1, attempt));
}

export type SubmitOutcome =
  | { status: "success"; result: SubmitResponse }
  | { status: "already-submitted"; result: AttemptResult }
  | { status: "retry"; error: string; retryInMs: number }
  | { status: "fatal"; error: string };

/** Galat yang bisa pulih sendiri: jaringan/timeout (0) atau server (5xx). */
function isRetryable(e: unknown): boolean {
  return e instanceof ApiError && (e.status === 0 || e.status >= 500);
}

function messageOf(e: unknown, fallback: string): string {
  return e instanceof ApiError && e.message ? e.message : fallback;
}

/**
 * Kumpulkan jawaban, lalu rekonsiliasi sebelum memutuskan mencoba ulang.
 *
 * Kalau respons hilang di tengah jalan, pengumpulan biasanya SUDAH berhasil di
 * server. Mengirim ulang begitu saja berarti menilai ulang dan berpotensi
 * menimpa nilai yang sudah tersimpan, jadi setiap kegagalan dicek dulu ke
 * riwayat attempt perangkat ini (`GET /api/quiz/attempts`).
 */
export async function submitWithReconcile(
  quizId: string,
  answers: Record<string, number | string>,
  attempt: number
): Promise<SubmitOutcome> {
  try {
    return { status: "success", result: await submitQuiz(quizId, answers) };
  } catch (e) {
    const landed = await findSubmittedAttempt(quizId);
    if (landed) return { status: "already-submitted", result: landed };

    if (isRetryable(e)) {
      return {
        status: "retry",
        error: messageOf(
          e,
          "Koneksi bermasalah. Jawaban tetap tersimpan — pengumpulan dicoba ulang otomatis."
        ),
        retryInMs: backoffMs(attempt),
      };
    }
    return {
      status: "fatal",
      error: messageOf(e, "Gagal mengumpulkan jawaban. Coba lagi."),
    };
  }
}

/** Cari attempt paket ini yang sudah punya `submitted_at` di perangkat ini. */
export async function findSubmittedAttempt(
  quizId: string
): Promise<AttemptResult | null> {
  try {
    const rows = await getAttempts();
    return rows.find((r) => r.quiz_id === quizId && r.submitted_at) ?? null;
  } catch {
    // Rekonsiliasi sendiri gagal (masih offline) — anggap belum terkirim.
    return null;
  }
}
