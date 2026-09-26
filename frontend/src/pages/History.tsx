import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import { getAttempts, type AttemptResult } from "../api/client";
import { formatDate } from "../storage/results";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { SCORE_RING_CLASS, scoreLevel } from "@/lib/score";
import { applySeo } from "../lib/seo";

export default function History() {
  const [attempts, setAttempts] = useState<AttemptResult[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    applySeo({ title: "Riwayat Hasil Latihan" });
  }, []);

  useEffect(() => {
    getAttempts()
      .then(setAttempts)
      .catch(() => setError("Gagal memuat riwayat. Coba muat ulang halaman."));
  }, []);

  if (error) {
    return (
      <Card>
        <CardContent className="flex flex-col gap-3">
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
          <Link to="/" className="text-primary font-semibold">
            Kembali ke Beranda
          </Link>
        </CardContent>
      </Card>
    );
  }

  if (attempts === null) {
    return (
      <Card className="gap-0 overflow-hidden py-0" aria-label="Memuat riwayat…">
        <span className="sr-only">Memuat riwayat…</span>
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center gap-3 border-b px-4 py-3 last:border-b-0">
            <Skeleton
              className="size-10 shrink-0 rounded-full"
              style={{ animationDelay: `${i * 150}ms` }}
            />
            <div className="flex flex-1 flex-col gap-2">
              <Skeleton className="h-4 w-1/2" style={{ animationDelay: `${i * 150}ms` }} />
              <Skeleton className="h-3 w-1/3" style={{ animationDelay: `${i * 150}ms` }} />
            </div>
          </div>
        ))}
      </Card>
    );
  }

  if (attempts.length === 0) {
    return (
      <Card>
        <CardContent className="flex flex-col gap-3">
          <p>Belum ada riwayat. Kerjakan soal dulu, hasilnya akan tersimpan di sini.</p>
          <Link to="/" className="text-primary font-semibold">
            Kembali ke Beranda
          </Link>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">Riwayat Hasil</CardTitle>
          <p className="text-muted-foreground text-sm">
            Riwayat tersimpan di server dan terkait dengan perangkat ini.
          </p>
        </CardHeader>
      </Card>
      <Card className="gap-0 overflow-hidden py-0">
        {attempts.map((a, i) => {
          const level = scoreLevel(a.nilai);
          return (
            <Link
              key={a.quiz_id}
              to={`/result/${a.quiz_id}`}
              className="animate-fade-up group hover:bg-muted flex items-center gap-3 border-b px-4 py-3 no-underline transition-colors duration-200 last:border-b-0"
              style={{ animationDelay: `${Math.min(i, 12) * 50}ms` }}
            >
              <div
                className={cn(
                  "flex size-10 shrink-0 items-center justify-center rounded-full border-4 text-sm font-extrabold transition-transform duration-300 ease-(--ease-spring) group-hover:scale-110",
                  SCORE_RING_CLASS[level]
                )}
              >
                {a.nilai}
              </div>
              <div className="min-w-0 flex-1 text-foreground">
                <strong>{a.subject}</strong> · Kelas {a.grade}
                {a.exam_type && ` · ${a.exam_type}`}
                <br />
                <span className="text-muted-foreground text-sm">
                  {a.submitted_at ? formatDate(a.submitted_at) : ""}
                  {typeof a.poin === "number" &&
                    typeof a.poin_maks === "number" &&
                    ` · ${a.poin}/${a.poin_maks} poin`}
                  {a.expired && " · lewat waktu"}
                </span>
              </div>
              <ChevronRight className="text-muted-foreground group-hover:text-primary shrink-0 transition-all duration-200 group-hover:translate-x-1" />
            </Link>
          );
        })}
      </Card>
    </div>
  );
}