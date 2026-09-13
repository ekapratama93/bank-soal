export type QuestionType = "pilihan_ganda" | "benar_salah" | "isian" | "deskripsi";

export type TipeSoalConfig = Partial<Record<QuestionType, number>>;

export interface QuestionPublic {
  nomor: number;
  tipe: QuestionType;
  pertanyaan: string;
  opsi?: string[];
  gambar?: string;
}

export interface Subject {
  id: string;
  name: string;
}

export interface ExamType {
  id: string;
  name: string;
  jumlah_soal: number | null;
  durasi_menit: number | null;
  tipe_soal: TipeSoalConfig | null;
  poin_per_tipe: TipeSoalConfig | null;
}

export interface QuizResponse {
  quiz_id: string;
  questions: QuestionPublic[];
  subject: string;
  grade: number;
  exam_type: string;
  durasi_menit: number;
  expires_at: string;
  repeat: boolean;
}

export type Verdict = "benar" | "parsial" | "salah";

export interface PerQuestionResult {
  nomor: number;
  tipe: QuestionType;
  pertanyaan: string;
  jawaban_siswa: string;
  jawaban_benar: string | number;
  verdict: Verdict;
  skor: number;
  poin?: number | null;
  poin_maks?: number | null;
  umpan_balik: string;
  pembahasan: string;
  gambar?: string;
  opsi?: string[] | null;
}

export interface SubmitResponse {
  quiz_id: string;
  subject: string;
  grade: number;
  exam_type: string;
  nilai: number;
  poin?: number | null;
  poin_maks?: number | null;
  per_question: PerQuestionResult[];
  expired: boolean;
}

export interface Material {
  id: string;
  subject: string;
  grade: number;
  exam_type_id: string;
  exam_types?: { name: string };
  title: string;
  content: string;
  file_name: string | null;
  created_by: string | null;
  created_at: string;
}

export interface MaterialInput {
  subject: string;
  grade: number;
  exam_type_id: string;
  title: string;
  content: string;
}

export interface AvailableCombo {
  subject: string;
  grade: number;
  exam_type_id: string;
  exam_type: string;
  unstarted: number;
  total: number;
}

export class ApiError extends Error {}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      ...(init?.headers ?? {}),
    },
  });
  if (res.status === 204) return undefined as T;
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    // respons tanpa isi
  }
  if (!res.ok) {
    const detail =
      body && typeof body === "object" && "detail" in body
        ? String((body as { detail: unknown }).detail)
        : "Terjadi kesalahan. Coba lagi nanti.";
    throw new ApiError(detail);
  }
  return body as T;
}

export function getExamTypes(): Promise<ExamType[]> {
  return request("/api/exam-types");
}

export function getAvailable(): Promise<AvailableCombo[]> {
  return request("/api/quiz/available");
}

export function getSubjects(): Promise<Subject[]> {
  return request("/api/subjects");
}

export function createSubject(token: string, name: string): Promise<Subject> {
  return request("/api/subjects", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
}

export function deleteSubject(token: string, id: string): Promise<void> {
  return request(`/api/subjects/${id}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
  });
}

export function requestQuiz(
  subject: string,
  grade: number,
  examTypeId: string,
  servedIds: string[]
): Promise<QuizResponse> {
  return request("/api/quiz/request", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      subject,
      grade,
      exam_type_id: examTypeId,
      served_ids: servedIds,
    }),
  });
}

export function generateBatch(
  token: string,
  data: { subject: string; grade: number; exam_type_id: string; jumlah_paket: number }
): Promise<{ generated: number; quiz_ids: string[] }> {
  return request("/api/quiz/generate", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export function getQuiz(quizId: string): Promise<QuizResponse> {
  return request(`/api/quiz/${quizId}`);
}

export function submitQuiz(
  quizId: string,
  answers: Record<string, unknown>
): Promise<SubmitResponse> {
  return request(`/api/quiz/${quizId}/submit`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ answers }),
  });
}

export function loginAdmin(email: string, password: string): Promise<{ access_token: string }> {
  return request("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
}

export function getMaterials(token: string): Promise<Material[]> {
  return request("/api/materials", {
    headers: { Authorization: `Bearer ${token}` },
  });
}

export function createMaterial(
  token: string,
  data: {
    subject: string;
    grade: number;
    exam_type_id: string;
    title: string;
    content: string;
  }
): Promise<Material> {
  return request("/api/materials", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export function uploadMaterial(token: string, form: FormData): Promise<Material> {
  return request("/api/materials/upload", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  });
}

export function updateMaterial(
  token: string,
  id: string,
  data: {
    title?: string;
    content?: string;
  }
): Promise<Material> {
  return request(`/api/materials/${id}`, {
    method: "PATCH",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export interface AttemptResult {
  quiz_id: string;
  subject: string;
  grade: number;
  exam_type: string;
  nilai: number;
  poin?: number | null;
  poin_maks?: number | null;
  per_question: PerQuestionResult[];
  expired: boolean;
  submitted_at: string;
}

export function getAttempts(): Promise<AttemptResult[]> {
  return request("/api/quiz/attempts");
}

export function resetPool(
  token: string,
  data: { subject: string; grade: number; exam_type_id: string }
): Promise<{ deleted: number }> {
  return request("/api/quiz/pool/reset", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export function deleteMaterial(token: string, id: string): Promise<void> {
  return request(`/api/materials/${id}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
  });
}

export function createExamType(
  token: string,
  data: {
    name: string;
    jumlah_soal: number | null;
    durasi_menit: number | null;
    tipe_soal: TipeSoalConfig | null;
    poin_per_tipe: TipeSoalConfig | null;
  }
): Promise<ExamType> {
  return request("/api/exam-types", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export function deleteExamType(token: string, id: string): Promise<void> {
  return request(`/api/exam-types/${id}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
  });
}