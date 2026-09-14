import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  Clock,
  Flag,
  LayoutGrid,
  Save,
} from "lucide-react";
import {
  ApiError,
  getQuiz,
  submitQuiz,
  type QuestionPublic,
  type SubmitResponse,
} from "../api/client";
import { markServed } from "../storage/results";
import {
  clearQuizDraft,
  loadQuizDraft,
  saveQuizDraft,
} from "../storage/quizDraft";
import { applySeo } from "../lib/seo";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Textarea } from "@/components/ui/textarea";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Separator } from "@/components/ui/separator";
import MathText from "@/components/MathText";
import QuestionImage from "@/components/QuestionImage";
import { NavigatorGrid, NavigatorLegend, type NavigatorItem } from "@/components/QuestionNavigator";
import { cn } from "@/lib/utils";
import { QUESTION_TYPE_LABELS } from "@/lib/questionTypes";

const NAV_LEGEND = [
  { dotClassName: "bg-primary", label: "Sudah dijawab" },
  { dotClassName: "border-input bg-card border", label: "Belum dijawab" },
  { dotClassName: "bg-warning", label: "Ditandai" },
];

function formatTime(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000);
  const m = Math.floor(totalSeconds / 60);
  const s = totalSeconds % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export default function Quiz() {
  const { quizId } = useParams<{ quizId: string }>();
  const navigate = useNavigate();

  const [questions, setQuestions] = useState<QuestionPublic[] | null>(null);
  const [subject, setSubject] = useState("");
  const [grade, setGrade] = useState(0);
  const [examType, setExamType] = useState("");
  const [expiresAt, setExpiresAt] = useState<number | null>(null);
  const [remaining, setRemaining] = useState(0);
  const [answers, setAnswers] = useState<Record<string, number | string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [offline, setOffline] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  const [current, setCurrent] = useState(0);
  const [flagged, setFlagged] = useState<Record<string, boolean>>({});
  const [phase, setPhase] = useState<"exam" | "review">("exam");
  const [navOpen, setNavOpen] = useState(false);
  const [restored, setRestored] = useState(false);

  const answersRef = useRef(answers);
  answersRef.current = answers;
  const submittedRef = useRef(false);
  const expiresAtRef = useRef<number | null>(null);
  const submitAttemptsRef = useRef(0);
  const submitRetryAtRef = useRef(0);

  async function doSubmit(manual = false) {
    if (submittedRef.current || !quizId) return;
    submittedRef.current = true;
    if (manual) {
      submitAttemptsRef.current = 0;
      submitRetryAtRef.current = 0;
    }
    setSubmitting(true);
    setError(null);
    try {
      const result: SubmitResponse = await submitQuiz(quizId, answersRef.current);
      clearQuizDraft(quizId);
      navigate(`/result/${result.quiz_id}`, { state: { result } });
    } catch (e) {
      submittedRef.current = false;
      // Backoff antar percobaan otomatis: 1s, 2s, 4s, … maks 30s.
      submitAttemptsRef.current += 1;
      submitRetryAtRef.current =
        Date.now() + Math.min(30000, 1000 * 2 ** submitAttemptsRef.current);
      setSubmitting(false);
      setError(
        e instanceof ApiError
          ? e.message
          : "Koneksi bermasalah. Jawaban tetap tersimpan — pengumpulan dicoba ulang otomatis."
      );
    }
  }

  useEffect(() => {
    applySeo({ title: "Latihan Soal", noindex: true });
  }, []);

  // Judul spesifik setelah paket dimuat (halaman tidak diindeks; ini untuk UX)
  useEffect(() => {
    if (!subject) return;
    applySeo({
      title: `Latihan Soal ${subject} Kelas ${grade}${examType ? ` — ${examType}` : ""}`,
      noindex: true,
    });
  }, [subject, grade, examType]);

  useEffect(() => {
    if (!quizId) return;
    let cancelled = false;
    let retryTimer: number | undefined;
    let attempts = 0;

    const load = () => {
      getQuiz(quizId)
        .then((quiz) => {
          if (cancelled) return;
          setOffline(false);
          setError(null);
          setQuestions(quiz.questions);
          setSubject(quiz.subject);
          setGrade(quiz.grade);
          setExamType(quiz.exam_type);
          markServed(quiz.quiz_id);
          const expiry = new Date(quiz.expires_at).getTime();
          setExpiresAt(expiry);
          expiresAtRef.current = expiry;
          // Pulihkan draf jawaban sebelumnya (mis. halaman sempat di-reload).
          // Cocokkan expires_at supaya draf lama dari attempt lain dibuang.
          const draft = loadQuizDraft(quizId);
          if (draft && draft.expiresAt === expiry) {
            const total = quiz.questions.length;
            const saved: Record<string, number | string> = {};
            for (const [k, v] of Object.entries(draft.answers ?? {})) {
              const idx = Number(k);
              if (
                Number.isInteger(idx) &&
                idx >= 0 &&
                idx < total &&
                v !== "" &&
                v !== undefined &&
                v !== null
              ) {
                saved[k] = v;
              }
            }
            setAnswers(saved);
            setFlagged(draft.flagged ?? {});
            setCurrent(Math.max(0, Math.min(draft.current, total - 1)));
            setPhase(draft.phase === "review" ? "review" : "exam");
            if (Object.keys(saved).length > 0) setRestored(true);
          }
        })
        .catch((e) => {
          if (cancelled) return;
          // Kegagalan jaringan (status 0) atau server (5xx) bisa pulih —
          // coba ulang otomatis. 4xx berarti kuis memang tidak berlaku.
          if (e instanceof ApiError && e.status !== 0 && e.status < 500) {
            setOffline(false);
            setError(
              e.message ||
                "Kuis tidak ditemukan atau sudah tidak berlaku. Kembali ke Beranda."
            );
            return;
          }
          setOffline(true);
          attempts += 1;
          retryTimer = window.setTimeout(
            load,
            Math.min(15000, 1000 * 2 ** attempts)
          );
        });
    };
    load();
    return () => {
      cancelled = true;
      if (retryTimer) window.clearTimeout(retryTimer);
    };
  }, [quizId, reloadKey]);

  useEffect(() => {
    if (expiresAt === null) return;
    const tick = () => {
      const left = Math.max(0, expiresAt - Date.now());
      setRemaining(left);
      if (left <= 0 && Date.now() >= submitRetryAtRef.current) {
        void doSubmit();
      }
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [expiresAt]);

  // Simpan draf jawaban tiap perubahan — aman terhadap reload/back.
  useEffect(() => {
    if (!quizId || expiresAt === null || submitting) return;
    saveQuizDraft({
      quizId,
      subject,
      answers,
      flagged,
      current,
      phase,
      expiresAt,
      savedAt: Date.now(),
    });
  }, [quizId, expiresAt, submitting, subject, answers, flagged, current, phase]);

  // Konfirmasi sebelum menutup/meninggalkan tab saat ujian masih berjalan.
  useEffect(() => {
    if (expiresAt === null) return;
    const handler = (e: BeforeUnloadEvent) => {
      const hasProgress = Object.values(answersRef.current).some(
        (v) => v !== undefined && v !== null && v !== ""
      );
      if (!hasProgress) return;
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [expiresAt]);

  function isAnswered(idx: number): boolean {
    const v = answers[String(idx)];
    if (v === undefined) return false;
    return typeof v === "string" ? v !== "" : true;
  }

  const answeredCount = useMemo(
    () => (questions ? questions.filter((_, i) => isAnswered(i)).length : 0),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [questions, answers]
  );
  const flaggedCount = useMemo(
    () => Object.values(flagged).filter(Boolean).length,
    [flagged]
  );

  if (error && !questions) {
    return (
      <Card>
        <CardContent className="flex flex-col gap-3">
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
          <a href="/" className="text-primary font-semibold">
            Kembali ke Beranda
          </a>
        </CardContent>
      </Card>
    );
  }

  if (!questions) {
    return (
      <Card>
        <CardContent className="flex flex-col gap-3">
          <p className="text-muted-foreground">Memuat soal…</p>
          {offline && (
            <Alert variant="warning">
              <AlertTriangle />
              <AlertDescription>
                Koneksi bermasalah — menyambungkan ulang otomatis. Jawaban yang
                sudah tersimpan tetap aman di perangkat ini.
              </AlertDescription>
            </Alert>
          )}
          <Button
            variant="outline"
            className="w-fit"
            onClick={() => setReloadKey((k) => k + 1)}
          >
            Coba Lagi
          </Button>
        </CardContent>
      </Card>
    );
  }

  const isWarning = remaining < 60000;
  const q = questions[current];
  const isLast = current === questions.length - 1;
  const currentFlagged = !!flagged[String(current)];
  const unansweredCount = questions.length - answeredCount;

  const navItems: NavigatorItem[] = questions.map((_, i) => ({
    className: isAnswered(i)
      ? "border-primary bg-primary text-primary-foreground"
      : "border-input bg-card hover:bg-accent/50",
    dot: !!flagged[String(i)],
  }));

  function goTo(i: number) {
    setCurrent(i);
    setPhase("exam");
    setNavOpen(false);
  }
  function goPrev() {
    setCurrent((c) => Math.max(0, c - 1));
  }
  function goNext() {
    setCurrent((c) => Math.min(questions!.length - 1, c + 1));
  }
  function toggleFlag() {
    setFlagged((f) => ({ ...f, [String(current)]: !f[String(current)] }));
  }

  return (
    <div className="flex flex-col gap-4">
      <Card className="sticky top-14 z-10 gap-2 py-4 shadow-md">
        <CardContent className="flex flex-col gap-3 px-4">
          <div className="flex items-center justify-between gap-3">
            <div className="text-sm sm:text-base">
              <span className="font-extrabold">{subject}</span>{" "}
              <span className="text-muted-foreground">
                · Kelas {grade}
                {examType && ` · ${examType}`}
              </span>
            </div>
            <Badge
              variant={isWarning ? "destructive" : "secondary"}
              className={cn(isWarning && "animate-pulse")}
            >
              <Clock />
              {formatTime(remaining)}
            </Badge>
          </div>
          <div className="flex flex-col gap-1.5">
            <Progress value={(answeredCount / questions.length) * 100} />
            <span className="text-muted-foreground text-xs">
              {answeredCount} dari {questions.length} soal terjawab
            </span>
          </div>
        </CardContent>
      </Card>

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {restored && phase === "exam" && (
        <Alert variant="success">
          <Save />
          <AlertDescription>
            Jawaban sebelumnya berhasil dipulihkan — kamu bisa lanjut dari
            terakhir kali mengerjakan.
          </AlertDescription>
        </Alert>
      )}

      {phase === "exam" && (
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start">
          <div className="flex min-w-0 flex-1 flex-col gap-4">
            {/* Navigator soal - versi mobile (dapat dibuka/tutup) */}
            <div className="lg:hidden">
              <Button
                variant="outline"
                onClick={() => setNavOpen((o) => !o)}
                className="w-full justify-between"
              >
                <span>
                  Soal {current + 1} dari {questions.length}
                </span>
                <LayoutGrid />
              </Button>
              {navOpen && (
                <Card className="mt-2 gap-3 p-4">
                  <NavigatorGrid items={navItems} current={current} onSelect={goTo} />
                  <NavigatorLegend items={NAV_LEGEND} />
                </Card>
              )}
            </div>

            <Card>
              <CardHeader>
                <div className="flex items-center gap-2">
                  <Badge variant="secondary">Soal {current + 1}</Badge>
                  <span className="text-muted-foreground text-xs font-semibold">
                    {QUESTION_TYPE_LABELS[q.tipe]}
                  </span>
                </div>
                <h3 className="mt-1 text-base font-bold leading-snug">
                  <MathText text={q.pertanyaan} />
                </h3>
              </CardHeader>
              <CardContent>
                <QuestionImage url={q.gambar} />
                {q.tipe === "pilihan_ganda" && (
                  <RadioGroup
                    value={
                      answers[String(current)] !== undefined
                        ? String(answers[String(current)])
                        : ""
                    }
                    onValueChange={(v) =>
                      setAnswers({ ...answers, [String(current)]: Number(v) })
                    }
                    className="grid grid-cols-1 gap-3 sm:grid-cols-2"
                  >
                    {q.opsi?.map((opt, oi) => {
                      const selected = answers[String(current)] === oi;
                      return (
                        <Label
                          key={oi}
                          htmlFor={`q-${current}-${oi}`}
                          className={cn(
                            "flex cursor-pointer items-center gap-3 rounded-lg border px-3.5 py-2.5 font-normal shadow-sm transition-colors",
                            selected
                              ? "border-primary bg-accent"
                              : "border-input hover:bg-accent/50"
                          )}
                        >
                          <RadioGroupItem
                            value={String(oi)}
                            id={`q-${current}-${oi}`}
                            disabled={submitting}
                          />
                          <MathText text={opt} />
                        </Label>
                      );
                    })}
                  </RadioGroup>
                )}
                {q.tipe === "benar_salah" && (
                  <RadioGroup
                    value={
                      typeof answers[String(current)] === "string"
                        ? (answers[String(current)] as string)
                        : ""
                    }
                    onValueChange={(v) =>
                      setAnswers({ ...answers, [String(current)]: v })
                    }
                    className="grid grid-cols-1 gap-3 sm:grid-cols-2"
                  >
                    {(["benar", "salah"] as const).map((v) => {
                      const selected = answers[String(current)] === v;
                      return (
                        <Label
                          key={v}
                          htmlFor={`q-${current}-${v}`}
                          className={cn(
                            "flex cursor-pointer items-center justify-center gap-3 rounded-lg border px-3.5 py-4 font-bold shadow-sm transition-colors",
                            selected
                              ? "border-primary bg-accent"
                              : "border-input hover:bg-accent/50"
                          )}
                        >
                          <RadioGroupItem
                            value={v}
                            id={`q-${current}-${v}`}
                            disabled={submitting}
                          />
                          {v === "benar" ? "Benar" : "Salah"}
                        </Label>
                      );
                    })}
                  </RadioGroup>
                )}
                {q.tipe === "isian" && (
                  <Input
                    placeholder="Tulis jawaban singkat di sini"
                    value={
                      typeof answers[String(current)] === "string"
                        ? (answers[String(current)] as string)
                        : ""
                    }
                    onChange={(e) =>
                      setAnswers({ ...answers, [String(current)]: e.target.value })
                    }
                    disabled={submitting}
                  />
                )}
                {q.tipe === "deskripsi" && (
                  <Textarea
                    placeholder="Tulis jawaban uraianmu di sini"
                    rows={6}
                    value={
                      typeof answers[String(current)] === "string"
                        ? (answers[String(current)] as string)
                        : ""
                    }
                    onChange={(e) =>
                      setAnswers({ ...answers, [String(current)]: e.target.value })
                    }
                    disabled={submitting}
                  />
                )}
              </CardContent>
            </Card>

            <div className="bg-background sticky bottom-0 -mx-4 flex items-center justify-between gap-2 border-t px-4 py-3 lg:static lg:mx-0 lg:border-0 lg:bg-transparent lg:px-0 lg:py-0">
              <Button variant="outline" onClick={goPrev} disabled={current === 0}>
                <ChevronLeft />
                Sebelumnya
              </Button>
              <Button
                variant="outline"
                onClick={toggleFlag}
                className={cn(
                  currentFlagged && "border-warning bg-warning/10 text-warning-foreground"
                )}
              >
                <Flag className={cn(currentFlagged && "fill-warning")} />
                {currentFlagged ? "Ditandai" : "Tandai"}
              </Button>
              <Button onClick={() => (isLast ? setPhase("review") : goNext())}>
                {isLast ? "Selesai & Tinjau" : "Selanjutnya"}
                <ChevronRight />
              </Button>
            </div>
          </div>

          <Card className="top-28 hidden w-72 shrink-0 gap-4 p-5 lg:sticky lg:flex lg:max-h-[calc(100vh-7rem)] lg:flex-col lg:overflow-auto">
            <div className="text-sm font-extrabold">Navigasi Soal</div>
            <NavigatorGrid items={navItems} current={current} onSelect={goTo} />
            <NavigatorLegend items={NAV_LEGEND} />
            <Separator />
            <div className="flex flex-col gap-1.5">
              <Progress value={(answeredCount / questions.length) * 100} />
              <span className="text-muted-foreground text-xs">
                {answeredCount} dari {questions.length} soal terjawab
              </span>
            </div>
            <Button onClick={() => setPhase("review")}>
              Tinjau &amp; Kumpulkan Ujian
            </Button>
          </Card>
        </div>
      )}

      {phase === "review" && (
        <div className="flex flex-col gap-4">
          <div>
            <h2 className="text-lg font-extrabold">Tinjau Jawaban Kamu</h2>
            <p className="text-muted-foreground text-sm">
              Periksa sekali lagi sebelum mengumpulkan ujian.
            </p>
          </div>

          {unansweredCount > 0 && (
            <Alert variant="warning">
              <AlertTriangle />
              <AlertDescription>
                Masih ada {unansweredCount} soal yang belum dijawab.
              </AlertDescription>
            </Alert>
          )}

          <Card className="gap-0 overflow-hidden py-0">
            {questions.map((qq, i) => {
              const answered = isAnswered(i);
              const isF = !!flagged[String(i)];
              return (
                <button
                  key={qq.nomor}
                  type="button"
                  onClick={() => goTo(i)}
                  className="hover:bg-muted flex w-full items-center gap-3 border-b px-4 py-3 text-left last:border-b-0"
                >
                  <Badge variant={answered ? "default" : "secondary"}>{i + 1}</Badge>
                  <span className="flex-1 truncate text-sm font-semibold">
                    <MathText text={qq.pertanyaan} />
                  </span>
                  <span
                    className={cn(
                      "shrink-0 text-xs font-bold",
                      answered ? "text-primary" : "text-destructive"
                    )}
                  >
                    {answered ? "Terjawab" : "Belum dijawab"}
                    {isF && " · Ditandai"}
                  </span>
                </button>
              );
            })}
          </Card>

          <p className="text-muted-foreground text-center text-sm">
            {answeredCount} terjawab · {unansweredCount} belum dijawab · {flaggedCount}{" "}
            ditandai
          </p>

          <div className="flex justify-center gap-3">
            <Button variant="outline" onClick={() => setPhase("exam")}>
              Kembali ke Soal
            </Button>
            <Button onClick={() => void doSubmit(true)} disabled={submitting}>
              {submitting ? "Mengumpulkan…" : "Konfirmasi & Kumpulkan"}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
