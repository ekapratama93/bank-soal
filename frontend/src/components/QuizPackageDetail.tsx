import { Check } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import MathText from "@/components/MathText";
import QuestionImage from "@/components/QuestionImage";
import { QUESTION_TYPE_LABELS } from "@/lib/questionTypes";
import { cn } from "@/lib/utils";
import type { AdminQuestion } from "@/api/client";

const OPTION_LETTERS = ["A", "B", "C", "D"];

function formatJawaban(q: AdminQuestion): string {
  if (q.tipe === "benar_salah") {
    return q.jawaban === "benar" ? "Benar" : "Salah";
  }
  if (q.tipe === "pilihan_ganda" && q.opsi) {
    const idx = Number(q.jawaban);
    const opsi = q.opsi[idx];
    if (opsi !== undefined) {
      const huruf = OPTION_LETTERS[idx] ?? String(idx + 1);
      return `${huruf}. ${opsi}`;
    }
  }
  return String(q.jawaban);
}

export default function QuizPackageDetail({
  questions,
}: {
  questions: AdminQuestion[];
}) {
  return (
    <div className="flex flex-col gap-3">
      {questions.map((q) => (
        <div
          key={q.nomor}
          className="border-border/60 flex flex-col gap-2 rounded-xl border px-4 py-3.5"
        >
          <div className="flex items-center gap-2">
            <Badge variant="secondary">Soal {q.nomor}</Badge>
            <span className="text-muted-foreground text-xs font-semibold">
              {QUESTION_TYPE_LABELS[q.tipe]}
            </span>
          </div>
          <h4 className="text-sm font-bold leading-snug">
            <MathText text={q.pertanyaan} />
          </h4>
          <QuestionImage url={q.gambar} />
          {q.tipe === "pilihan_ganda" && q.opsi && (
            <div className="flex flex-col gap-1.5">
              {q.opsi.map((opt, i) => {
                const correct = i === Number(q.jawaban);
                return (
                  <div
                    key={i}
                    className={cn(
                      "flex items-center gap-2.5 rounded-lg border px-3 py-2 text-sm",
                      correct
                        ? "border-success bg-success/10 text-success"
                        : "border-input text-muted-foreground"
                    )}
                  >
                    <span className="font-extrabold">
                      {OPTION_LETTERS[i] ?? i + 1}.
                    </span>
                    <MathText text={opt} />
                    {correct && <Check className="ml-auto size-4 shrink-0" />}
                  </div>
                );
              })}
            </div>
          )}
          {q.tipe !== "pilihan_ganda" && (
            <p className="text-sm">
              Kunci jawaban: <strong><MathText text={formatJawaban(q)} /></strong>
            </p>
          )}
          <Separator />
          <div className="bg-muted border-primary rounded-md border-l-4 px-3.5 py-2.5 text-sm">
            <strong>Pembahasan:</strong> <MathText text={q.pembahasan} />
          </div>
        </div>
      ))}
    </div>
  );
}