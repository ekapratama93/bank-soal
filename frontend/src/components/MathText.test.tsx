import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import MathText from "./MathText";

describe("MathText", () => {
  it("renders plain text unchanged", () => {
    const { container } = render(<MathText text="Berapa hasil 2+2?" />);
    expect(container.textContent).toBe("Berapa hasil 2+2?");
    expect(container.querySelector(".katex")).toBeNull();
  });

  it("renders inline math as KaTeX", () => {
    const { container } = render(<MathText text="Nilai $x^2$ adalah" />);
    expect(container.querySelector(".katex")).not.toBeNull();
    expect(container.textContent).toContain("Nilai");
    expect(container.textContent).toContain("adalah");
  });

  it("renders display math as KaTeX", () => {
    const { container } = render(<MathText text="$$\frac{a}{b} = c$$" />);
    const katexEl = container.querySelector(".katex-display, .katex");
    expect(katexEl).not.toBeNull();
  });

  it("renders mixed plain text and math", () => {
    const { container } = render(<MathText text="Jika $a=1$ maka $b=2$." />);
    const katexEls = container.querySelectorAll(".katex");
    expect(katexEls.length).toBe(2);
    expect(container.textContent).toContain("Jika");
    expect(container.textContent).toContain("maka");
  });

  it("falls back to literal text on unbalanced delimiters", () => {
    const { container } = render(<MathText text="Harga $5 saja" />);
    expect(container.textContent).toBe("Harga $5 saja");
    expect(container.querySelector(".katex")).toBeNull();
  });

  it("does not treat separated currency amounts as math", () => {
    const text = "Total plus $5 and $10 more";
    const { container } = render(<MathText text={text} />);
    expect(container.textContent).toBe(text);
    expect(container.querySelector(".katex")).toBeNull();
  });

  it("wraps output with dir=auto for correct Arabic bidi rendering", () => {
    const text = "Hadis: إنَّمَا الأَعْمَالُ بِالنِّيَّاتِ";
    const { container } = render(<MathText text={text} />);
    expect(container.textContent).toBe(text);
    expect(container.querySelector("span[dir='auto']")).not.toBeNull();
  });

  it("wraps math-containing Arabic text with dir=auto", () => {
    const { container } = render(<MathText text="النتيجة $x^2$ صحيحة" />);
    expect(container.querySelector("span[dir='auto']")).not.toBeNull();
    expect(container.querySelector(".katex")).not.toBeNull();
  });
});
