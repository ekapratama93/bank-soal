import { afterEach, vi } from "vitest";
import { cleanup } from "@testing-library/react";
import { resetServerTime } from "../lib/serverTime";

// Tanpa ini, timer palsu, stub global, dan isi localStorage bocor antar berkas test.
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
  resetServerTime();
});
