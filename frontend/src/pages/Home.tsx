import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  BookOpen,
  Check,
  CheckCircle2,
  History,
  Layers,
  ListChecks,
  Sparkles,
} from "lucide-react";
import {
  ApiError,
  getAvailable,
  getExamTypes,
  getSubjects,
  requestQuiz,
  type AvailableCombo,
  type ExamType,
  type Subject,
} from "../api/client";
import { getServedIds } from "../storage/results";
import { getActiveDraft, type QuizDraft } from "../storage/quizDraft";
import { GRADES } from "../subjects";
import { applySeo } from "../lib/seo";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";

const STEPS = [
  {
    icon: ListChecks,
    label: "Langkah 1",
    title: "Pilih mapel, kelas & tipe ujian",
    desc: "Sesuaikan dengan apa yang sedang kamu pelajari di kelas.",
  },
  {
    icon: Sparkles,
    label: "Langkah 2",
    title: "Kerjakan paket soal",
    desc: "Soal pilihan ganda, benar/salah, isian, dan uraian, dengan timer yang berjalan otomatis.",
  },
  {
    icon: CheckCircle2,
    label: "Langkah 3",
    title: "Lihat nilai & pembahasan",
    desc: "Nilai keluar begitu waktu habis, lengkap dengan pembahasan tiap soal.",
  },
];

const FEATURES = [
  {
    icon: Layers,
    title: "Soal campuran",
    desc: "Pilihan ganda, benar/salah, isian, dan uraian digabung dalam satu paket, bukan cuma satu jenis soal.",
  },
  {
    icon: Sparkles,
    title: "Dibuatkan AI, dari materi guru",
    desc: "Paket soal disusun AI dari materi yang diunggah guru atau admin, bukan asal generate.",
  },
  {
    icon: CheckCircle2,
    title: "Nilai otomatis",
    desc: "Skor langsung keluar, termasuk koreksi AI untuk jawaban isian yang butuh penilaian.",
  },
  {
    icon: History,
    title: "Riwayat tersimpan",
    desc: "Setiap hasil latihan tersimpan di perangkatmu, bisa dibuka lagi kapan saja.",
  },
];

