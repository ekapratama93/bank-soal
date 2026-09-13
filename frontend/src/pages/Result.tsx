import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import {
  Check,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  LayoutGrid,
  MinusCircle,
  X,
  XCircle,
} from "lucide-react";
import { getAttempts, type AttemptResult } from "../api/client";
import { formatDate } from "../storage/results";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Textarea } from "@/components/ui/textarea";
import MathText from "@/components/MathText";
import QuestionImage from "@/components/QuestionImage";
import { NavigatorGrid, NavigatorLegend, type NavigatorItem } from "@/components/QuestionNavigator";
import { cn } from "@/lib/utils";
import { SCORE_MESSAGE, SCORE_RING_CLASS, scoreLevel } from "@/lib/score";
import { QUESTION_TYPE_LABELS } from "@/lib/questionTypes";
import type { PerQuestionResult, Verdict } from "@/api/client";

const NAV_LEGEND = [
  { dotClassName: "bg-success", label: "Benar" },
  { dotClassName: "bg-warning", label: "Parsial" },
  { dotClassName: "bg-destructive", label: "Salah" },
];

const VERDICT_NAV_CLASS: Record<Verdict, string> = {
  benar: "border-success bg-success text-success-foreground",
  parsial: "border-warning bg-warning text-warning-foreground",
  salah: "border-destructive bg-destructive text-destructive-foreground",
};

const VERDICT_LABEL: Record<string, string> = {
  benar: "Benar",
  parsial: "Parsial",
  salah: "Salah",
};

const VERDICT_ICON: Record<string, typeof CheckCircle2> = {
  benar: CheckCircle2,
  parsial: MinusCircle,
  salah: XCircle,
};

const VERDICT_DOT_CLASS: Record<string, string> = {
  benar: "bg-success text-success-foreground",
  parsial: "bg-warning text-warning-foreground",
  salah: "bg-destructive text-destructive-foreground",
};

function VerdictDot({ verdict }: { verdict: Verdict }) {
  const Icon = VERDICT_ICON[verdict];
  return (
    <span
      className={cn(
        "absolute -top-2 -right-2 flex size-6 items-center justify-center rounded-full shadow",
        VERDICT_DOT_CLASS[verdict]
      )}
    >
      <Icon className="size-4" />
    </span>
  );
}

function formatAnswer(value: string | number): string {
  if (value === "benar" || value === "salah") {
    return value === "benar" ? "Benar" : "Salah";
  }
  return String(value);
}

function OptionMarker({ state }: { state: "correct" | "wrong" | "neutral" }) {
  if (state === "correct") {
    return (
      <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-success text-success-foreground">
        <Check className="size-3.5" />
      </span>
    );
  }
  if (state === "wrong") {
    return (
      <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-destructive text-destructive-foreground">
        <X className="size-3.5" />
      </span>
    );
  }
  return <span className="border-muted-foreground/40 size-5 shrink-0 rounded-full border-2" />;
}

