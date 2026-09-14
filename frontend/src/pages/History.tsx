import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import { getAttempts, type AttemptResult } from "../api/client";
import { formatDate } from "../storage/results";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
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
      <Card>
        <CardContent className="text-muted-foreground">Memuat riwayat…</CardContent>
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
        {attempts.map((a) => {
          const level = scoreLevel(a.nilai);
          return (
            <Link
              key={a.quiz_id}
              to={`/result/${a.quiz_id}`}
              className="hover:bg-muted flex items-center gap-3 border-b px-4 py-3 no-underline last:border-b-0"
            >
              <div
                className={cn(
                  "flex size-10 shrink-0 items-center justify-center rounded-full border-4 text-sm font-extrabold",
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
              <ChevronRight className="text-muted-foreground shrink-0" />
            </Link>
          );
        })}
      </Card>
    </div>
  );
}