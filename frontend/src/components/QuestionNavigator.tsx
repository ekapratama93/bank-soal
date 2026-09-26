import { cn } from "@/lib/utils";

export interface NavigatorItem {
  /** Border/background/text classes for this button's state (answered, verdict, ...). */
  className: string;
  /** Small corner indicator (e.g. flagged-for-review dot). */
  dot?: boolean;
  dotClassName?: string;
}

export function NavigatorGrid({
  items,
  current,
  onSelect,
}: {
  items: NavigatorItem[];
  current: number;
  onSelect: (i: number) => void;
}) {
  return (
    <div className="grid grid-cols-5 gap-2">
      {items.map((item, i) => {
        const isCurrent = i === current;
        return (
          <button
            key={i}
            type="button"
            onClick={() => onSelect(i)}
            className={cn(
              "relative h-9 rounded-md border text-sm font-extrabold transition-all duration-200 hover:scale-105 active:scale-95",
              item.className,
              isCurrent && "ring-primary ring-offset-background scale-105 ring-2 ring-offset-2"
            )}
          >
            {i + 1}
            {item.dot && (
              <span
                className={cn(
                  "animate-pop border-card absolute -top-1 -right-1 size-2 rounded-full border",
                  item.dotClassName ?? "bg-warning"
                )}
              />
            )}
          </button>
        );
      })}
    </div>
  );
}

export function NavigatorLegend({
  items,
}: {
  items: { dotClassName: string; label: string }[];
}) {
  return (
    <div className="text-muted-foreground flex flex-col gap-1.5 text-xs font-semibold">
      {items.map((it, i) => (
        <div key={i} className="flex items-center gap-2">
          <span className={cn("inline-block size-2.5 rounded-full", it.dotClassName)} />
          {it.label}
        </div>
      ))}
    </div>
  );
}
