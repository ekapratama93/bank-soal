import * as React from "react";

import { cn } from "@/lib/utils";

/**
 * Menampilkan anak dengan animasi fade-up saat pertama kali masuk viewport.
 * Tanpa IntersectionObserver (mis. jsdom di test) konten langsung tampil.
 */
export function Reveal({
  className,
  delay = 0,
  style,
  children,
  ...props
}: React.ComponentProps<"div"> & { delay?: number }) {
  const ref = React.useRef<HTMLDivElement>(null);
  const [visible, setVisible] = React.useState(
    () => typeof IntersectionObserver === "undefined"
  );

  React.useEffect(() => {
    const el = ref.current;
    if (visible || !el) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "0px 0px -10% 0px" }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [visible]);

  return (
    <div
      ref={ref}
      className={cn(visible ? "animate-fade-up" : "opacity-0", className)}
      style={{ animationDelay: `${delay}ms`, ...style }}
      {...props}
    >
      {children}
    </div>
  );
}
