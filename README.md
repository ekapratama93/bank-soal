# Bank Soal

Aplikasi pembuat soal latihan sekolah dengan bantuan AI (OpenRouter). Siswa memilih mata pelajaran + kelas + tipe ujian, aplikasi menyajikan paket soal campuran (pilihan ganda, benar/salah, isian), timer countdown, penilaian otomatis (isian dicocokkan lokal, uraian/deskripsi dikoreksi AI), dan riwayat hasil tersimpan di browser.

**Semua teks UI, soal, dan pembahasan dalam Bahasa Indonesia.**

## Konsep utama

- **Tipe ujian** dikelola admin (Ujian Harian, Ujian Semester, dst.). Tiap tipe bisa mengatur jumlah soal, durasi, **komposisi tipe soal** (jumlah per tipe: pilihan ganda, benar/salah, isian singkat, uraian/deskripsi), dan **poin per tipe soal** (mis. pilihan ganda 2 poin, isian 5 poin, uraian 10 poin; tipe tak disebut = 1 poin). Jika kosong, mengikuti konfigurasi kelas dengan komposisi otomatis dan semua soal bernilai 1 poin. Nilai siswa = poin diperoleh ÷ total poin × 100. Soal uraian/deskripsi selalu dikoreksi & dinilai AI (skor 0–1 dikali poin tipe).
- **Mata pelajaran** dikelola admin (9 mapel awal sudah di-seed; bisa tambah/hapus lewat Panel Admin atau SQL `insert into public.subjects (name) values (...)`).
- **Materi** di-link ke tipe ujian. Isi materi bisa berupa teks, file PDF/TXT, atau **keduanya sekaligus** (teks digabung dengan isi file). Tiap materi menentukan **jumlah paket soal** (1–5) yang dibuat dari materi itu. Materi bisa diedit atau dihapus dari Daftar Materi.
- **Paket soal**: paket dibuat **oleh admin** lewat Panel Admin → "Paket Soal (Pool) → Generate Paket Soal" — AI membuat semua paket dari SEMUA materi untuk kombinasi yang dipilih. Siswa hanya **menerima** satu paket acak dari pool (tanpa memicu pembuatan soal); setiap klik "Ambil Soal" memberi paket berbeda di perangkat itu. Jika belum ada paket, siswa diberi tahu untuk menghubungi guru/admin. Setiap kali materi ditambah, diedit, atau dihapus, paket yang belum dimulai untuk kombinasi tersebut dibuang — admin generate ulang, dan batch baru memakai SEMUA materi untuk kombinasi itu (materi lama tetap jadi konteks). Paket yang sudah dibuat bisa dikelola admin di tab **Kuis**: lihat daftar semua paket (bisa difilter mapel/kelas/tipe ujian), lihat isi soal beserta kunci jawaban & pembahasan, **perbaiki soal hasil AI yang tidak valid** (ubah pertanyaan, opsi, kunci jawaban, pembahasan, tipe soal, hapus atau tambah soal — divalidasi dengan aturan yang sama seperti generator AI), dan hapus paket individual (menghapus paket juga menghapus riwayat pengerjaan paket itu).
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
- **Admin**: halaman "Admin" → login dengan akun ber-role `admin` → kelola **mata pelajaran** (tambah/hapus), **tipe ujian** (nama + jumlah soal/durasi opsional + komposisi tipe soal opsional, mis. 10 pilihan ganda + 5 isian + 5 uraian; jika komposisi diisi, total soal dihitung darinya), tambah/hapus **materi** (ketik manual atau unggah file PDF/TXT, tentukan jumlah paket 1–5), **Generate Paket Soal** (membuat batch paket via AI), **Reset Pool** per kombinasi, dan kelola **kuis/paket soal** yang sudah dibuat di tab "Kuis" (daftar + filter, lihat soal beserta kunci jawaban & pembahasan, edit soal untuk memperbaiki hasil AI, hapus paket individual).
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

**Render**: hubungkan repo, deploy `backend/` dan `frontend/` sebagai dua service. Backend: set env `PORT`. Frontend: set env `BACKEND_HOST` (host/URL backend) dan `BACKEND_PORT` — konfigurasi nginx di-render otomatis dari `frontend/nginx.conf.template`.

## Deployment (Railway)

Konfigurasi Railway disimpan sebagai **Infrastructure as Code** di `.railway/railway.ts` — satu file yang mendefinisikan dua service dari repo ini:

- **`backend`** — root directory `backend/`, build via `backend/Dockerfile`, healthcheck `/api/health`
- **`frontend`** — root directory `frontend/`, build Dockerfile nginx; `BACKEND_HOST` otomatis mengarah ke backend lewat private networking Railway, `BACKEND_PORT` mengikuti port backend

Karena root directory sudah diatur di file tersebut, Railway tidak mencoba mem-build repo root (sumber error "Railpack could not determine how to build the app").

### Langkah deploy

1. Install Railway CLI dan login:

   ```bash
   npm i -g @railway/cli   # atau: brew install railway
   railway login
   ```

2. Buat proyek Railway baru, atau hubungkan proyek yang sudah ada:

   ```bash
   railway init     # proyek baru
   # atau: railway link
   ```

