// Jam server sebagai acuan waktu ujian.
//
// Timer ujian membandingkan `expires_at` dari server dengan waktu perangkat.
// Di perangkat yang jamnya meleset (sering terjadi di tablet murah), selisih itu
// membuat hitung mundur salah — bahkan bisa langsung nol dan mengumpulkan
// lembar kosong begitu kuis dibuka. Header `Date` pada tiap respons HTTP dipakai
// untuk mengukur selisihnya, tanpa perlu endpoint tambahan.

/** Sampel dengan bolak-balik selambat ini tidak cukup akurat untuk dipakai. */
const MAX_RTT_MS = 5000;
/** Selisih di bawah ini diabaikan — bukan jam meleset, hanya jitter jaringan. */
const MIN_OFFSET_MS = 2000;

let offsetMs = 0;
let bestRttMs = Number.POSITIVE_INFINITY;

/**
 * Catat satu sampel jam server (algoritma Cristian).
 *
 * Header `Date` hanya presisi detik, jadi titik tengah detiknya diperkirakan
 * (+500ms) dan separuh waktu bolak-balik ditambahkan. Sampel dengan bolak-balik
 * tercepat yang dipakai, karena itu yang paling kecil ketidakpastiannya.
 */
export function noteServerDate(
  serverEpochMs: number,
  sentAt: number,
  receivedAt: number
): void {
  if (!Number.isFinite(serverEpochMs)) return;
  const rtt = receivedAt - sentAt;
  if (rtt < 0 || rtt > MAX_RTT_MS) return;
  if (rtt > bestRttMs) return;

  bestRttMs = rtt;
  const serverMid = serverEpochMs + 500 + rtt / 2;
  const raw = serverMid - receivedAt;
  offsetMs = Math.abs(raw) > MIN_OFFSET_MS ? raw : 0;
}

/** Catat dari header `Date` mentah sebuah respons. */
export function noteServerDateHeader(
  header: string | null | undefined,
  sentAt: number,
  receivedAt: number
): void {
  if (!header) return;
  noteServerDate(Date.parse(header), sentAt, receivedAt);
}

/** Waktu sekarang menurut jam server (perkiraan terbaik). */
export function serverNow(): number {
  return Date.now() + offsetMs;
}

/** Selisih jam perangkat terhadap server, dalam milidetik. */
export function getClockOffset(): number {
  return offsetMs;
}

/** Hanya untuk test. */
export function resetServerTime(): void {
  offsetMs = 0;
  bestRttMs = Number.POSITIVE_INFINITY;
}