export default function Home() {
  const navigate = useNavigate();
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [examTypes, setExamTypes] = useState<ExamType[]>([]);
  // null = gagal memuat info ketersediaan; dropdown memakai daftar lengkap
  const [available, setAvailable] = useState<AvailableCombo[] | null>(null);
  const [subject, setSubject] = useState("");
  const [grade, setGrade] = useState<number | null>(null);
  const [examTypeId, setExamTypeId] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Draf ujian yang belum dikumpulkan (jawaban tersimpan di perangkat)
  const [resume, setResume] = useState<QuizDraft | null>(null);

  useEffect(() => {
    applySeo({
      title: "Bank Soal — Latihan Soal Online Kelas 1–12 dengan Nilai Otomatis",
      path: "/",
    });
  }, []);

  useEffect(() => {
    setResume(getActiveDraft());
  }, []);

  useEffect(() => {
    getSubjects()
      .then(setSubjects)
      .catch(() => setError("Gagal memuat mata pelajaran. Coba muat ulang halaman."));
    getExamTypes()
      .then(setExamTypes)
      .catch(() => setError("Gagal memuat tipe ujian. Coba muat ulang halaman."));
    getAvailable()
      .then(setAvailable)
      .catch(() => setAvailable(null));
  }, []);

  // Kombinasi yang benar-benar punya paket soal ter-generate
  const availableSubjects = useMemo(() => {
    if (available === null) return subjects;
    const names = new Set(available.map((a) => a.subject));
    return subjects.filter((s) => names.has(s.name));
  }, [subjects, available]);

  const availableGrades = useMemo(() => {
    if (available === null || !subject) return GRADES;
    const grades = new Set(
      available.filter((a) => a.subject === subject).map((a) => a.grade)
    );
    return GRADES.filter((g) => grades.has(g));
  }, [available, subject]);

  const availableExamTypes = useMemo(() => {
    if (available === null || !subject || grade === null) return examTypes;
    const ids = new Set(
      available
        .filter((a) => a.subject === subject && a.grade === grade)
        .map((a) => a.exam_type_id)
    );
    return examTypes.filter((t) => ids.has(t.id));
  }, [available, examTypes, subject, grade]);

  // Rapikan pilihan saat daftar tersedia berubah
  useEffect(() => {
    if (!availableSubjects.some((s) => s.name === subject) && availableSubjects.length > 0) {
      setSubject(availableSubjects[0].name);
    }
  }, [availableSubjects, subject]);

  useEffect(() => {
    if (!availableGrades.includes(grade as number) && availableGrades.length > 0) {
      setGrade(availableGrades[0]);
    }
  }, [availableGrades, grade]);

  useEffect(() => {
    if (!availableExamTypes.some((t) => t.id === examTypeId) && availableExamTypes.length > 0) {
      setExamTypeId(availableExamTypes[0].id);
    }
  }, [availableExamTypes, examTypeId]);

  async function handleGenerate() {
    if (!examTypeId || !subject || grade === null) {
      setError("Pilih mata pelajaran, kelas, dan tipe ujian terlebih dahulu.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const res = await requestQuiz(subject, grade, examTypeId, getServedIds());
      navigate(`/quiz/${res.quiz_id}`);
    } catch (e) {
      setError(
        e instanceof ApiError
          ? e.message
          : "Gagal mengambil paket soal. Coba lagi beberapa saat lagi."
      );
      setLoading(false);
    }
  }

  return (
    <div className="flex flex-col">
      {/* Hero */}
      <section className="relative overflow-hidden px-6 py-16 sm:px-10 sm:py-20 lg:px-16">
        <div
          aria-hidden
          className="pointer-events-none absolute -top-40 -right-32 size-[520px] rounded-full opacity-70"
          style={{
            background:
              "radial-gradient(circle at 30% 30%, var(--accent), transparent 70%)",
          }}
        />
        <div className="relative mx-auto grid max-w-6xl items-center gap-10 lg:grid-cols-[1fr_440px] lg:gap-14">
          <div>
            <span className="bg-secondary text-secondary-foreground inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-extrabold tracking-wide">
              <Sparkles className="size-3.5" />
              Dibantu AI &middot; Bahasa Indonesia
            </span>
            <h1 className="mt-5 text-4xl leading-tight font-extrabold tracking-tight text-balance sm:text-5xl">
              Latihan soal yang pas untuk kelas dan mapelmu
            </h1>
            <p className="text-muted-foreground mt-5 max-w-[42ch] text-lg leading-relaxed">
              Pilih mata pelajaran, kelas, dan tipe ujian — dapatkan paket
              soal campuran (pilihan ganda, benar/salah, isian, uraian) lengkap
              dengan timer dan nilai otomatis begitu selesai.
            </p>
            <div className="mt-8 flex flex-wrap items-center gap-3">
              <Button size="lg" asChild>
                <a href="#mulai">
                  <Sparkles className="size-4" />
                  Ambil Soal
                </a>
              </Button>
              <Button size="lg" variant="outline" asChild>
                <a href="#cara-kerja">Lihat cara kerja</a>
              </Button>
            </div>
            <p className="text-muted-foreground mt-4 text-sm">
              Langsung dari HP atau laptop, tanpa perlu login.
            </p>
          </div>

          {/* Ilustrasi */}
          <div className="relative mx-auto h-[380px] w-full max-w-[420px] sm:h-[420px]">
            <div className="border-border/60 bg-card absolute top-6 left-0 w-[230px] -rotate-6 rounded-xl border p-5 shadow-sm sm:w-[250px]">
              <div className="bg-muted h-2.5 w-3/4 rounded-full" />
              <div className="bg-muted mt-2 h-2.5 w-2/5 rounded-full" />
              <div className="mt-4 flex flex-col gap-2">
                <div className="flex items-center gap-2">
                  <span className="border-border size-3.5 shrink-0 rounded-full border" />
                  <span className="bg-muted h-2 w-4/5 rounded-full" />
                </div>
                <div className="flex items-center gap-2">
                  <span className="bg-primary flex size-3.5 shrink-0 items-center justify-center rounded-full">
                    <Check className="text-primary-foreground size-2.5" />
                  </span>
                  <span className="bg-secondary h-2 w-3/5 rounded-full" />
                </div>
                <div className="flex items-center gap-2">
                  <span className="border-border size-3.5 shrink-0 rounded-full border" />
                  <span className="bg-muted h-2 w-[70%] rounded-full" />
                </div>
              </div>
            </div>

            <div className="border-border/60 bg-card absolute right-0 bottom-0 w-[250px] rotate-3 rounded-xl border p-5 shadow-md sm:w-[270px]">
              <div className="flex flex-wrap gap-1.5">
                <span className="border-border rounded-full border px-2.5 py-1 text-[11px] font-bold">
                  Pilihan Ganda
                </span>
                <span className="border-border rounded-full border px-2.5 py-1 text-[11px] font-bold">
                  Isian
                </span>
              </div>
              <div className="mt-4 flex items-center gap-4">
                <svg width="76" height="76" viewBox="0 0 76 76" className="shrink-0">
                  <circle
                    cx="38"
                    cy="38"
                    r="32"
                    fill="none"
                    stroke="var(--muted)"
                    strokeWidth="7"
                  />
                  <circle
                    cx="38"
                    cy="38"
                    r="32"
                    fill="none"
                    stroke="var(--primary)"
                    strokeWidth="7"
                    strokeLinecap="round"
                    strokeDasharray="201.06"
                    strokeDashoffset="64.3"
                    transform="rotate(-90 38 38)"
                  />
                  <text
                    x="38"
                    y="34"
                    textAnchor="middle"
                    fontSize="13"
                    fontWeight="800"
                    style={{ fill: "var(--foreground)" }}
                  >
                    18:42
                  </text>
                  <text
                    x="38"
                    y="48"
                    textAnchor="middle"
                    fontSize="8"
                    fontWeight="700"
                    style={{ fill: "var(--muted-foreground)" }}
                  >
                    tersisa
                  </text>
                </svg>
                <div>
                  <div className="text-muted-foreground text-xs font-bold">
                    Nilai kamu
                  </div>
                  <div className="mt-0.5 flex items-baseline gap-1">
                    <span className="text-primary text-2xl font-extrabold">92</span>
                    <span className="text-muted-foreground text-sm font-bold">
                      / 100
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Cara kerja */}
      <section id="cara-kerja" className="scroll-mt-20 px-6 py-16 sm:px-10 lg:px-16">
        <div className="mx-auto max-w-5xl text-center">
          <p className="text-primary text-xs font-extrabold tracking-wider uppercase">
            Cara kerja
          </p>
          <h2 className="mt-2 text-3xl font-extrabold">
            Tiga langkah, langsung mulai
          </h2>
        </div>
        <div className="mx-auto mt-10 grid max-w-5xl gap-8 sm:grid-cols-3">
          {STEPS.map((step) => (
            <div key={step.title} className="text-center">
              <div className="bg-secondary text-primary mx-auto flex size-14 items-center justify-center rounded-xl">
                <step.icon className="size-6" />
              </div>
              <div className="text-primary mt-4 text-xs font-extrabold tracking-wider uppercase">
                {step.label}
              </div>
              <div className="mt-1 text-lg font-extrabold">{step.title}</div>
              <p className="text-muted-foreground mt-2 text-sm leading-relaxed">
                {step.desc}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* Fitur */}
      <section className="border-border/60 bg-card border-y px-6 py-16 sm:px-10 lg:px-16">
        <div className="mx-auto max-w-5xl text-center">
          <h2 className="text-3xl font-extrabold">Kenapa latihan di sini</h2>
        </div>
        <div className="mx-auto mt-10 grid max-w-5xl gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {FEATURES.map((feature) => (
            <div
              key={feature.title}
              className="border-border/60 rounded-xl border p-6 shadow-sm"
            >
              <feature.icon className="text-primary size-6" />
              <div className="mt-3.5 text-base font-extrabold">{feature.title}</div>
              <p className="text-muted-foreground mt-1.5 text-sm leading-relaxed">
                {feature.desc}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* Mulai / form */}
      <section id="mulai" className="scroll-mt-20 px-4 py-16 sm:px-6">
        <div className="mx-auto max-w-2xl text-center">
          <p className="text-primary text-xs font-extrabold tracking-wider uppercase">
            Mulai sekarang
          </p>
          <h2 className="mt-2 text-2xl font-extrabold sm:text-3xl">
            Pilih paketmu, mulai latihan
          </h2>
        </div>

        {resume && (
          <Card className="border-primary/40 mx-auto mt-8 max-w-2xl">
            <CardContent className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="text-sm font-extrabold">
                  Ada ujian yang belum dikumpulkan
                </div>
                <p className="text-muted-foreground text-sm">
                  {resume.subject} ·{" "}
                  {
                    Object.values(resume.answers).filter(
                      (v) => v !== "" && v !== undefined && v !== null
                    ).length
                  }{" "}
                  soal terjawab · sisa waktu ±
                  {Math.max(
                    0,
                    Math.ceil((resume.expiresAt - Date.now()) / 60000)
                  )}{" "}
                  menit. Jawabanmu tersimpan di perangkat ini.
                </p>
              </div>
              <Button asChild>
                <Link to={`/quiz/${resume.quizId}`}>Lanjutkan Ujian</Link>
              </Button>
            </CardContent>
          </Card>
        )}

        <Card className="mx-auto mt-8 max-w-2xl">
          <CardHeader>
            <div className="flex items-center gap-2 text-primary">
              <BookOpen className="size-6" />
              <CardTitle className="text-2xl">Latihan Soal</CardTitle>
            </div>
            <CardDescription>
              Pilih mata pelajaran, kelas, dan tipe ujian, lalu klik "Ambil
              Soal". Paket soal disiapkan oleh guru/admin; setiap klik
              memberi paket yang berbeda.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <Label htmlFor="subject">Mata Pelajaran</Label>
              <Select
                value={subject}
                onValueChange={(v) => {
                  setSubject(v);
                  setGrade(null);
                  setExamTypeId("");
                }}
                disabled={loading || availableSubjects.length === 0}
              >
                <SelectTrigger id="subject">
                  <SelectValue placeholder="Tidak ada paket soal tersedia" />
                </SelectTrigger>
                <SelectContent>
                  {availableSubjects.map((s) => (
                    <SelectItem key={s.id} value={s.name}>
                      {s.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="grade">Kelas</Label>
              <Select
                value={grade !== null ? String(grade) : ""}
                onValueChange={(v) => {
                  setGrade(Number(v));
                  setExamTypeId("");
                }}
                disabled={loading || availableGrades.length === 0}
              >
                <SelectTrigger id="grade">
                  <SelectValue placeholder="Tidak ada paket tersedia" />
                </SelectTrigger>
                <SelectContent>
                  {availableGrades.map((g) => (
                    <SelectItem key={g} value={String(g)}>
                      Kelas {g}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="exam-type">Tipe Ujian</Label>
              <Select
                value={examTypeId}
                onValueChange={setExamTypeId}
                disabled={loading || availableExamTypes.length === 0}
              >
                <SelectTrigger id="exam-type">
                  <SelectValue placeholder="Tidak ada paket tersedia" />
                </SelectTrigger>
                <SelectContent>
                  {availableExamTypes.map((t) => (
                    <SelectItem key={t.id} value={t.id}>
                      {t.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <Button
              size="lg"
              onClick={handleGenerate}
              disabled={
                loading ||
                availableSubjects.length === 0 ||
                availableExamTypes.length === 0 ||
                grade === null
              }
            >
              <Sparkles className="size-4" />
              {loading ? "Mengambil paket soal…" : "Ambil Soal"}
            </Button>
            {loading && (
              <p className="text-muted-foreground text-sm">
                Memuat paket soal, mohon tunggu.
              </p>
            )}
          </CardContent>
        </Card>
      </section>

      {/* Footer */}
      <footer className="border-border/60 border-t px-6 py-14 text-center">
        <p className="text-base">Guru atau admin sekolah?</p>
        <p className="text-muted-foreground mt-1.5 text-sm">
          Kelola mata pelajaran, tipe ujian, dan materi di Panel Admin.
        </p>
        <Button variant="outline" className="mt-4" asChild>
          <Link to="/admin">Buka Panel Admin</Link>
        </Button>
        <p className="text-muted-foreground mt-9 text-xs">
          Bank Soal &mdash; Latihan soal sekolah dengan bantuan AI
        </p>
      </footer>
    </div>
  );
}