3. Pasang dependensi IaC di root repo (sekali saja; `railway` SDK), lalu pratinjau dan terapkan konfigurasi:

   ```bash
   npm install
   railway config plan    # cek rencana perubahan
   railway config apply   # buat service backend + frontend
   ```

   Bila sebelumnya ada service lama yang dibuat manual (root = root repo), apply akan menandainya untuk dihapus — itu wajar; konfirmasi jika setuju, atau hapus manual di dashboard sebelum apply.

4. Isi variabel rahasia service `backend` (sekali saja):

   ```bash
   railway variable set OPENROUTER_API_KEY=xxx -s backend
   railway variable set SUPABASE_URL=https://xxxxx.supabase.co -s backend
   railway variable set SUPABASE_SERVICE_KEY=xxx -s backend
   ```

   atau lewat dashboard → service **backend** → tab Variables.

5. Deploy: `git push` ke GitHub (branch `main`) — kedua service otomatis ter-build dan ter-deploy dari `railway.ts` di atas. Alternatif tanpa push: `railway up -s <nama-service>` dari folder `backend/` atau `frontend/`.

6. Buat domain publik untuk tiap service (dashboard → Settings → Networking → Generate Domain, atau `railway domain`).

Setelah jalan, frontend mem-proxy `/api` ke backend lewat private networking (`BACKEND_HOST` = `${{backend.RAILWAY_PRIVATE_DOMAIN}}`), sehingga tidak perlu mengatur CORS. Alternatif bila private network tidak dipakai: beri domain publik ke backend, lalu set di service frontend `BACKEND_HOST=<domain-backend>.up.railway.app`, `BACKEND_PORT=443`, `BACKEND_SCHEME=https`. Mengubah konfigurasi service cukup edit `.railway/railway.ts`, lalu `railway config plan` dan `railway config apply`.

## Catatan

- Jika schema.sql pernah dijalankan sebelumnya, jalankan ulang seluruh file — semua pernyataan idempoten (tabel & kolom baru ditambahkan otomatis).
- Jawaban benar & pembahasan tidak pernah dikirim ke browser sebelum kuis dikumpulkan.
- Riwayat hasil disimpan di server (tabel `attempts`) dan dikorelasikan per perangkat lewat cookie anonim `client_id` (httpOnly, berlaku ±1 tahun). Menghapus cookie membuat riwayat lama tidak lagi muncul di perangkat itu, tetapi data di Supabase tetap ada.
- PDF hasil **scan** (gambar) tidak terbaca — hanya PDF ber-teks dan TXT yang didukung, maksimal 2 MB.
- Submission setelah waktu habis tetap diterima dan ditandai `expired`.
- Pengumpulan bersifat **idempoten**: mengirim ulang paket yang sudah dikumpulkan mengembalikan hasil yang tersimpan tanpa menilai ulang ke AI, sehingga percobaan ulang saat koneksi putus tidak menimpa nilai.
- Jawaban yang sedang dikerjakan disimpan sebagai draf di perangkat (localStorage) dan dipulihkan saat halaman dibuka ulang. Timer memakai jam server (header `Date`), jadi jam perangkat yang meleset tidak memengaruhi sisa waktu.
- Menghapus tipe ujian atau mata pelajaran ditolak bila masih dipakai materi atau kuis; pindahkan/hapus materinya atau Reset Pool dulu.
- Menghapus paket soal individual (Panel Admin → tab Kuis) juga menghapus riwayat attempt paket tersebut — konfirmasi ditampilkan lebih dulu untuk paket yang sudah pernah dibuka siswa.
- Login admin dibatasi 5 percobaan per 15 menit per alamat email (server, `backend/app/routers/auth.py`) — mencegah tebak-tebak password bertubi-tubi. Batas ini hidup selama proses backend berjalan (reset saat restart).
- Soal **isian** dinilai lokal lewat pencocokan string (toleransi kecil pada salah ketik/ejaan, `backend/app/text_match.py`), bukan lewat AI — tidak ada nilai parsial untuk isian (benar/salah). Hanya soal **uraian/deskripsi** yang dikoreksi AI, karena itu yang sungguh butuh penilaian kelengkapan & ketepatan isi. Perubahan ini mempercepat pengumpulan secara signifikan untuk paket yang komposisinya banyak isian.
- Kalau satu paket punya banyak soal uraian/deskripsi (komposisi khusus dari admin — komposisi otomatis tidak pernah menyertakan deskripsi), soal-soal itu dinilai AI dalam beberapa batch **paralel** (maks 4 soal per panggilan, `backend/app/llm.py` `GRADE_BATCH_SIZE`), bukan satu panggilan besar berurutan — waktu tunggu mendekati satu batch terbesar, bukan jumlah semua soal digabung.
- Jawaban isian berupa **angka** dibandingkan sebagai nilai, bukan string — notasi ribuan/desimal Indonesia ("1.000", "0,25") vs internasional ("1000", "0.25"), serta pecahan ("1/2" = "0.5" = "0,5"), semuanya dianggap sama asal nilainya sama.