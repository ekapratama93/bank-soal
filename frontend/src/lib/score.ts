export type ScoreLevel = "high" | "mid" | "low";

export function scoreLevel(nilai: number): ScoreLevel {
  if (nilai >= 80) return "high";
  if (nilai >= 60) return "mid";
  return "low";
}

export const SCORE_MESSAGE: Record<ScoreLevel, string> = {
  high: "Kerja bagus! Pertahankan ya 🎉",
  mid: "Sudah cukup baik, terus berlatih ya 💪",
  low: "Jangan menyerah, coba pelajari lagi ya 🌱",
};

export const SCORE_RING_CLASS: Record<ScoreLevel, string> = {
  high: "border-success text-success",
  mid: "border-warning text-warning",
  low: "border-destructive text-destructive",
};

export const SCORE_BADGE_VARIANT: Record<ScoreLevel, "success" | "warning" | "destructive"> = {
  high: "success",
  mid: "warning",
  low: "destructive",
};
