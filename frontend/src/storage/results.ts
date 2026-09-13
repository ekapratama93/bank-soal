// Riwayat hasil kini disimpan di server (tabel attempts) dan dikorelasikan
// per perangkat lewat cookie client_id — lihat api/client.ts getAttempts().
// Modul ini hanya menyimpan daftar ID kuis yang pernah disajikan ke browser.

const SERVED_KEY = "bank-soal-served";

export function getServedIds(): string[] {
  try {
    const raw = localStorage.getItem(SERVED_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as string[]) : [];
  } catch {
    return [];
  }
}

export function markServed(quizId: string): void {
  const ids = getServedIds().filter((id) => id !== quizId);
  ids.unshift(quizId);
  localStorage.setItem(SERVED_KEY, JSON.stringify(ids.slice(0, 200)));
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("id-ID", {
    day: "numeric",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}