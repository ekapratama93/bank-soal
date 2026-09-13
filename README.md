# Bank Soal

Aplikasi pembuat soal latihan sekolah dengan bantuan AI (OpenRouter). Siswa memilih mata pelajaran + kelas + tipe ujian, aplikasi menyajikan paket soal campuran (pilihan ganda, benar/salah, isian), timer countdown, penilaian otomatis + koreksi AI untuk soal isian, dan riwayat hasil tersimpan di browser.

**Semua teks UI, soal, dan pembahasan dalam Bahasa Indonesia.**

## Konsep utama

- **Tipe ujian** dikelola admin (Ujian Harian, Ujian Semester, dst.). Tiap tipe bisa mengatur jumlah soal, durasi, **komposisi tipe soal** (jumlah per tipe: pilihan ganda, benar/salah, isian singkat, uraian/deskripsi), dan **poin per tipe soal** (mis. pilihan ganda 2 poin, isian 5 poin, uraian 10 poin; tipe tak disebut = 1 poin). Jika kosong, mengikuti konfigurasi kelas dengan komposisi otomatis dan semua soal bernilai 1 poin. Nilai siswa = poin diperoleh ÷ total poin × 100. Soal uraian/deskripsi selalu dikoreksi & dinilai AI (skor 0–1 dikali poin tipe).
- **Mata pelajaran** dikelola admin (9 mapel awal sudah di-seed; bisa tambah/hapus lewat Panel Admin atau SQL `insert into public.subjects (name) values (...)`).
- **Materi** di-link ke tipe ujian. Isi materi bisa berupa teks, file PDF/TXT, atau **keduanya sekaligus** (teks digabung dengan isi file). Tiap materi menentukan **jumlah paket soal** (1–5) yang dibuat dari materi itu. Materi bisa diedit atau dihapus dari Daftar Materi.
- **Paket soal**: paket dibuat **oleh admin** lewat Panel Admin → "Paket Soal (Pool) → Generate Paket Soal" — AI membuat semua paket dari SEMUA materi untuk kombinasi yang dipilih. Siswa hanya **menerima** satu paket acak dari pool (tanpa memicu pembuatan soal); setiap klik "Ambil Soal" memberi paket berbeda di perangkat itu. Jika belum ada paket, siswa diberi tahu untuk menghubungi guru/admin. Setiap kali materi ditambah, diedit, atau dihapus, paket yang belum dimulai untuk kombinasi tersebut dibuang — admin generate ulang, dan batch baru memakai SEMUA materi untuk kombinasi itu (materi lama tetap jadi konteks).
- **Timer** mulai saat kuis pertama dibuka (diatur server, anti-refresh); paket yang belum dibuka tidak kedaluwarsa.
- **Reset Pool** di Panel Admin menghapus paket yang belum dimulai untuk kombinasi mapel+kelas+tipe ujian — dipakai setelah materi diperbarui.

## Struktur

```
backend/   FastAPI (Python) — API + integrasi OpenRouter + Supabase
frontend/  React (Vite + TypeScript)
schema.sql Skema database Supabase (backend/schema.sql)
```

## 1. Setup Supabase (wajib dilakukan lebih dulu)

