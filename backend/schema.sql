-- Bank Soal — skema Supabase
-- Jalankan seluruh file ini di Supabase SQL Editor (aman dijalankan ulang / idempoten).

-- ============ profiles ============
create table if not exists public.profiles (
  id uuid primary key references auth.users (id) on delete cascade,
  email text not null,
  role text not null default 'student' check (role in ('admin', 'student')),
  created_at timestamptz not null default now()
);

-- Otomatis buat profile saat user baru mendaftar
create or replace function public.handle_new_user()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  insert into public.profiles (id, email, role)
  values (new.id, new.email, 'student')
  on conflict (id) do nothing;
  return new;
end;
$$;

drop trigger if exists on_auth_user_created on auth.users;
create trigger on_auth_user_created
  after insert on auth.users
  for each row execute function public.handle_new_user();

-- ============ exam_types ============
create table if not exists public.exam_types (
  id uuid primary key default gen_random_uuid(),
  name text not null unique,
  jumlah_soal int check (jumlah_soal between 5 and 50),
  durasi_menit int check (durasi_menit between 10 and 180),
  created_at timestamptz not null default now()
);

insert into public.exam_types (name)
select v.name
from (values ('Ujian Harian'), ('Ujian Tengah Semester'), ('Ujian Semester')) as v(name)
where not exists (select 1 from public.exam_types where name = v.name);

-- ============ subjects ============
create table if not exists public.subjects (
  id uuid primary key default gen_random_uuid(),
  name text not null unique,
  created_at timestamptz not null default now()
);

insert into public.subjects (name)
select v.name
from (values
  ('Matematika'), ('Bahasa Indonesia'), ('Bahasa Inggris'), ('IPA'), ('IPS'),
  ('PPKn'), ('Seni Budaya'), ('PJOK'), ('PAI/Agama')
) as v(name)
where not exists (select 1 from public.subjects where name = v.name);

-- ============ materials ============
create table if not exists public.materials (
  id uuid primary key default gen_random_uuid(),
  subject text not null,
  grade int not null check (grade between 1 and 12),
  title text not null,
  content text not null,
  created_by text,
  created_at timestamptz not null default now()
);

-- ============ quizzes ============
create table if not exists public.quizzes (
  id uuid primary key default gen_random_uuid(),
  subject text not null,
  grade int not null check (grade between 1 and 12),
  questions jsonb not null,
  expires_at timestamptz,
  created_at timestamptz not null default now()
);

-- ============ attempts ============
create table if not exists public.attempts (
  id uuid primary key default gen_random_uuid(),
  quiz_id uuid not null references public.quizzes (id) on delete cascade,
  answers jsonb not null,
  score jsonb not null,
  expired boolean not null default false,
  submitted_at timestamptz not null default now()
);

-- ============================================================
-- Kolom tambahan (idempoten — aman untuk database yang sudah ada)
-- ============================================================
alter table public.materials add column if not exists exam_type_id uuid references public.exam_types (id);
alter table public.materials add column if not exists file_name text;
-- jumlah_paket pindah dari materi ke input Generate Paket Soal (lihat POST /api/quiz/generate)
alter table public.materials drop column if exists jumlah_paket;

alter table public.quizzes add column if not exists exam_type_id uuid references public.exam_types (id);
alter table public.quizzes add column if not exists batch_id uuid;
alter table public.quizzes add column if not exists durasi_menit int;
alter table public.quizzes add column if not exists started boolean not null default false;

-- expires_at harus boleh NULL (timer dimulai saat kuis pertama dibuka)
alter table public.quizzes alter column expires_at drop not null;

-- Komposisi tipe soal per tipe ujian, mis. {"pilihan_ganda": 10, "isian": 5, "deskripsi": 5}.
-- NULL berarti komposisi otomatis (±40% pilihan ganda, ±20% benar/salah, sisanya isian).
alter table public.exam_types add column if not exists tipe_soal jsonb;

-- Bobot poin per tipe soal, mis. {"pilihan_ganda": 2, "isian": 5, "deskripsi": 10}.
-- NULL berarti semua soal bernilai 1 poin.
alter table public.exam_types add column if not exists poin_per_tipe jsonb;

-- attempts: korelasi anonim per perangkat lewat cookie client_id
alter table public.attempts add column if not exists client_id uuid;

-- attempts sekarang dibuat saat paket DIBUKA (bukan hanya saat submit), jadi
-- expires_at/started_at hidup di sini per (quiz, client) — bukan di quizzes —
-- supaya satu paket bisa dipakai banyak siswa sekaligus, masing-masing
-- dengan jam mulai/kedaluwarsa sendiri. quizzes.started/expires_at hanya
-- indikator "pernah dibuka siapa pun" untuk admin, tidak lagi menentukan pool.
alter table public.attempts add column if not exists started_at timestamptz not null default now();
alter table public.attempts add column if not exists expires_at timestamptz;
alter table public.attempts alter column answers drop not null;
alter table public.attempts alter column score drop not null;
alter table public.attempts alter column submitted_at drop not null;
alter table public.attempts alter column submitted_at drop default;

-- Satu siswa (client_id) hanya boleh punya satu attempt per paket soal —
-- ini yang mencegah siswa yang sama mengulang paket yang sama.
create unique index if not exists idx_attempts_quiz_client
  on public.attempts (quiz_id, client_id);

-- ============ indeks ============
-- PK & unique constraint sudah terindeks otomatis; FK dan kolom filter tidak.

create index if not exists idx_quizzes_exam_type on public.quizzes (exam_type_id);

-- Riwayat per perangkat: filter client_id + urut waktu
create index if not exists idx_attempts_client
  on public.attempts (client_id, submitted_at);
-- FK lookup & ON DELETE CASCADE
create index if not exists idx_attempts_quiz on public.attempts (quiz_id);

-- Ambil semua materi untuk kombinasi (juga melayani filter subject saja)
create index if not exists idx_materials_combo
  on public.materials (subject, grade, exam_type_id);
create index if not exists idx_materials_exam_type on public.materials (exam_type_id);

-- Klien backend memakai service_role; pastikan punya akses penuh ke semua tabel.
-- (Beberapa proyek Supabase baru tidak memberi grant ini otomatis.)
grant select, insert, update, delete on all tables in schema public to service_role;

-- Mata pelajaran bisa dikelola admin; tambah lewat Panel Admin atau SQL:
--
--   insert into public.subjects (name) values ('Sejarah')
--   on conflict (name) do nothing;
--
-- ============================================================
-- BOOTSTRAP ADMIN (jalankan SEKALI setelah membuat user admin
-- lewat Supabase Dashboard → Authentication → Add user).
-- Ganti email di bawah, lalu jalankan. Aman dijalankan ulang:
--   - jika profil belum ada (user dibuat sebelum schema.sql),
--     profil dibuat langsung dengan role admin;
--   - jika profil sudah ada, role diubah menjadi admin.
--
--   insert into public.profiles (id, email, role)
--   select u.id, u.email, 'admin'
--   from auth.users u
--   where u.email = 'email-admin-anda@example.com'
--   on conflict (id) do update set role = 'admin';
--
-- Verifikasi:
--
--   select id, email, role from public.profiles;
--   select id, name, jumlah_soal, durasi_menit, tipe_soal from public.exam_types;
--
-- Kolom role harus bernilai 'admin' untuk user tersebut.
-- ============================================================