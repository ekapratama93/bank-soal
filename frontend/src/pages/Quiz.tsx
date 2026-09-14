import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useBlocker, useNavigate, useParams } from "react-router-dom";
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  Clock,
  Flag,
  LayoutGrid,
  Save,
} from "lucide-react";
import { ApiError, getQuiz, type QuestionPublic } from "../api/client";
import { submitWithReconcile } from "../lib/quizSubmit";
import { serverNow } from "../lib/serverTime";
import { markServed } from "../storage/results";
import {
  clearQuizDraft,
  loadQuizDraft,
  reconcileDraft,
  saveQuizDraft,
  type QuizDraft,
  type ReconciledDraft,
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

/** Jangan auto-submit dalam sekian milidetik pertama setelah kuis dimuat. */
const MOUNT_GRACE_MS = 1500;
/** Tunda penulisan draf selama siswa masih mengetik. */
const DRAFT_DEBOUNCE_MS = 800;
/** Tapi tetap simpan sesering ini walau mengetik terus. */
const DRAFT_MAX_WAIT_MS = 5000;
/** Percobaan kirim otomatis sebelum menyerah dan minta aksi siswa. */
const MAX_SUBMIT_ATTEMPTS = 5;

type SubmitStatus = "idle" | "submitting" | "waiting" | "done" | "gave_up";

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
  const [error, setError] = useState<string | null>(null);
  const [offline, setOffline] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  const [current, setCurrent] = useState(0);
  const [flagged, setFlagged] = useState<Record<string, boolean>>({});
  const [phase, setPhase] = useState<"exam" | "review">("exam");
  const [navOpen, setNavOpen] = useState(false);
  const [restored, setRestored] = useState(false);
  // Draf yang tidak cocok dengan attempt server — siswa yang memutuskan.
  const [staleDraft, setStaleDraft] = useState<ReconciledDraft | null>(null);
  const [storageBlocked, setStorageBlocked] = useState(false);

  const [submitStatus, setSubmitStatus] = useState<SubmitStatus>("idle");
  const [retryAt, setRetryAt] = useState<number | null>(null);

  const answersRef = useRef(answers);
  answersRef.current = answers;
  const inFlightRef = useRef(false);
  const settledRef = useRef(false);
  const submitAttemptRef = useRef(0);
  const expiredFiredRef = useRef(false);
  const pendingDraftRef = useRef<QuizDraft | null>(null);
  const lastSavedAtRef = useRef(0);

  // Selama pengumpulan berjalan, jawaban tidak boleh diubah lagi.
  const busy = submitStatus === "submitting" || submitStatus === "done";

  /** Tulis draf yang tertunda ke penyimpanan sekarang juga. */
  const flushDraft = useCallback(() => {
    const draft = pendingDraftRef.current;
    if (!draft) return;
    pendingDraftRef.current = null;
    lastSavedAtRef.current = Date.now();
    setStorageBlocked(saveQuizDraft(draft) !== "ok");
  }, []);

  /**
   * Satu percobaan pengumpulan. Kegagalan direkonsiliasi dulu (lihat
   * lib/quizSubmit.ts) supaya percobaan ulang tidak menilai ulang jawaban yang
   * sebenarnya sudah masuk.
   */
  const runSubmit = useCallback(async () => {
    if (inFlightRef.current || settledRef.current || !quizId) return;
    inFlightRef.current = true;
    setSubmitStatus("submitting");
    setError(null);
    flushDraft();

    const attempt = submitAttemptRef.current;
    const outcome = await submitWithReconcile(quizId, answersRef.current, attempt);
    inFlightRef.current = false;
    if (settledRef.current) return;

    if (outcome.status === "success" || outcome.status === "already-submitted") {
      settledRef.current = true;
      setSubmitStatus("done");
      setRetryAt(null);
      clearQuizDraft(quizId);
      navigate(`/result/${quizId}`, { state: { result: outcome.result } });
      return;
    }

    setError(outcome.error);
    if (outcome.status === "retry") {
      submitAttemptRef.current = attempt + 1;
      if (submitAttemptRef.current < MAX_SUBMIT_ATTEMPTS) {
        setSubmitStatus("waiting");
        setRetryAt(Date.now() + outcome.retryInMs);
        return;
      }
    }
    setSubmitStatus("gave_up");
    setRetryAt(null);
  }, [quizId, navigate, flushDraft]);

  /** Kumpulkan atas permintaan siswa — mulai lagi dari percobaan pertama. */
  const submitNow = useCallback(() => {
    submitAttemptRef.current = 0;
    setRetryAt(null);
    void runSubmit();
  }, [runSubmit]);

  const applyDraft = useCallback((d: ReconciledDraft) => {
    setAnswers(d.answers);
    setFlagged(d.flagged);
    setCurrent(d.current);
    setPhase(d.phase);
  }, []);

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
          // Pulihkan draf jawaban sebelumnya (mis. halaman sempat di-reload).
          const draft = reconcileDraft(
            loadQuizDraft(quizId),
            expiry,
            quiz.questions.length
          );
          if (!draft) return;
          const hasAnswers = Object.keys(draft.answers).length > 0;
          if (draft.match === "stale") {
            // Draf dari attempt lain: tawarkan, jangan buang diam-diam.
            if (hasAnswers) setStaleDraft(draft);
            return;
          }
          applyDraft(draft);
          if (hasAnswers) setRestored(true);
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
  }, [quizId, reloadKey, applyDraft]);

  // Hitung mundur memakai jam server (lihat lib/serverTime.ts) supaya jam
  // perangkat yang meleset tidak membuat ujian berakhir terlalu cepat/lambat.
  useEffect(() => {
    if (expiresAt === null) return;
    const graceUntil = Date.now() + MOUNT_GRACE_MS;
    const tick = () => {
      setRemaining(Math.max(0, expiresAt - serverNow()));
      if (expiresAt - serverNow() > 0) return;
      if (expiredFiredRef.current || settledRef.current) return;
      // Beri jeda sesaat setelah halaman dibuka: kalau kuis memang sudah lewat
      // waktunya, siswa sempat melihat pesannya sebelum jawaban dikirim.
      if (Date.now() < graceUntil) return;
      expiredFiredRef.current = true;
      void runSubmit();
    };
    tick();
    const id = window.setInterval(tick, 1000);
    // setInterval dibekukan saat tab tersembunyi / layar terkunci; hitung ulang
    // begitu halaman aktif lagi supaya auto-submit tidak tertinggal jauh.
    window.addEventListener("focus", tick);
    window.addEventListener("online", tick);
    document.addEventListener("visibilitychange", tick);
    return () => {
      window.clearInterval(id);
      window.removeEventListener("focus", tick);
      window.removeEventListener("online", tick);
      document.removeEventListener("visibilitychange", tick);
    };
  }, [expiresAt, runSubmit]);

  // Percobaan ulang terjadwal setelah pengumpulan gagal.
  useEffect(() => {
    if (submitStatus !== "waiting" || retryAt === null) return;
    const id = window.setTimeout(
      () => void runSubmit(),
      Math.max(0, retryAt - Date.now())
    );
    return () => window.clearTimeout(id);
  }, [submitStatus, retryAt, runSubmit]);

  // Koneksi kembali / tab dibuka lagi: jangan tunggu sisa backoff.
  useEffect(() => {
    if (submitStatus !== "waiting") return;
    const now = () => setRetryAt(Date.now());
    window.addEventListener("online", now);
    window.addEventListener("focus", now);
    return () => {
      window.removeEventListener("online", now);
      window.removeEventListener("focus", now);
    };
  }, [submitStatus]);

  // Simpan draf jawaban, ditunda selagi siswa mengetik.
  useEffect(() => {
    if (!quizId || expiresAt === null || settledRef.current || busy) return;
    pendingDraftRef.current = {
      quizId,
      subject,
      answers,
      flagged,
      current,
      phase,
      expiresAt,
      savedAt: Date.now(),
    };
    // Mengetik terus-menerus tidak boleh menunda simpan tanpa batas.
    if (Date.now() - lastSavedAtRef.current >= DRAFT_MAX_WAIT_MS) {
      flushDraft();
      return;
    }
    const id = window.setTimeout(flushDraft, DRAFT_DEBOUNCE_MS);
    return () => window.clearTimeout(id);
  }, [
    quizId,
    expiresAt,
    busy,
    subject,
    answers,
    flagged,
    current,
    phase,
    flushDraft,
  ]);

  // Tulis draf sebelum halaman ditutup/disembunyikan (penting di iOS).
  useEffect(() => {
    const onHide = () => flushDraft();
    window.addEventListener("pagehide", onHide);
    document.addEventListener("visibilitychange", onHide);
    return () => {
      window.removeEventListener("pagehide", onHide);
      document.removeEventListener("visibilitychange", onHide);
      flushDraft();
    };
  }, [flushDraft]);

  // Konfirmasi sebelum menutup/meninggalkan tab saat ujian masih berjalan.
  useEffect(() => {
    if (expiresAt === null) return;
    const handler = (e: BeforeUnloadEvent) => {
      const hasProgress = Object.values(answersRef.current).some(
        (v) => v !== undefined && v !== null && v !== ""
      );
      if (!hasProgress || settledRef.current) return;
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [expiresAt]);

  const hasProgress = Object.values(answers).some(
    (v) => v !== undefined && v !== null && v !== ""
  );
  // Pindah halaman di dalam aplikasi meng-unmount halaman ini — timer dan
  // auto-submit ikut mati — jadi tahan dulu dan minta konfirmasi.
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      questions !== null &&
      !settledRef.current &&
      submitStatus !== "done" &&
      remaining > 0 &&
      hasProgress &&
      currentLocation.pathname !== nextLocation.pathname
  );

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

      {blocker.state === "blocked" && (
        <Card className="border-warning">
          <CardContent className="flex flex-col gap-3">
            <div className="flex items-center gap-2 font-extrabold">
              <AlertTriangle className="text-warning" />
              Ujian masih berjalan
            </div>
            <p className="text-muted-foreground text-sm">
              Waktu terus berjalan dan jawabanmu belum dikumpulkan. Kalau keluar
              sekarang, jawaban tetap tersimpan di perangkat ini tetapi timer
              tidak berhenti.
            </p>
            <div className="flex flex-wrap gap-2">
              <Button onClick={() => blocker.reset?.()}>Lanjutkan Ujian</Button>
              <Button
                variant="outline"
                onClick={() => {
                  flushDraft();
                  blocker.proceed?.();
                }}
              >
                Keluar
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {storageBlocked && (
        <Alert variant="destructive">
          <AlertTriangle />
          <AlertDescription>
            Jawaban tidak bisa disimpan di perangkat ini. Jangan tutup halaman
            sampai ujian selesai dikumpulkan.
          </AlertDescription>
        </Alert>
      )}

      {staleDraft && (
        <Alert variant="warning">
          <Save />
          <AlertDescription className="flex flex-col items-start gap-2">
            <span>
              Ada draf jawaban dari sesi sebelumnya (
              {Object.keys(staleDraft.answers).length} soal terjawab). Pulihkan?
            </span>
            <span className="flex gap-2">
              <Button
                size="sm"
                onClick={() => {
                  applyDraft(staleDraft);
                  setStaleDraft(null);
                  setRestored(true);
                }}
              >
                Pulihkan
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setStaleDraft(null)}
              >
                Abaikan
              </Button>
            </span>
          </AlertDescription>
        </Alert>
      )}

      {error && (
        <Alert variant={submitStatus === "waiting" ? "warning" : "destructive"}>
          {submitStatus === "waiting" && <AlertTriangle />}
          <AlertDescription className="flex flex-col items-start gap-2">
            <span>{error}</span>
            {submitStatus === "waiting" && (
              <Button size="sm" variant="outline" onClick={submitNow}>
                Coba Sekarang
              </Button>
            )}
            {submitStatus === "gave_up" && (
              <span className="text-xs">
                Jawabanmu masih tersimpan di perangkat ini — coba kumpulkan lagi
                setelah koneksi membaik.
              </span>
            )}
          </AlertDescription>
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
                      setAnswers((prev) => ({ ...prev, [String(current)]: Number(v) }))
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
                            disabled={busy}
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
                      setAnswers((prev) => ({ ...prev, [String(current)]: v }))
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
                            disabled={busy}
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
                      setAnswers((prev) => ({ ...prev, [String(current)]: e.target.value }))
                    }
                    disabled={busy}
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
                      setAnswers((prev) => ({ ...prev, [String(current)]: e.target.value }))
                    }
                    disabled={busy}
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
            <Button onClick={submitNow} disabled={busy}>
              {submitStatus === "submitting"
                ? "Mengumpulkan…"
                : submitStatus === "gave_up"
                  ? "Coba Kumpulkan Lagi"
                  : "Konfirmasi & Kumpulkan"}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