1. Buat proyek gratis di [supabase.com](https://supabase.com).
2. Buka **Project Settings → API**, salin `Project URL` dan `service_role` key.
3. Buka **SQL Editor**, tempel seluruh isi `backend/schema.sql`, jalankan. Ini membuat tabel `profiles`, `exam_types`, `materials`, `quizzes`, `attempts`, trigger pembuatan profil otomatis, dan grant akses `service_role`.
4. Aktifkan **Authentication → Providers → Email**.
5. Buat user admin lewat **Authentication → Users → Add user** (email + password).
6. **PROMOSI ADMIN (wajib, sekali saja)** — di SQL Editor jalankan (ganti email):

   ```sql
   insert into public.profiles (id, email, role)
   select u.id, u.email, 'admin'
   from auth.users u
   where u.email = 'email-admin-anda@example.com'
   on conflict (id) do update set role = 'admin';
   ```

   Query ini juga berfungsi bila user dibuat **sebelum** `schema.sql` dijalankan (profil belum ada — dibuat sekaligus dengan role `admin`).

   Verifikasi:

   ```sql
   select id, email, role from public.profiles;
   ```

   Kolom `role` harus `admin`. Tanpa langkah ini, login akan ditolak (403).

## 2. Setup Backend

```bash
cd backend
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env
```

Isi `.env`:

| Variabel | Keterangan |
| --- | --- |
| `OPENROUTER_API_KEY` | Kunci API dari https://openrouter.ai/keys |
| `OPENROUTER_MODEL` | Model (default `z-ai/glm-4.5-air:free`, gratis) |
| `SUPABASE_URL` | URL proyek Supabase |
| `SUPABASE_SERVICE_KEY` | service_role key (server-side saja) |
| `FRONTEND_ORIGIN` | Origin frontend untuk CORS (dev) |

Jalankan:

```bash
uvicorn app.main:app --reload --port 8000
```

## 3. Setup Frontend

```bash
cd frontend
npm install
npm run dev
```

Terbuka di http://localhost:5173. Vite mem-proxy `/api` ke backend di port 8000.

## Cara pakai

- **Siswa** (tanpa login): Beranda → pilih mapel + kelas + tipe ujian → "Ambil Soal" (hanya mengambil paket yang sudah dibuat admin) → kerjakan → "Kumpulkan" (atau otomatis saat waktu habis) → lihat nilai + pembahasan. Setiap "Ambil Soal" berikutnya memberi paket berbeda. Riwayat tersimpan di perangkat lewat halaman "Riwayat".
- **Admin**: halaman "Admin" → login dengan akun ber-role `admin` → kelola **mata pelajaran** (tambah/hapus), **tipe ujian** (nama + jumlah soal/durasi opsional + komposisi tipe soal opsional, mis. 10 pilihan ganda + 5 isian + 5 uraian; jika komposisi diisi, total soal dihitung darinya), tambah/hapus **materi** (ketik manual atau unggah file PDF/TXT, tentukan jumlah paket 1–5), **Generate Paket Soal** (membuat batch paket via AI), dan **Reset Pool** per kombinasi.
- Jumlah soal/waktu per kelas: kelas 1–2 → 20 soal/60 menit, 3–4 → 25/60, 5–6 → 30/60, 7+ → 30/90 (`backend/app/grade_config.py`).

## Tests

Backend (menggunakan fake Supabase + stub LLM, tidak butuh API key):

```bash
cd backend
pip install -r requirements-dev.txt
python -m pytest
```

Frontend:

```bash
cd frontend
npm run test
```

## Deployment (Docker)

```bash
cp backend/.env.example backend/.env   # isi semua nilai
docker compose up --build
```

Frontend di http://localhost:8080; nginx mem-proxy `/api` ke backend sehingga CORS tidak diperlukan.

**Railway/Render**: hubungkan repo, deploy `backend/` dan `frontend/` sebagai dua service. Backend: set env `PORT`. Frontend: set env `BACKEND_HOST` (host/URL backend) dan `BACKEND_PORT` — konfigurasi nginx di-render otomatis dari `frontend/nginx.conf.template`.

## Catatan

- Jika schema.sql pernah dijalankan sebelumnya, jalankan ulang seluruh file — semua pernyataan idempoten (tabel & kolom baru ditambahkan otomatis).
- Jawaban benar & pembahasan tidak pernah dikirim ke browser sebelum kuis dikumpulkan.
- Riwayat hasil disimpan di server (tabel `attempts`) dan dikorelasikan per perangkat lewat cookie anonim `client_id` (httpOnly, berlaku ±1 tahun). Menghapus cookie membuat riwayat lama tidak lagi muncul di perangkat itu, tetapi data di Supabase tetap ada.
- PDF hasil **scan** (gambar) tidak terbaca — hanya PDF ber-teks dan TXT yang didukung, maksimal 2 MB.
- Submission setelah waktu habis tetap diterima dan ditandai `expired`.
- Menghapus tipe ujian atau mata pelajaran ditolak bila masih dipakai materi atau kuis; pindahkan/hapus materinya atau Reset Pool dulu.