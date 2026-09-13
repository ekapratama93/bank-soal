import { beforeEach, describe, expect, it } from "vitest";
import { getServedIds, markServed } from "./results";

beforeEach(() => {
  localStorage.clear();
});

describe("served quiz ids", () => {
  it("menyimpan id kuis yang sudah disajikan", () => {
    expect(getServedIds()).toEqual([]);
    markServed("q1");
    markServed("q2");
    expect(getServedIds()).toEqual(["q2", "q1"]);
  });

  it("tidak menduplikasi id", () => {
    markServed("q1");
    markServed("q1");
    expect(getServedIds()).toEqual(["q1"]);
  });

  it("capped 200 id terbaru", () => {
    for (let i = 0; i < 210; i++) markServed(`q-${i}`);
    const ids = getServedIds();
    expect(ids).toHaveLength(200);
    expect(ids[0]).toBe("q-209");
    expect(ids.includes("q-0")).toBe(false);
  });
});