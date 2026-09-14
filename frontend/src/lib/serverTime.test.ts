import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  getClockOffset,
  noteServerDate,
  noteServerDateHeader,
  resetServerTime,
  serverNow,
} from "./serverTime";

beforeEach(() => {
  resetServerTime();
  vi.useRealTimers();
});

describe("jam server", () => {
  it("tanpa sampel, memakai jam perangkat apa adanya", () => {
    vi.useFakeTimers();
    vi.setSystemTime(1_000_000);
    expect(getClockOffset()).toBe(0);
    expect(serverNow()).toBe(1_000_000);
  });

  it("menghitung selisih saat jam perangkat meleset jauh", () => {
    vi.useFakeTimers();
    vi.setSystemTime(1_000_000);
    // Server 10 menit di depan; bolak-balik 200ms.
    const server = 1_000_000 + 600_000;
    noteServerDate(server, 999_900, 1_000_000);
    // +500ms (presisi detik) +50ms (rtt/2)
    expect(getClockOffset()).toBe(600_550);
    expect(serverNow()).toBe(1_600_550);
  });

  it("mengabaikan selisih kecil yang cuma jitter jaringan", () => {
    vi.useFakeTimers();
    vi.setSystemTime(1_000_000);
    noteServerDate(1_000_000, 999_900, 1_000_000);
    expect(getClockOffset()).toBe(0);
  });

  it("mengabaikan sampel dengan bolak-balik terlalu lama", () => {
    vi.useFakeTimers();
    vi.setSystemTime(1_000_000);
    noteServerDate(1_600_000, 994_000, 1_000_000);
    expect(getClockOffset()).toBe(0);
  });

  it("mempertahankan sampel dengan bolak-balik tercepat", () => {
    vi.useFakeTimers();
    vi.setSystemTime(1_000_000);
    noteServerDate(1_600_000, 999_900, 1_000_000); // rtt 100
    const best = getClockOffset();
    noteServerDate(1_900_000, 996_000, 1_000_000); // rtt 4000, diabaikan
    expect(getClockOffset()).toBe(best);
  });

  it("mengabaikan header Date yang kosong atau tidak valid", () => {
    noteServerDateHeader(null, 0, 10);
    noteServerDateHeader("bukan tanggal", 0, 10);
    expect(getClockOffset()).toBe(0);
  });

  it("membaca header Date sungguhan", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-01-01T00:00:00Z"));
    noteServerDateHeader(
      "Thu, 01 Jan 2026 01:00:00 GMT",
      Date.now() - 100,
      Date.now()
    );
    expect(getClockOffset()).toBeGreaterThan(3_599_000);
  });
});