function MultipleChoiceReview({ pq }: { pq: PerQuestionResult }) {
  if (!pq.opsi || pq.opsi.length === 0) {
    return (
      <p className="text-sm">
        Jawaban kamu: <strong><MathText text={formatAnswer(pq.jawaban_siswa)} /></strong>
        <br />
        Jawaban benar: <strong><MathText text={formatAnswer(pq.jawaban_benar)} /></strong>
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-2">
      {pq.opsi.map((opt, oi) => {
        const isCorrect = opt === pq.jawaban_benar;
        const isWrongPick = !isCorrect && opt === pq.jawaban_siswa;
        const state = isCorrect ? "correct" : isWrongPick ? "wrong" : "neutral";
        return (
          <div
            key={oi}
            className={cn(
              "flex items-center gap-3 rounded-lg border px-3.5 py-2.5 font-normal",
              isCorrect && "border-success bg-success/10 text-success",
              isWrongPick && "border-destructive bg-destructive/10 text-destructive",
              !isCorrect && !isWrongPick && "border-input text-muted-foreground opacity-60"
            )}
          >
            <OptionMarker state={state} />
            <MathText text={opt} />
          </div>
        );
      })}
    </div>
  );
}

export default function Result() {
  const { quizId } = useParams<{ quizId: string }>();
  const location = useLocation();
  const [attempt, setAttempt] = useState<AttemptResult | null>(null);
  const [missing, setMissing] = useState(false);
  const [current, setCurrent] = useState(0);
  const [navOpen, setNavOpen] = useState(false);

  const fresh = useMemo<AttemptResult | null>(() => {
    const state = location.state as { result?: AttemptResult } | null;
    return state?.result ?? null;
  }, [location.state]);

  useEffect(() => {
    if (fresh || !quizId) return;
    getAttempts()
      .then((rows) => {
        const found = rows.find((r) => r.quiz_id === quizId);
        if (found) setAttempt(found);
        else setMissing(true);
      })
      .catch(() => setMissing(true));
  }, [fresh, quizId]);

  if (missing) {
    return (
      <Card>
        <CardContent className="flex flex-col gap-3">
          <p>Hasil tidak ditemukan. Kuis belum dikumpulkan atau riwayat sudah dihapus.</p>
          <Link to="/" className="text-primary font-semibold">
            Kembali ke Beranda
          </Link>
        </CardContent>
      </Card>
    );
  }

  const result = fresh ?? attempt;
  if (!result)
    return (
      <Card>
        <CardContent className="text-muted-foreground">Memuat hasil…</CardContent>
      </Card>
    );

  const level = scoreLevel(result.nilai);
  const pq = result.per_question[current];
  const isFirst = current === 0;
  const isLast = current === result.per_question.length - 1;
  const Icon = VERDICT_ICON[pq.verdict];

  const navItems: NavigatorItem[] = result.per_question.map((q) => ({
    className: VERDICT_NAV_CLASS[q.verdict],
  }));

  function goTo(i: number) {
    setCurrent(i);
    setNavOpen(false);
  }

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader className="items-center text-center">
          <CardTitle className="text-2xl">Hasil Ujian</CardTitle>
          <p className="text-muted-foreground text-sm">
            {result.subject} · Kelas {result.grade}
            {result.exam_type && ` · ${result.exam_type}`}
            {result.expired && " · dikumpulkan lewat waktu"}
            {result.submitted_at && ` · ${formatDate(result.submitted_at)}`}
          </p>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-2">
          <div
            className={cn(
              "flex size-32 items-center justify-center rounded-full border-8 text-4xl font-extrabold",
              SCORE_RING_CLASS[level]
            )}
          >
            {result.nilai}
          </div>
          <p className="text-muted-foreground text-xs">Nilai (skala 0–100)</p>
          <p className="text-center text-sm font-semibold">{SCORE_MESSAGE[level]}</p>
        </CardContent>
      </Card>

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
                Soal {current + 1} dari {result.per_question.length}
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
                <Badge variant="secondary">Soal {pq.nomor}</Badge>
                <span className="text-muted-foreground text-xs font-semibold">
                  {QUESTION_TYPE_LABELS[pq.tipe]}
                </span>
              </div>
              <h3 className="mt-1 text-base font-bold leading-snug">
                <MathText text={pq.pertanyaan} />
              </h3>
            </CardHeader>
            <CardContent className="flex flex-col gap-2">
              <QuestionImage url={pq.gambar} />

              {pq.tipe === "pilihan_ganda" && <MultipleChoiceReview pq={pq} />}

              {pq.tipe === "benar_salah" && (
                <>
                  <p className="text-sm">
                    Jawaban kamu: <strong><MathText text={formatAnswer(pq.jawaban_siswa)} /></strong>
                    <br />
                    Jawaban benar: <strong><MathText text={formatAnswer(pq.jawaban_benar)} /></strong>
                  </p>
                  <Badge
                    variant={
                      pq.verdict === "benar"
                        ? "success"
                        : pq.verdict === "parsial"
                          ? "warning"
                          : "destructive"
                    }
                    className="w-fit"
                  >
                    <Icon />
                    {VERDICT_LABEL[pq.verdict]}
                  </Badge>
                </>
              )}

              {(pq.tipe === "isian" || pq.tipe === "deskripsi") && (
                <>
                  <div className="relative w-full">
                    <Textarea
                      value={pq.jawaban_siswa || "(Tidak dijawab)"}
                      disabled
                      readOnly
                      className="min-h-0 w-full resize-none disabled:opacity-100"
                      rows={pq.tipe === "deskripsi" ? 5 : 2}
                    />
                    <VerdictDot verdict={pq.verdict} />
                  </div>
                  <p className="text-sm">
                    {pq.tipe === "deskripsi" ? "Jawaban model:" : "Jawaban benar:"}{" "}
                    <strong><MathText text={formatAnswer(pq.jawaban_benar)} /></strong>
                  </p>
                </>
              )}

              {typeof pq.poin === "number" && typeof pq.poin_maks === "number" && (
                <p className="text-sm font-semibold">
                  Skor: {pq.poin}/{pq.poin_maks} poin
                </p>
              )}

              {pq.umpan_balik && pq.umpan_balik !== "-" && (
                <p className="text-muted-foreground text-sm">
                  {pq.tipe === "isian" || pq.tipe === "deskripsi" ? "Koreksi AI: " : "Korektor: "}
                  {pq.umpan_balik}
                </p>
              )}
              <Separator />
              <div className="bg-muted border-l-4 border-primary rounded-md px-3.5 py-2.5 text-sm">
                <strong>Pembahasan:</strong> <MathText text={pq.pembahasan} />
              </div>
            </CardContent>
          </Card>

          <div className="flex items-center justify-between gap-2">
            <Button
              variant="outline"
              onClick={() => setCurrent((c) => Math.max(0, c - 1))}
              disabled={isFirst}
            >
              <ChevronLeft />
              Sebelumnya
            </Button>
            <Button
              variant="outline"
              onClick={() =>
                setCurrent((c) => Math.min(result.per_question.length - 1, c + 1))
              }
              disabled={isLast}
            >
              Selanjutnya
              <ChevronRight />
            </Button>
          </div>
        </div>

        <Card className="top-16 hidden w-72 shrink-0 gap-4 p-5 lg:sticky lg:flex lg:max-h-[calc(100vh-5rem)] lg:flex-col lg:overflow-auto">
          <div className="text-sm font-extrabold">Navigasi Soal</div>
          <NavigatorGrid items={navItems} current={current} onSelect={goTo} />
          <NavigatorLegend items={NAV_LEGEND} />
        </Card>
      </div>
    </div>
  );
}