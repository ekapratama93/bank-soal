import { noteServerDateHeader } from "../lib/serverTime";

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
  subject_id?: string | null;
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
  subject_id?: string | null;
  subject: string;
  grade: number;
  exam_type: string;
  nilai: number;
  poin?: number | null;
  poin_maks?: number | null;
  per_question: PerQuestionResult[];
  expired: boolean;
}

export interface MaterialImage {
  url: string;
  name: string;
}

export interface Material {
  id: string;
  subject_id?: string | null;
  subject: string;
  grade: number;
  exam_type_id: string;
  exam_types?: { name: string };
  title: string;
  content: string;
  file_name: string | null;
  file_url: string | null;
  images: MaterialImage[];
  created_by: string | null;
  created_at: string;
}

export interface MaterialInput {
  subject_id: string;
  grade: number;
  exam_type_id: string;
  title: string;
  content: string;
}

export interface AvailableCombo {
  subject_id?: string | null;
  subject: string;
  grade: number;
  exam_type_id: string;
  exam_type: string;
  unstarted: number;
  total: number;
}

export interface QuizPackage {
  id: string;
  subject_id?: string | null;
  subject: string;
  grade: number;
  exam_type_id: string;
  exam_type: string;
  jumlah_soal: number;
  started: boolean;
  durasi_menit: number | null;
  batch_id: string | null;
  created_at: string | null;
}

export interface AdminQuestion {
  nomor: number;
  tipe: QuestionType;
  pertanyaan: string;
  opsi?: string[];
  jawaban: number | string;
  pembahasan: string;
  gambar?: string;
}

export interface AdminQuestionInput {
  tipe: QuestionType;
  pertanyaan: string;
  opsi?: string[];
  jawaban: number | string;
  pembahasan: string;
  gambar?: string;
}

export interface QuizPackageDetail extends QuizPackage {
  questions: AdminQuestion[];
}

export interface QuizPackageFilters {
  subject_id?: string;
  grade?: number;
  exam_type_id?: string;
}

export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

/** Batas waktu default; permintaan yang menggantung lebih lama dianggap gagal. */
const DEFAULT_TIMEOUT_MS = 20_000;

/**
 * Endpoint yang memicu panggilan AI di server butuh tenggang jauh lebih
 * panjang (nginx sendiri memberi 300s) — dipakai submitQuiz (koreksi AI
 * jawaban isian) dan generateBatch (bisa memanggil AI berkali-kali, hingga
 * 5 paket sekaligus). Membatalkan terlalu cepat justru berbahaya: prosesnya
 * sudah jalan di server, lalu klien mengira gagal dan mencoba ulang.
 */
const LONG_TIMEOUT_MS = 120_000;

interface RequestOptions {
  timeoutMs?: number;
}

async function request<T>(
  path: string,
  init?: RequestInit,
  opts?: RequestOptions
): Promise<T> {
  let res: Response;
  const sentAt = Date.now();
  // AbortController manual (bukan AbortSignal.timeout) supaya jalan di jsdom.
  const controller = new AbortController();
  const timeoutMs = opts?.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, timeoutMs);
  try {
    res = await fetch(path, {
      ...init,
      signal: controller.signal,
      headers: {
        ...(init?.headers ?? {}),
      },
    });
  } catch {
    // Kegagalan jaringan (offline, DNS, dsb.) — bedakan dari respons HTTP.
    // Timeout diperlakukan sama: status 0 = kelas yang boleh dicoba ulang.
    throw new ApiError(
      timedOut
        ? "Server terlalu lama merespons."
        : "Tidak ada koneksi ke server.",
      0
    );
  } finally {
    clearTimeout(timer);
  }
  // Setiap respons membawa header Date — dipakai mengoreksi jam perangkat.
  noteServerDateHeader(res.headers.get("date"), sentAt, Date.now());
  if (res.status === 204) return undefined as T;
  let body: unknown = null;
  let parsed = false;
  try {
    body = await res.json();
    parsed = true;
  } catch {
    // Respons tanpa isi — atau BUKAN JSON (mis. SPA fallback index.html saat
    // proxy /api tidak jalan). Jangan diam-diam dianggap data valid.
  }
  if (!res.ok) {
    const detail =
      body && typeof body === "object" && "detail" in body
        ? String((body as { detail: unknown }).detail)
        : "Terjadi kesalahan. Coba lagi nanti.";
    throw new ApiError(detail, res.status);
  }
  if (!parsed) {
    // 200 dengan body bukan-JSON hampir pasti berarti permintaan tidak
    // sampai ke backend (mis. dijawab halaman statis). Lempar sebagai galat
    // supaya halaman menampilkan peringatan, bukan state null yang meledak.
    throw new ApiError("Server mengirim respons yang tidak valid.", res.status);
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

export function renameSubject(token: string, id: string, name: string): Promise<Subject> {
  return request(`/api/subjects/${id}`, {
    method: "PATCH",
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
  subjectId: string,
  grade: number,
  examTypeId: string,
  servedIds: string[]
): Promise<QuizResponse> {
  return request("/api/quiz/request", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      subject_id: subjectId,
      grade,
      exam_type_id: examTypeId,
      served_ids: servedIds,
    }),
  });
}

