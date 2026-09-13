import { Fragment, useMemo } from "react";
import katex from "katex";

// Splits on $$...$$ (display) and $...$ (inline), skipping escaped \$.
// The char right after the opening delimiter and right before the closing one
// must not be whitespace — the usual convention to avoid misreading plain
// currency amounts like "$5 and $10" as a math span.
const MATH_SPLIT_RE =
  /(\$\$(?:\\.|[^\\$])*?[^\\$\s]\$\$|\$(?!\$)(?:\\.|[^\\$])*?[^\\$\s]\$(?!\$))/g;

function renderMath(expr: string, displayMode: boolean): string {
  try {
    return katex.renderToString(expr, { throwOnError: false, displayMode });
  } catch {
    return expr;
  }
}

export default function MathText({ text }: { text: string }) {
  const parts = useMemo(() => {
    if (!text.includes("$")) return null;
    return text.split(MATH_SPLIT_RE).filter((part) => part !== "");
  }, [text]);

  if (!parts) {
    return <span dir="auto">{text}</span>;
  }

  return (
    <span dir="auto">
      {parts.map((part, i) => {
        if (part.startsWith("$$") && part.endsWith("$$") && part.length >= 4) {
          const html = renderMath(part.slice(2, -2), true);
          return <span key={i} dangerouslySetInnerHTML={{ __html: html }} />;
        }
        if (part.startsWith("$") && part.endsWith("$") && part.length >= 2) {
          const html = renderMath(part.slice(1, -1), false);
          return <span key={i} dangerouslySetInnerHTML={{ __html: html }} />;
        }
        return <Fragment key={i}>{part}</Fragment>;
      })}
    </span>
  );
}
