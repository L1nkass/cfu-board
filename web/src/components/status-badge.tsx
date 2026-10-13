import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { PostStatus } from "@/lib/types";

const LABELS: Record<PostStatus, string> = {
  pending: "На модерации",
  approved: "Опубликован",
  rejected: "Отклонён",
};

export function StatusBadge({
  status,
  className,
}: {
  status: PostStatus;
  className?: string;
}) {
  if (status === "approved") return null;
  return (
    <Badge
      variant="outline"
      className={cn(
        "border-transparent font-medium",
        status === "pending" &&
          "bg-amber-500/15 text-amber-600 dark:text-amber-400",
        status === "rejected" &&
          "bg-destructive/15 text-destructive dark:text-red-400",
        className,
      )}
    >
      {LABELS[status]}
    </Badge>
  );
}