export function generateBatch(
  token: string,
  data: { subject_id: string; grade: number; exam_type_id: string; jumlah_paket: number }
): Promise<{ generated: number; quiz_ids: string[] }> {
  return request(
    "/api/quiz/generate",
    {
      method: "POST",
      headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
      body: JSON.stringify(data),
    },
    { timeoutMs: LONG_TIMEOUT_MS }
  );
}

export function getQuiz(quizId: string): Promise<QuizResponse> {
  return request(`/api/quiz/${quizId}`);
}

export function submitQuiz(
  quizId: string,
  answers: Record<string, unknown>
): Promise<SubmitResponse> {
  return request(
    `/api/quiz/${quizId}/submit`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ answers }),
    },
    { timeoutMs: LONG_TIMEOUT_MS }
  );
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

export function createMaterial(token: string, data: MaterialInput): Promise<Material> {
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
  subject_id?: string | null;
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
  data: { subject_id: string; grade: number; exam_type_id: string }
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

export interface ExamTypeInput {
  name: string;
  jumlah_soal: number | null;
  durasi_menit: number | null;
  tipe_soal: TipeSoalConfig | null;
  poin_per_tipe: TipeSoalConfig | null;
}

export function createExamType(token: string, data: ExamTypeInput): Promise<ExamType> {
  return request("/api/exam-types", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

/** Edit penuh tipe ujian — payload sama dengan create; field opsional yang
 * dikirim null dikosongkan kembali di backend. */
export function updateExamType(token: string, id: string, data: ExamTypeInput): Promise<ExamType> {
  return request(`/api/exam-types/${id}`, {
    method: "PATCH",
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

export function getQuizPackages(
  token: string,
  filters: QuizPackageFilters = {}
): Promise<QuizPackage[]> {
  const params = new URLSearchParams();
  if (filters.subject_id) params.set("subject_id", filters.subject_id);
  if (filters.grade !== undefined) params.set("grade", String(filters.grade));
  if (filters.exam_type_id) params.set("exam_type_id", filters.exam_type_id);
  const qs = params.toString();
  return request(`/api/quiz/admin/list${qs ? `?${qs}` : ""}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
}

export function getQuizPackageDetail(
  token: string,
  id: string
): Promise<QuizPackageDetail> {
  return request(`/api/quiz/admin/quizzes/${id}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
}

export function deleteQuizPackage(token: string, id: string): Promise<void> {
  return request(`/api/quiz/admin/quizzes/${id}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
  });
}

export function bulkDeleteQuizPackages(token: string, ids: string[]): Promise<{ deleted: number }> {
  return request("/api/quiz/admin/quizzes/bulk-delete", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ ids }),
  });
}

export function updateQuizPackage(
  token: string,
  id: string,
  questions: AdminQuestionInput[]
): Promise<QuizPackageDetail> {
  return request(`/api/quiz/admin/quizzes/${id}`, {
    method: "PATCH",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ questions }),
  });
}