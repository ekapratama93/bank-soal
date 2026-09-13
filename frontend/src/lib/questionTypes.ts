import type { QuestionType } from "@/api/client";

export const QUESTION_TYPE_LABELS: Record<QuestionType, string> = {
  pilihan_ganda: "Pilihan ganda",
  benar_salah: "Benar / Salah",
  isian: "Isian singkat",
  deskripsi: "Uraian / Deskripsi",
};

export const QUESTION_TYPE_OPTIONS: {
  key: QuestionType;
  label: string;
  short: string;
}[] = [
  { key: "pilihan_ganda", label: "Pilihan Ganda", short: "PG" },
  { key: "benar_salah", label: "Benar / Salah", short: "B/S" },
  { key: "isian", label: "Isian Singkat", short: "Isian" },
  { key: "deskripsi", label: "Uraian / Deskripsi", short: "Uraian" },
];