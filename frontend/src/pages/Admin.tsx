import { useEffect, useState, type FormEvent } from "react";
import {
  ApiError,
  createExamType,
  createMaterial,
  createSubject,
  deleteExamType,
  deleteMaterial,
  deleteQuizPackage,
  deleteSubject,
  getExamTypes,
  getMaterials,
  getQuizPackageDetail,
  getQuizPackages,
  getSubjects,
  loginAdmin,
  generateBatch,
  resetPool,
  updateMaterial,
  updateQuizPackage,
  uploadMaterial,
  type AdminQuestion,
  type AdminQuestionInput,
  type ExamType,
  type Material,
  type QuestionType,
  type QuizPackage,
  type QuizPackageDetail as QuizPackageDetailData,
  type Subject,
  type TipeSoalConfig,
} from "../api/client";
import { GRADES } from "../subjects";
import { QUESTION_TYPE_OPTIONS } from "@/lib/questionTypes";
import { formatDate } from "../storage/results";
import { applySeo } from "../lib/seo";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import {
  AlertTriangle,
  BookOpen,
  CheckCircle2,
  Clock,
  Coins,
  Eye,
  FileText,
  Hash,
  Layers,
  ListChecks,
  LogOut,
  Package,
  Pencil,
  Plus,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  Trash2,
  type LucideIcon,
} from "lucide-react";
import QuizPackageDetail from "@/components/QuizPackageDetail";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

const OPTION_LETTERS = ["A", "B", "C", "D"];

interface EditableQuestion {
  tipe: QuestionType;
  pertanyaan: string;
  opsi: string[];
  jawabanPg: number;
  jawabanBs: "benar" | "salah";
  jawabanTeks: string;
  pembahasan: string;
  gambar?: string;
}

function toEditableQuestion(q: AdminQuestion): EditableQuestion {
  return {
    tipe: q.tipe,
    pertanyaan: q.pertanyaan,
    opsi: q.opsi ? [...q.opsi] : ["", "", "", ""],
    jawabanPg:
      q.tipe === "pilihan_ganda" && typeof q.jawaban === "number" ? q.jawaban : 0,
    jawabanBs: q.jawaban === "salah" ? "salah" : "benar",
    jawabanTeks:
      q.tipe === "isian" || q.tipe === "deskripsi" ? String(q.jawaban) : "",
    pembahasan: q.pembahasan,
    ...(q.gambar ? { gambar: q.gambar } : {}),
  };
}

function StatCard({
  icon: Icon,
  label,
  value,
}: {
  icon: LucideIcon;
  label: string;
  value: number;
}) {
  return (
    <div className="border-border/60 bg-card flex items-center gap-3 rounded-xl border px-4 py-3.5 shadow-sm">
      <span className="bg-secondary text-primary flex size-10 shrink-0 items-center justify-center rounded-full">
        <Icon className="size-5" />
      </span>
      <div>
        <div className="text-xl leading-none font-extrabold">{value}</div>
        <div className="text-muted-foreground mt-1 text-xs font-semibold">{label}</div>
      </div>
    </div>
  );
}

function EmptyState({ icon: Icon, text }: { icon: LucideIcon; text: string }) {
  return (
    <div className="border-border flex flex-col items-center gap-2.5 rounded-xl border border-dashed py-10 text-center">
      <span className="bg-muted text-muted-foreground flex size-11 items-center justify-center rounded-full">
        <Icon className="size-5" />
      </span>
      <p className="text-muted-foreground text-sm">{text}</p>
    </div>
  );
}

const TOKEN_KEY = "bank-soal-admin-token";

function loadToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export default function Admin() {
  const [token, setToken] = useState<string | null>(() => loadToken());
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loginError, setLoginError] = useState<string | null>(null);

  useEffect(() => {
    applySeo({ title: "Panel Admin", noindex: true });
  }, []);

  const [examTypes, setExamTypes] = useState<ExamType[]>([]);
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [materials, setMaterials] = useState<Material[]>([]);
  const [listError, setListError] = useState<string | null>(null);

  const [subject, setSubject] = useState<string>("");
  const [grade, setGrade] = useState<number>(1);
  const [examTypeId, setExamTypeId] = useState<string>("");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [formSuccess, setFormSuccess] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [etName, setEtName] = useState("");
  const [etJumlahSoal, setEtJumlahSoal] = useState<string>("");
  const [etDurasi, setEtDurasi] = useState<string>("");
  const [etTipeSoal, setEtTipeSoal] = useState<Record<string, string>>({
    pilihan_ganda: "",
    benar_salah: "",
    isian: "",
    deskripsi: "",
  });
  const [etPoinPerTipe, setEtPoinPerTipe] = useState<Record<string, string>>({
    pilihan_ganda: "",
    benar_salah: "",
    isian: "",
    deskripsi: "",
  });
  const [etError, setEtError] = useState<string | null>(null);
  const [etSuccess, setEtSuccess] = useState<string | null>(null);

  const [resetSubject, setResetSubject] = useState<string>("");
  const [resetGrade, setResetGrade] = useState<number>(1);
  const [resetExamTypeId, setResetExamTypeId] = useState<string>("");
  const [resetMessage, setResetMessage] = useState<string | null>(null);
  const [resetError, setResetError] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);
  const [genJumlahPaket, setGenJumlahPaket] = useState<number>(3);

  const [subName, setSubName] = useState("");
  const [subError, setSubError] = useState<string | null>(null);
  const [subSuccess, setSubSuccess] = useState<string | null>(null);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editContent, setEditContent] = useState("");
  const [editError, setEditError] = useState<string | null>(null);

  const [quizzes, setQuizzes] = useState<QuizPackage[]>([]);
  const [qmSubject, setQmSubject] = useState<string>("all");
  const [qmGrade, setQmGrade] = useState<string>("all");
  const [qmExamTypeId, setQmExamTypeId] = useState<string>("all");
  const [qmLoading, setQmLoading] = useState(false);
  const [qmError, setQmError] = useState<string | null>(null);
  const [qmMessage, setQmMessage] = useState<string | null>(null);
  const [detailId, setDetailId] = useState<string | null>(null);
  const [detailData, setDetailData] = useState<QuizPackageDetailData | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [editingQuiz, setEditingQuiz] = useState(false);
  const [editQuestions, setEditQuestions] = useState<EditableQuestion[]>([]);
  const [savingQuiz, setSavingQuiz] = useState(false);
  const [editQuizError, setEditQuizError] = useState<string | null>(null);

  function quizFilters() {
    return {
      subject: qmSubject !== "all" ? qmSubject : undefined,
      grade: qmGrade !== "all" ? Number(qmGrade) : undefined,
      exam_type_id: qmExamTypeId !== "all" ? qmExamTypeId : undefined,
    };
  }

  function loadQuizzes(tok: string) {
    setQmLoading(true);
    getQuizPackages(tok, quizFilters())
      .then((rows) => {
        setQuizzes(rows);
        setQmError(null);
      })
      .catch((e) => {
        if (e instanceof ApiError && (e.message.includes("login") || e.message.includes("admin"))) {
          localStorage.removeItem(TOKEN_KEY);
          setToken(null);
        }
        setQmError(e instanceof ApiError ? e.message : "Gagal memuat paket soal.");
      })
      .finally(() => setQmLoading(false));
  }

  useEffect(() => {
    if (token) loadQuizzes(token);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, qmSubject, qmGrade, qmExamTypeId]);

  async function handleViewQuiz(id: string) {
    if (!token) return;
    if (detailId === id) {
      setDetailId(null);
      setDetailData(null);
      setDetailError(null);
      setEditingQuiz(false);
      setEditQuizError(null);
      return;
    }
    setDetailId(id);
    setDetailData(null);
    setDetailError(null);
    setEditingQuiz(false);
    setEditQuizError(null);
    setDetailLoading(true);
    try {
      setDetailData(await getQuizPackageDetail(token, id));
    } catch (e) {
      setDetailError(e instanceof ApiError ? e.message : "Gagal memuat detail paket.");
      setDetailId(null);
    } finally {
      setDetailLoading(false);
    }
  }

  async function handleDeleteQuiz(q: QuizPackage) {
    if (!token) return;
    const confirmed = window.confirm(
      q.started
        ? "Paket ini sudah pernah dibuka siswa. Menghapusnya juga menghapus riwayat pengerjaan terkait. Hapus paket ini?"
        : "Hapus paket soal ini?"
    );
    if (!confirmed) return;
    setQmError(null);
    setQmMessage(null);
    try {
      await deleteQuizPackage(token, q.id);
      setQuizzes((prev) => prev.filter((x) => x.id !== q.id));
      if (detailId === q.id) {
        setDetailId(null);
        setDetailData(null);
      }
      setQmMessage("Paket soal dihapus.");
    } catch (e) {
      setQmError(e instanceof ApiError ? e.message : "Gagal menghapus paket soal.");
    }
  }

  function patchEditQuestion(i: number, patch: Partial<EditableQuestion>) {
    setEditQuestions((prev) =>
      prev.map((q, idx) => (idx === i ? { ...q, ...patch } : q))
    );
  }

  function patchEditOpsi(i: number, oi: number, value: string) {
    setEditQuestions((prev) =>
      prev.map((q, idx) =>
        idx === i ? { ...q, opsi: q.opsi.map((o, j) => (j === oi ? value : o)) } : q
      )
    );
  }

  function changeEditTipe(i: number, tipe: QuestionType) {
    setEditQuestions((prev) =>
      prev.map((q, idx) => {
        if (idx !== i) return q;
        if (tipe === "pilihan_ganda") {
          const opsi = [...q.opsi];
          while (opsi.length < 4) opsi.push("");
          return { ...q, tipe, opsi, jawabanPg: q.jawabanPg % 4 };
        }
        return { ...q, tipe };
      })
    );
  }

  function addEditQuestion() {
    setEditQuestions((prev) => [
      ...prev,
      {
        tipe: "pilihan_ganda",
        pertanyaan: "",
        opsi: ["", "", "", ""],
        jawabanPg: 0,
        jawabanBs: "benar",
        jawabanTeks: "",
        pembahasan: "",
      },
    ]);
  }

  async function saveQuizEdit() {
    if (!token || !detailData) return;
    for (let i = 0; i < editQuestions.length; i++) {
      const q = editQuestions[i];
      const no = i + 1;
      if (!q.pertanyaan.trim() || !q.pembahasan.trim()) {
        setEditQuizError(`Soal ${no}: pertanyaan/pembahasan kosong`);
        return;
      }
      if (
        q.tipe === "pilihan_ganda" &&
        (q.opsi.length !== 4 || q.opsi.some((o) => !o.trim()))
      ) {
        setEditQuizError(`Soal ${no}: opsi harus 4 item dan tidak kosong`);
        return;
      }
      if ((q.tipe === "isian" || q.tipe === "deskripsi") && !q.jawabanTeks.trim()) {
        setEditQuizError(`Soal ${no}: kunci jawaban kosong`);
        return;
      }
    }
    if (detailData.started) {
      const ok = window.confirm(
        "Paket ini sudah pernah dibuka siswa. Soal yang diubah berlaku untuk pengerjaan berikutnya. Lanjutkan?"
      );
      if (!ok) return;
    }
    setSavingQuiz(true);
    setEditQuizError(null);
    try {
      const payload: AdminQuestionInput[] = editQuestions.map((q): AdminQuestionInput => {
        const shared = {
          tipe: q.tipe,
          pertanyaan: q.pertanyaan.trim(),
          pembahasan: q.pembahasan.trim(),
          ...(q.gambar ? { gambar: q.gambar } : {}),
        };
        if (q.tipe === "pilihan_ganda") {
          return { ...shared, opsi: q.opsi, jawaban: q.jawabanPg };
        }
        if (q.tipe === "benar_salah") {
          return { ...shared, jawaban: q.jawabanBs };
        }
        return { ...shared, jawaban: q.jawabanTeks.trim() };
      });
      const updated = await updateQuizPackage(token, detailData.id, payload);
      setDetailData(updated);
      setEditingQuiz(false);
      setQmError(null);
      setQmMessage("Paket soal berhasil diperbarui.");
      loadQuizzes(token);
    } catch (e) {
      setEditQuizError(e instanceof ApiError ? e.message : "Gagal menyimpan perubahan.");
    } finally {
      setSavingQuiz(false);
    }
  }

  function loadAll(tok: string) {
    getSubjects()
      .then((subs) => {
        setSubjects(subs);
        if (subs.length > 0) {
          setSubject((cur) => cur || subs[0].name);
          setResetSubject((cur) => cur || subs[0].name);
        }
      })
      .catch(() => setListError("Gagal memuat mata pelajaran."));
    getExamTypes()
      .then((types) => {
        setExamTypes(types);
        if (types.length > 0 && !examTypeId) setExamTypeId(types[0].id);
        if (types.length > 0 && !resetExamTypeId) setResetExamTypeId(types[0].id);
      })
      .catch(() => setListError("Gagal memuat tipe ujian."));
    getMaterials(tok)
      .then(setMaterials)
      .catch((e) => {
        if (e instanceof ApiError && (e.message.includes("login") || e.message.includes("admin"))) {
          localStorage.removeItem(TOKEN_KEY);
          setToken(null);
        }
        setListError(e instanceof ApiError ? e.message : "Gagal memuat materi.");
      });
  }

  useEffect(() => {
    if (token) loadAll(token);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  async function handleLogin(ev: FormEvent) {
    ev.preventDefault();
    setLoginError(null);
    try {
      const res = await loginAdmin(email, password);
      localStorage.setItem(TOKEN_KEY, res.access_token);
      setToken(res.access_token);
      setPassword("");
    } catch (e) {
      setLoginError(e instanceof ApiError ? e.message : "Gagal login. Coba lagi.");
    }
  }

  async function handleSave(ev: FormEvent) {
    ev.preventDefault();
    if (!token) return;
    if (!examTypeId) {
      setFormError("Buat tipe ujian dulu di bagian atas halaman.");
      return;
    }
    setSaving(true);
    setFormError(null);
    setFormSuccess(null);
    try {
      if (file) {
        const form = new FormData();
        form.append("file", file);
        form.append("subject", subject);
        form.append("grade", String(grade));
        form.append("exam_type_id", examTypeId);
        if (title.trim()) form.append("title", title.trim());
        // Isi teks boleh ditambahkan bersama file (digabung jadi satu materi)
        if (content.trim()) form.append("content", content);
        await uploadMaterial(token, form);
      } else {
        await createMaterial(token, {
          subject,
          grade,
          exam_type_id: examTypeId,
          title,
          content,
        });
      }
      setMaterials(await getMaterials(token));
      setTitle("");
      setContent("");
      setFile(null);
      setFormSuccess("Materi berhasil disimpan.");
    } catch (e) {
      setFormError(e instanceof ApiError ? e.message : "Gagal menyimpan materi.");
    } finally {
      setSaving(false);
    }
  }

  function startEditMaterial(m: Material) {
    setEditingId(m.id);
    setEditTitle(m.title);
    setEditContent(m.content);
    setEditError(null);
  }

  async function handleSaveEditMaterial(id: string) {
    if (!token) return;
    setEditError(null);
    try {
      const updated = await updateMaterial(token, id, {
        title: editTitle,
        content: editContent,
      });
      setMaterials((prev) => prev.map((m) => (m.id === id ? updated : m)));
      setEditingId(null);
    } catch (e) {
      setEditError(e instanceof ApiError ? e.message : "Gagal menyimpan perubahan.");
    }
  }

  async function handleDeleteMaterial(id: string) {
    if (!token) return;
    try {
      await deleteMaterial(token, id);
      setMaterials((prev) => prev.filter((m) => m.id !== id));
    } catch (e) {
      setListError(e instanceof ApiError ? e.message : "Gagal menghapus materi.");
    }
  }

  async function handleCreateSubject(ev: FormEvent) {
    ev.preventDefault();
    if (!token) return;
    setSubError(null);
    setSubSuccess(null);
    try {
      await createSubject(token, subName);
      setSubjects(await getSubjects());
      setSubName("");
      setSubSuccess("Mata pelajaran ditambahkan.");
    } catch (e) {
      setSubError(e instanceof ApiError ? e.message : "Gagal menyimpan mata pelajaran.");
    }
  }

  async function handleDeleteSubject(id: string) {
    if (!token) return;
    setSubError(null);
    try {
      await deleteSubject(token, id);
      const subs = await getSubjects();
      setSubjects(subs);
      setSubject((cur) => (subs.some((s) => s.name === cur) ? cur : subs[0]?.name ?? ""));
    } catch (e) {
      setSubError(e instanceof ApiError ? e.message : "Gagal menghapus mata pelajaran.");
    }
  }

  async function handleCreateExamType(ev: FormEvent) {
    ev.preventDefault();
    if (!token) return;
    setEtError(null);
    setEtSuccess(null);
    try {
      const adaTipe = QUESTION_TYPE_OPTIONS.some((o) => etTipeSoal[o.key] !== "");
      const tipeSoal: TipeSoalConfig | null = adaTipe
        ? Object.fromEntries(
            QUESTION_TYPE_OPTIONS.map((o) => [o.key, Number(etTipeSoal[o.key] || 0)])
          )
        : null;
      const poinEntries = QUESTION_TYPE_OPTIONS.filter(
        (o) => etPoinPerTipe[o.key] !== ""
      ).map((o) => [o.key, Number(etPoinPerTipe[o.key])]);
      const poinPerTipe: TipeSoalConfig | null = poinEntries.length
        ? Object.fromEntries(poinEntries)
        : null;
      await createExamType(token, {
        name: etName,
        jumlah_soal: etJumlahSoal ? Number(etJumlahSoal) : null,
        durasi_menit: etDurasi ? Number(etDurasi) : null,
        tipe_soal: tipeSoal,
        poin_per_tipe: poinPerTipe,
      });
      setExamTypes(await getExamTypes());
      setEtName("");
      setEtJumlahSoal("");
      setEtDurasi("");
      setEtTipeSoal({ pilihan_ganda: "", benar_salah: "", isian: "", deskripsi: "" });
      setEtPoinPerTipe({
        pilihan_ganda: "",
        benar_salah: "",
        isian: "",
        deskripsi: "",
      });
      setEtSuccess("Tipe ujian tersimpan.");
    } catch (e) {
      setEtError(e instanceof ApiError ? e.message : "Gagal menyimpan tipe ujian.");
    }
  }

  async function handleDeleteExamType(id: string) {
    if (!token) return;
    setEtError(null);
    try {
      await deleteExamType(token, id);
      setExamTypes(await getExamTypes());
    } catch (e) {
      setEtError(e instanceof ApiError ? e.message : "Gagal menghapus tipe ujian.");
    }
  }

  async function handleGenerateBatch() {
    if (!token) return;
    setResetError(null);
    setResetMessage(null);
    setGenerating(true);
    try {
      const res = await generateBatch(token, {
        subject: resetSubject,
        grade: resetGrade,
        exam_type_id: resetExamTypeId,
        jumlah_paket: genJumlahPaket,
      });
      setResetMessage(`${res.generated} paket soal berhasil dibuat.`);
    } catch (e) {
      setResetError(
        e instanceof ApiError ? e.message : "Gagal membuat paket soal."
      );
    } finally {
      setGenerating(false);
    }
  }

  async function handleResetPool() {
    if (!token) return;
    setResetError(null);
    setResetMessage(null);
    try {
      const res = await resetPool(token, {
        subject: resetSubject,
        grade: resetGrade,
        exam_type_id: resetExamTypeId,
      });
      setResetMessage(`${res.deleted} paket yang belum dimulai telah dihapus.`);
    } catch (e) {
      setResetError(e instanceof ApiError ? e.message : "Gagal reset pool.");
    }
  }

  if (!token) {
    return (
      <div className="relative flex min-h-[65vh] items-center justify-center overflow-hidden px-4 py-10">
        <div
          aria-hidden
          className="pointer-events-none absolute -top-32 -right-20 size-[380px] rounded-full opacity-70"
          style={{
            background:
              "radial-gradient(circle at 30% 30%, var(--accent), transparent 70%)",
          }}
        />
        <Card className="relative w-full max-w-sm">
          <CardHeader>
            <div className="bg-secondary text-primary mx-auto flex size-14 items-center justify-center rounded-2xl">
              <ShieldCheck className="size-7" />
            </div>
            <CardTitle className="text-center text-2xl">Login Admin</CardTitle>
            <CardDescription className="text-center">
              Khusus guru/admin pengelola materi dan soal.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleLogin} className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>
              {loginError && (
                <Alert variant="destructive">
                  <AlertTriangle />
                  <AlertDescription>{loginError}</AlertDescription>
                </Alert>
              )}
              <Button type="submit" size="lg" className="mt-1">
                Masuk
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="bg-secondary text-primary flex size-12 shrink-0 items-center justify-center rounded-2xl">
            <ShieldCheck className="size-6" />
          </div>
          <div>
            <h1 className="text-2xl font-extrabold">Panel Admin</h1>
            <p className="text-muted-foreground text-sm">
              Kelola mata pelajaran, tipe ujian, materi, dan pool soal.
            </p>
          </div>
        </div>
        <Button
          variant="outline"
          onClick={() => {
            localStorage.removeItem(TOKEN_KEY);
            setToken(null);
          }}
        >
          <LogOut />
          Keluar
        </Button>
      </div>

      <div className="grid grid-cols-3 gap-3">
        <StatCard icon={BookOpen} label="Mata Pelajaran" value={subjects.length} />
        <StatCard icon={ListChecks} label="Tipe Ujian" value={examTypes.length} />
        <StatCard icon={Layers} label="Materi" value={materials.length} />
      </div>

      <Tabs defaultValue="subjects">
        <TabsList className="w-full sm:w-fit">
          <TabsTrigger value="subjects">
            <BookOpen className="size-4" />
            Mata Pelajaran
          </TabsTrigger>
          <TabsTrigger value="exam-types">
            <ListChecks className="size-4" />
            Tipe Ujian
          </TabsTrigger>
          <TabsTrigger value="materials">
            <Layers className="size-4" />
            Materi
          </TabsTrigger>
          <TabsTrigger value="pool">
            <RefreshCw className="size-4" />
            Pool
          </TabsTrigger>
          <TabsTrigger value="kuis">
            <Package className="size-4" />
            Kuis
          </TabsTrigger>
        </TabsList>

        <TabsContent value="subjects" className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2 text-primary">
                <BookOpen className="size-6" />
                <CardTitle className="text-2xl">Mata Pelajaran</CardTitle>
              </div>
              <CardDescription>Daftar mapel yang dipilih siswa di Beranda.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <form onSubmit={handleCreateSubject} className="flex flex-col gap-4">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="sub-name">Nama Mata Pelajaran</Label>
                  <Input
                    id="sub-name"
                    type="text"
                    value={subName}
                    onChange={(e) => setSubName(e.target.value)}
                    placeholder="mis. Sejarah"
                    required
                  />
                </div>
                {subError && (
                  <Alert variant="destructive">
                    <AlertTriangle />
                    <AlertDescription>{subError}</AlertDescription>
                  </Alert>
                )}
                {subSuccess && (
                  <Alert variant="success">
                    <CheckCircle2 />
                    <AlertDescription>{subSuccess}</AlertDescription>
                  </Alert>
                )}
                <Button type="submit" className="w-fit">
                  <Sparkles />
                  Tambah Mata Pelajaran
                </Button>
              </form>
              <Separator />
              {subjects.length === 0 ? (
                <EmptyState icon={BookOpen} text="Belum ada mata pelajaran." />
              ) : (
                <div className="flex flex-col gap-2">
                  {subjects.map((s) => (
                    <div
                      key={s.id}
                      className="border-border/60 flex items-center justify-between gap-3 rounded-xl border px-4 py-3"
                    >
                      <div className="flex items-center gap-3">
                        <span className="bg-secondary text-primary flex size-9 shrink-0 items-center justify-center rounded-full">
                          <BookOpen className="size-4" />
                        </span>
                        <strong className="font-bold">{s.name}</strong>
                      </div>
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => void handleDeleteSubject(s.id)}
                      >
                        <Trash2 />
                        Hapus
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="exam-types" className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2 text-primary">
                <ListChecks className="size-6" />
                <CardTitle className="text-2xl">Tipe Ujian</CardTitle>
              </div>
              <CardDescription>
                Tipe ujian dipilih siswa di Beranda. Jumlah soal dan durasi kosong berarti
                mengikuti konfigurasi kelas. Komposisi tipe soal mengatur jumlah soal per
                tipe (pilihan ganda, benar/salah, isian, uraian).
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <form onSubmit={handleCreateExamType} className="flex flex-col gap-4">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="et-name">Nama Tipe Ujian</Label>
                  <Input
                    id="et-name"
                    type="text"
                    value={etName}
                    onChange={(e) => setEtName(e.target.value)}
                    placeholder="mis. Ujian Harian"
                    required
                  />
                </div>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="et-jumlah">Jumlah Soal (opsional)</Label>
                    <Input
                      id="et-jumlah"
                      type="number"
                      min={5}
                      max={50}
                      value={etJumlahSoal}
                      onChange={(e) => setEtJumlahSoal(e.target.value)}
                    />
                  </div>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="et-durasi">Durasi Menit (opsional)</Label>
                    <Input
                      id="et-durasi"
                      type="number"
                      min={10}
                      max={180}
                      value={etDurasi}
                      onChange={(e) => setEtDurasi(e.target.value)}
                    />
                  </div>
                </div>
                <div className="border-border/60 flex flex-col gap-3 rounded-xl border border-dashed p-4">
                  <div className="flex flex-col gap-1">
                    <Label>Komposisi Tipe Soal (opsional)</Label>
                    <p className="text-muted-foreground text-xs">
                      Isi jumlah soal per tipe (mis. 10 pilihan ganda, 5 isian, 5 uraian).
                      Jika diisi, total soal = jumlah semua tipe dan kolom Jumlah Soal di
                      atas diabaikan. Kosongkan semua untuk komposisi otomatis (±40%
                      pilihan ganda, ±20% benar/salah, sisanya isian).
                    </p>
                  </div>
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                    {QUESTION_TYPE_OPTIONS.map((o) => (
                      <div key={o.key} className="flex flex-col gap-1.5">
                        <Label htmlFor={`et-${o.key}`} className="text-xs">
                          {o.label}
                        </Label>
                        <Input
                          id={`et-${o.key}`}
                          type="number"
                          min={0}
                          max={50}
                          value={etTipeSoal[o.key] ?? ""}
                          onChange={(e) =>
                            setEtTipeSoal({ ...etTipeSoal, [o.key]: e.target.value })
                          }
                        />
                      </div>
                    ))}
                  </div>
                </div>
                <div className="border-border/60 flex flex-col gap-3 rounded-xl border border-dashed p-4">
                  <div className="flex flex-col gap-1">
                    <Label>Poin per Tipe Soal (opsional)</Label>
                    <p className="text-muted-foreground text-xs">
                      Bobot nilai tiap tipe soal, mis. pilihan ganda 2, isian 5, uraian
                      10. Tipe yang tidak diisi bernilai 1 poin; total nilai siswa =
                      poin yang diperoleh dibagi total poin paket. Soal uraian/deskripsi
                      selalu dikoreksi &amp; dinilai AI (skor 0–1 × poin tipe).
                    </p>
                  </div>
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                    {QUESTION_TYPE_OPTIONS.map((o) => (
                      <div key={o.key} className="flex flex-col gap-1.5">
                        <Label htmlFor={`et-poin-${o.key}`} className="text-xs">
                          {o.label}
                        </Label>
                        <Input
                          id={`et-poin-${o.key}`}
                          type="number"
                          min={1}
                          max={100}
                          value={etPoinPerTipe[o.key] ?? ""}
                          onChange={(e) =>
                            setEtPoinPerTipe({
                              ...etPoinPerTipe,
                              [o.key]: e.target.value,
                            })
                          }
                        />
                      </div>
                    ))}
                  </div>
                </div>
                {etError && (
                  <Alert variant="destructive">
                    <AlertTriangle />
                    <AlertDescription>{etError}</AlertDescription>
                  </Alert>
                )}
                {etSuccess && (
                  <Alert variant="success">
                    <CheckCircle2 />
                    <AlertDescription>{etSuccess}</AlertDescription>
                  </Alert>
                )}
                <Button type="submit" className="w-fit">
                  <Sparkles />
                  Tambah Tipe Ujian
                </Button>
              </form>
              <Separator />
              {examTypes.length === 0 ? (
                <EmptyState icon={ListChecks} text="Belum ada tipe ujian." />
              ) : (
                <div className="flex flex-col gap-2">
                  {examTypes.map((t) => (
                    <div
                      key={t.id}
                      className="border-border/60 flex items-center justify-between gap-3 rounded-xl border px-4 py-3"
                    >
                      <div className="flex items-center gap-3">
                        <span className="bg-secondary text-primary flex size-9 shrink-0 items-center justify-center rounded-full">
                          <ListChecks className="size-4" />
                        </span>
                        <div>
                          <strong className="font-bold">{t.name}</strong>
                          <div className="mt-1.5 flex flex-wrap gap-1.5">
                            {t.tipe_soal && Object.keys(t.tipe_soal).length > 0 ? (
                              QUESTION_TYPE_OPTIONS.filter(
                                (o) => (t.tipe_soal?.[o.key] ?? 0) > 0
                              ).map((o) => (
                                <Badge key={o.key} variant="secondary">
                                  <Hash />
                                  {t.tipe_soal![o.key]} {o.short}
                                </Badge>
                              ))
                            ) : (
                              <Badge variant="secondary">
                                <Hash />
                                {t.jumlah_soal ? `${t.jumlah_soal} soal` : "ikut kelas"}
                              </Badge>
                            )}
                            {t.poin_per_tipe &&
                              Object.keys(t.poin_per_tipe).length > 0 &&
                              QUESTION_TYPE_OPTIONS.filter(
                                (o) => (t.poin_per_tipe?.[o.key] ?? 0) > 0
                              ).map((o) => (
                                <Badge key={`poin-${o.key}`} variant="outline">
                                  <Coins />
                                  {t.poin_per_tipe![o.key]} poin {o.short}
                                </Badge>
                              ))}
                            <Badge variant="secondary">
                              <Clock />
                              {t.durasi_menit ? `${t.durasi_menit} menit` : "ikut kelas"}
                            </Badge>
                          </div>
                        </div>
                      </div>
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => void handleDeleteExamType(t.id)}
                      >
                        <Trash2 />
                        Hapus
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="materials" className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2 text-primary">
                <Layers className="size-6" />
                <CardTitle className="text-2xl">Kelola Materi</CardTitle>
              </div>
              <CardDescription>
                Materi dipakai AI sebagai sumber soal untuk mapel + kelas + tipe ujian yang
                sesuai. Jika tidak ada materi, AI memakai kurikulum umum.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSave} className="flex flex-col gap-4">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="m-subject">Mata Pelajaran</Label>
                  <Select value={subject} onValueChange={setSubject}>
                    <SelectTrigger id="m-subject">
                      <SelectValue placeholder="Tidak ada mata pelajaran" />
                    </SelectTrigger>
                    <SelectContent>
                      {subjects.map((s) => (
                        <SelectItem key={s.id} value={s.name}>
                          {s.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="m-grade">Kelas</Label>
                  <Select value={String(grade)} onValueChange={(v) => setGrade(Number(v))}>
                    <SelectTrigger id="m-grade">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {GRADES.map((g) => (
                        <SelectItem key={g} value={String(g)}>
                          Kelas {g}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="m-exam-type">Tipe Ujian</Label>
                  <Select value={examTypeId} onValueChange={setExamTypeId}>
                    <SelectTrigger id="m-exam-type">
                      <SelectValue placeholder="Tidak ada tipe ujian tersedia" />
                    </SelectTrigger>
                    <SelectContent>
                      {examTypes.map((t) => (
                        <SelectItem key={t.id} value={t.id}>
                          {t.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="m-title">Judul Materi</Label>
                  <Input
                    id="m-title"
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="Opsional jika mengunggah file"
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="m-file">Unggah File (PDF/DOCX/TXT, maks 2 MB)</Label>
                  <Input
                    id="m-file"
                    type="file"
                    accept=".pdf,.docx,.txt"
                    onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="m-content">
                    Isi Materi (opsional jika mengunggah file — teks digabung dengan isi file)
                  </Label>
                  <Textarea
                    id="m-content"
                    dir="auto"
                    value={content}
                    onChange={(e) => setContent(e.target.value)}
                    required={file === null}
                  />
                </div>
                {formError && (
                  <Alert variant="destructive">
                    <AlertTriangle />
                    <AlertDescription>{formError}</AlertDescription>
                  </Alert>
                )}
                {formSuccess && (
                  <Alert variant="success">
                    <CheckCircle2 />
                    <AlertDescription>{formSuccess}</AlertDescription>
                  </Alert>
                )}
                <Button
                  type="submit"
                  className="w-fit"
                  disabled={saving || examTypes.length === 0 || subjects.length === 0}
                >
                  <Sparkles />
                  {saving ? "Menyimpan…" : "Simpan Materi"}
                </Button>
              </form>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <div className="flex items-center gap-2 text-primary">
                <FileText className="size-6" />
                <CardTitle className="text-2xl">Daftar Materi</CardTitle>
              </div>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              {listError && (
                <Alert variant="destructive">
                  <AlertTriangle />
                  <AlertDescription>{listError}</AlertDescription>
                </Alert>
              )}
              {materials.length === 0 ? (
                <EmptyState icon={FileText} text="Belum ada materi." />
              ) : (
                materials.map((m) =>
                  editingId === m.id ? (
                    <div
                      key={m.id}
                      className="border-border/60 flex flex-col gap-3 rounded-xl border px-4 py-3.5"
                    >
                      <div className="flex flex-col gap-2">
                        <Label htmlFor={`e-title-${m.id}`}>Judul</Label>
                        <Input
                          id={`e-title-${m.id}`}
                          type="text"
                          value={editTitle}
                          onChange={(e) => setEditTitle(e.target.value)}
                        />
                      </div>
                      <div className="flex flex-col gap-2">
                        <Label htmlFor={`e-content-${m.id}`}>Isi Materi</Label>
                        <Textarea
                          id={`e-content-${m.id}`}
                          dir="auto"
                          value={editContent}
                          onChange={(e) => setEditContent(e.target.value)}
                        />
                      </div>
                      {editError && (
                        <Alert variant="destructive">
                          <AlertTriangle />
                          <AlertDescription>{editError}</AlertDescription>
                        </Alert>
                      )}
                      <div className="flex gap-2">
                        <Button onClick={() => void handleSaveEditMaterial(m.id)}>Simpan</Button>
                        <Button variant="outline" onClick={() => setEditingId(null)}>
                          Batal
                        </Button>
                      </div>
                    </div>
                  ) : (
                    <div
                      key={m.id}
                      className="border-border/60 flex items-center justify-between gap-3 rounded-xl border px-4 py-3"
                    >
                      <div className="flex items-center gap-3">
                        <span className="bg-secondary text-primary flex size-9 shrink-0 items-center justify-center rounded-full">
                          <FileText className="size-4" />
                        </span>
                        <div>
                          <strong className="font-bold">{m.title}</strong>
                          {m.file_name && (
                            <span className="text-muted-foreground"> ({m.file_name})</span>
                          )}
                          <div className="mt-1.5 flex flex-wrap gap-1.5">
                            <Badge variant="secondary">{m.subject}</Badge>
                            <Badge variant="secondary">Kelas {m.grade}</Badge>
                            <Badge variant="secondary">{m.exam_types?.name ?? "-"}</Badge>
                          </div>
                        </div>
                      </div>
                      <div className="flex shrink-0 gap-2">
                        <Button variant="outline" size="sm" onClick={() => startEditMaterial(m)}>
                          <Pencil />
                          Edit
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => void handleDeleteMaterial(m.id)}
                        >
                          <Trash2 />
                          Hapus
                        </Button>
                      </div>
                    </div>
                  )
                )
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="pool" className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2 text-primary">
                <RefreshCw className="size-6" />
                <CardTitle className="text-2xl">Paket Soal (Pool)</CardTitle>
              </div>
              <CardDescription>
                Buat batch paket soal lebih dulu agar siswa bisa mengambil soal tanpa menunggu —
                AI membuat paket sebanyak jumlah yang dipilih di bawah, memakai SEMUA materi untuk
                kombinasi ini sebagai konteks. Reset Pool membuang paket yang belum dimulai (mis.
                setelah materi berubah); kuis yang sudah/sedang dikerjakan tidak terpengaruh.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="r-subject">Mata Pelajaran</Label>
                <Select value={resetSubject} onValueChange={setResetSubject}>
                  <SelectTrigger id="r-subject">
                    <SelectValue placeholder="Tidak ada mata pelajaran" />
                  </SelectTrigger>
                  <SelectContent>
                    {subjects.map((s) => (
                      <SelectItem key={s.id} value={s.name}>
                        {s.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="r-grade">Kelas</Label>
                <Select value={String(resetGrade)} onValueChange={(v) => setResetGrade(Number(v))}>
                  <SelectTrigger id="r-grade">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {GRADES.map((g) => (
                      <SelectItem key={g} value={String(g)}>
                        Kelas {g}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="r-exam-type">Tipe Ujian</Label>
                <Select value={resetExamTypeId} onValueChange={setResetExamTypeId}>
                  <SelectTrigger id="r-exam-type">
                    <SelectValue placeholder="Tidak ada tipe ujian tersedia" />
                  </SelectTrigger>
                  <SelectContent>
                    {examTypes.map((t) => (
                      <SelectItem key={t.id} value={t.id}>
                        {t.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="gen-jumlah-paket">Jumlah Paket Soal (1–5)</Label>
                <Input
                  id="gen-jumlah-paket"
                  type="number"
                  min={1}
                  max={5}
                  value={genJumlahPaket}
                  onChange={(e) => setGenJumlahPaket(Number(e.target.value))}
                />
              </div>
              {resetError && (
                <Alert variant="destructive">
                  <AlertTriangle />
                  <AlertDescription>{resetError}</AlertDescription>
                </Alert>
              )}
              {resetMessage && (
                <Alert variant="success">
                  <CheckCircle2 />
                  <AlertDescription>{resetMessage}</AlertDescription>
                </Alert>
              )}
              <div className="flex flex-wrap gap-2">
                <Button
                  onClick={() => void handleGenerateBatch()}
                  disabled={
                    generating ||
                    examTypes.length === 0 ||
                    subjects.length === 0 ||
                    !resetExamTypeId ||
                    !resetSubject
                  }
                >
                  <Sparkles />
                  {generating ? "Menghasilkan paket… (bisa beberapa saat)" : "Generate Paket Soal"}
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => void handleResetPool()}
                  disabled={
                    generating ||
                    examTypes.length === 0 ||
                    subjects.length === 0 ||
                    !resetExamTypeId ||
                    !resetSubject
                  }
                >
                  <Trash2 />
                  Reset Pool
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="kuis" className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2 text-primary">
                <Package className="size-6" />
                <CardTitle className="text-2xl">Kelola Kuis (Paket Soal)</CardTitle>
              </div>
              <CardDescription>
                Semua paket soal yang sudah di-generate, termasuk yang sudah dibuka
                siswa. Lihat isi soal beserta kunci jawaban &amp; pembahasan, atau hapus
                paket individual tanpa mengganggu paket lain.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="qm-subject">Mata Pelajaran</Label>
                  <Select value={qmSubject} onValueChange={setQmSubject}>
                    <SelectTrigger id="qm-subject">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">Semua</SelectItem>
                      {subjects.map((s) => (
                        <SelectItem key={s.id} value={s.name}>
                          {s.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="qm-grade">Kelas</Label>
                  <Select value={qmGrade} onValueChange={setQmGrade}>
                    <SelectTrigger id="qm-grade">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">Semua</SelectItem>
                      {GRADES.map((g) => (
                        <SelectItem key={g} value={String(g)}>
                          Kelas {g}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="qm-exam-type">Tipe Ujian</Label>
                  <Select value={qmExamTypeId} onValueChange={setQmExamTypeId}>
                    <SelectTrigger id="qm-exam-type">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">Semua</SelectItem>
                      {examTypes.map((t) => (
                        <SelectItem key={t.id} value={t.id}>
                          {t.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              {qmError && (
                <Alert variant="destructive">
                  <AlertTriangle />
                  <AlertDescription>{qmError}</AlertDescription>
                </Alert>
              )}
              {qmMessage && (
                <Alert variant="success">
                  <CheckCircle2 />
                  <AlertDescription>{qmMessage}</AlertDescription>
                </Alert>
              )}
              <div className="flex items-center justify-between gap-2">
                <p className="text-muted-foreground text-sm">
                  {quizzes.length} paket soal
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={qmLoading || !token}
                  onClick={() => token && loadQuizzes(token)}
                >
                  <RefreshCw />
                  Muat Ulang
                </Button>
              </div>
              {quizzes.length === 0 ? (
                <EmptyState
                  icon={Package}
                  text={
                    qmLoading
                      ? "Memuat paket soal…"
                      : "Belum ada paket soal. Generate dulu di tab Pool."
                  }
                />
              ) : (
                <div className="flex flex-col gap-2">
                  {quizzes.map((q) => (
                    <div key={q.id} className="flex flex-col gap-2">
                      <div className="border-border/60 flex flex-wrap items-center justify-between gap-3 rounded-xl border px-4 py-3">
                        <div className="flex min-w-0 items-center gap-3">
                          <span className="bg-secondary text-primary flex size-9 shrink-0 items-center justify-center rounded-full">
                            <Package className="size-4" />
                          </span>
                          <div className="min-w-0">
                            <strong className="font-bold">
                              {q.subject} · Kelas {q.grade}
                            </strong>
                            <div className="mt-1.5 flex flex-wrap gap-1.5">
                              <Badge variant="secondary">{q.exam_type || "-"}</Badge>
                              <Badge variant="secondary">
                                <Hash />
                                {q.jumlah_soal} soal
                              </Badge>
                              <Badge variant="secondary">
                                <Clock />
                                {q.durasi_menit ? `${q.durasi_menit} menit` : "ikut kelas"}
                              </Badge>
                              <Badge variant={q.started ? "default" : "outline"}>
                                {q.started ? "Sudah dibuka" : "Belum dimulai"}
                              </Badge>
                              {q.created_at && (
                                <Badge variant="outline">
                                  {formatDate(q.created_at)}
                                </Badge>
                              )}
                            </div>
                          </div>
                        </div>
                        <div className="flex shrink-0 gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => void handleViewQuiz(q.id)}
                            disabled={detailLoading && detailId !== q.id}
                          >
                            <Eye />
                            {detailId === q.id ? "Tutup" : "Lihat"}
                          </Button>
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => void handleDeleteQuiz(q)}
                          >
                            <Trash2 />
                            Hapus
                          </Button>
                        </div>
                      </div>
                      {detailId === q.id && (
                        <div className="flex flex-col gap-2 rounded-xl border border-dashed px-4 py-3.5">
                          {detailLoading && (
                            <p className="text-muted-foreground text-sm">Memuat detail…</p>
                          )}
                          {detailError && (
                            <Alert variant="destructive">
                              <AlertTriangle />
                              <AlertDescription>{detailError}</AlertDescription>
                            </Alert>
                          )}
                          {detailData && !editingQuiz && (
                            <>
                              <div className="flex flex-wrap items-center justify-between gap-2">
                                <p className="text-muted-foreground text-xs">
                                  Kunci jawaban &amp; pembahasan hanya terlihat di sini (admin)
                                  — tidak pernah dikirim ke siswa sebelum kuis dikumpulkan.
                                </p>
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={() => {
                                    setEditQuestions(
                                      detailData.questions.map(toEditableQuestion)
                                    );
                                    setEditQuizError(null);
                                    setEditingQuiz(true);
                                  }}
                                >
                                  <Pencil />
                                  Edit Soal
                                </Button>
                              </div>
                              <QuizPackageDetail questions={detailData.questions} />
                            </>
                          )}
                          {detailData && editingQuiz && (
                            <div className="flex flex-col gap-3">
                              <p className="text-muted-foreground text-xs">
                                Perbaiki soal hasil AI: ubah pertanyaan, opsi, kunci jawaban,
                                atau pembahasan; ubah tipe soal; hapus soal; atau tambah soal baru.
                                Perubahan berlaku untuk pengerjaan berikutnya.
                              </p>
                              {editQuizError && (
                                <Alert variant="destructive">
                                  <AlertTriangle />
                                  <AlertDescription>{editQuizError}</AlertDescription>
                                </Alert>
                              )}
                              {editQuestions.map((eq, i) => (
                                <div
                                  key={i}
                                  className="border-border/60 flex flex-col gap-2 rounded-xl border px-4 py-3.5"
                                >
                                  <div className="flex flex-wrap items-center justify-between gap-2">
                                    <div className="flex items-center gap-2">
                                      <Badge variant="secondary">Soal {i + 1}</Badge>
                                      <Select
                                        value={eq.tipe}
                                        onValueChange={(v) => changeEditTipe(i, v as QuestionType)}
                                      >
                                        <SelectTrigger className="h-8 w-fit text-xs">
                                          <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                          {QUESTION_TYPE_OPTIONS.map((o) => (
                                            <SelectItem key={o.key} value={o.key}>
                                              {o.label}
                                            </SelectItem>
                                          ))}
                                        </SelectContent>
                                      </Select>
                                    </div>
                                    <Button
                                      variant="outline"
                                      size="sm"
                                      className="text-destructive"
                                      onClick={() =>
                                        setEditQuestions((prev) => prev.filter((_, idx) => idx !== i))
                                      }
                                    >
                                      <Trash2 />
                                      Hapus Soal
                                    </Button>
                                  </div>
                                  {eq.gambar && (
                                    <p className="text-muted-foreground text-xs">
                                      Soal ini punya gambar — gambar tidak diubah dari sini.
                                    </p>
                                  )}
                                  <div className="flex flex-col gap-1.5">
                                    <Label
                                      htmlFor={`eq-pertanyaan-${detailData.id}-${i}`}
                                      className="text-xs"
                                    >
                                      Pertanyaan
                                    </Label>
                                    <Textarea
                                      id={`eq-pertanyaan-${detailData.id}-${i}`}
                                      dir="auto"
                                      value={eq.pertanyaan}
                                      onChange={(e) =>
                                        patchEditQuestion(i, { pertanyaan: e.target.value })
                                      }
                                    />
                                  </div>
                                  {eq.tipe === "pilihan_ganda" && (
                                    <div className="flex flex-col gap-1.5">
                                      <Label className="text-xs">
                                        Opsi — klik radio untuk menandai jawaban benar
                                      </Label>
                                      <RadioGroup
                                        value={String(eq.jawabanPg)}
                                        onValueChange={(v) =>
                                          patchEditQuestion(i, { jawabanPg: Number(v) })
                                        }
                                        className="gap-2"
                                      >
                                        {eq.opsi.map((opt, oi) => (
                                          <div key={oi} className="flex items-center gap-2.5">
                                            <RadioGroupItem value={String(oi)} />
                                            <Input
                                              value={opt}
                                              onChange={(e) => patchEditOpsi(i, oi, e.target.value)}
                                              placeholder={`Opsi ${OPTION_LETTERS[oi]}`}
                                              className="h-9"
                                            />
                                          </div>
                                        ))}
                                      </RadioGroup>
                                    </div>
                                  )}
                                  {eq.tipe === "benar_salah" && (
                                    <div className="flex flex-col gap-1.5">
                                      <Label className="text-xs">Kunci Jawaban</Label>
                                      <Select
                                        value={eq.jawabanBs}
                                        onValueChange={(v) =>
                                          patchEditQuestion(i, {
                                            jawabanBs: v as "benar" | "salah",
                                          })
                                        }
                                      >
                                        <SelectTrigger className="w-fit">
                                          <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                          <SelectItem value="benar">Benar</SelectItem>
                                          <SelectItem value="salah">Salah</SelectItem>
                                        </SelectContent>
                                      </Select>
                                    </div>
                                  )}
                                  {(eq.tipe === "isian" || eq.tipe === "deskripsi") && (
                                    <div className="flex flex-col gap-1.5">
                                      <Label
                                        htmlFor={`eq-jawaban-${detailData.id}-${i}`}
                                        className="text-xs"
                                      >
                                        Kunci Jawaban
                                      </Label>
                                      <Textarea
                                        id={`eq-jawaban-${detailData.id}-${i}`}
                                        dir="auto"
                                        value={eq.jawabanTeks}
                                        rows={eq.tipe === "deskripsi" ? 4 : 1}
                                        onChange={(e) =>
                                          patchEditQuestion(i, { jawabanTeks: e.target.value })
                                        }
                                      />
                                    </div>
                                  )}
                                  <div className="flex flex-col gap-1.5">
                                    <Label
                                      htmlFor={`eq-pembahasan-${detailData.id}-${i}`}
                                      className="text-xs"
                                    >
                                      Pembahasan
                                    </Label>
                                    <Textarea
                                      id={`eq-pembahasan-${detailData.id}-${i}`}
                                      dir="auto"
                                      value={eq.pembahasan}
                                      onChange={(e) =>
                                        patchEditQuestion(i, { pembahasan: e.target.value })
                                      }
                                    />
                                  </div>
                                </div>
                              ))}
                              <div className="flex flex-wrap items-center gap-2">
                                <Button variant="outline" size="sm" onClick={addEditQuestion}>
                                  <Plus />
                                  Tambah Soal
                                </Button>
                                <Button
                                  onClick={() => void saveQuizEdit()}
                                  disabled={savingQuiz || editQuestions.length === 0}
                                >
                                  <CheckCircle2 />
                                  {savingQuiz ? "Menyimpan…" : "Simpan Perubahan"}
                                </Button>
                                <Button
                                  variant="outline"
                                  onClick={() => {
                                    setEditingQuiz(false);
                                    setEditQuizError(null);
                                  }}
                                  disabled={savingQuiz}
                                >
                                  Batal
                                </Button>
                              </div>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}