"use client";

import { Eye } from "lucide-react";
import { formatCompact } from "@/lib/format";
import { cn } from "@/lib/utils";

/** Счётчик просмотров: иконка глаза + компактное число (только чтение). */
export function ViewsCount({
  count,
  className,
}: {
  count: number;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "flex items-center gap-1.5 px-1 text-sm text-muted-foreground",
        className,
      )}
      title={`${new Intl.NumberFormat("ru-RU").format(count)} просмотров`}
      aria-label={`${count} просмотров`}
    >
      <Eye className="h-4 w-4" />
      <span className="tabular-nums">{formatCompact(count)}</span>
    </span>
  );
}
