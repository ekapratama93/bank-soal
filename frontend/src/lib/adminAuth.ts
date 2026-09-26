import { useSyncExternalStore } from "react";

// Token admin dipakai bersama oleh halaman Admin dan header (tombol Keluar),
// jadi disimpan di satu tempat dengan pemberitahuan ke semua pelanggan.
const TOKEN_KEY = "bank-soal-admin-token";

const listeners = new Set<() => void>();

function notify() {
  listeners.forEach((l) => l());
}

export function getAdminToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setAdminToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
  notify();
}

export function clearAdminToken() {
  localStorage.removeItem(TOKEN_KEY);
  notify();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  // Event storage hanya datang dari tab lain — login/keluar di tab lain ikut
  // tersinkron di sini.
  const onStorage = (e: StorageEvent) => {
    if (e.key === TOKEN_KEY || e.key === null) listener();
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
  };
}

export function useAdminToken(): string | null {
  return useSyncExternalStore(subscribe, getAdminToken, () => null);
}
