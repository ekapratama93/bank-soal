import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { clearAdminToken, getAdminToken, setAdminToken, useAdminToken } from "./adminAuth";

beforeEach(() => {
  localStorage.clear();
});

describe("admin token", () => {
  it("menyimpan dan menghapus token", () => {
    expect(getAdminToken()).toBeNull();
    setAdminToken("abc");
    expect(getAdminToken()).toBe("abc");
    clearAdminToken();
    expect(getAdminToken()).toBeNull();
  });

  it("useAdminToken ikut berubah saat login/keluar", () => {
    const { result } = renderHook(() => useAdminToken());
    expect(result.current).toBeNull();
    act(() => setAdminToken("abc"));
    expect(result.current).toBe("abc");
    act(() => clearAdminToken());
    expect(result.current).toBeNull();
  });
});
