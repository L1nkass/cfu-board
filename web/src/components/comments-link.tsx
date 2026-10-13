"use client";

import Link from "next/link";
import { MessageSquare } from "lucide-react";
import { formatCompact } from "@/lib/format";
import { cn } from "@/lib/utils";

interface CommentsLinkProps {
  postId: number;
  count: number;
  className?: string;
}

/**
 * Кликабельная иконка комментариев: ведёт на страницу поста
 * с якорем #comments (автоскролл к секции комментариев).
 */
export function CommentsLink({ postId, count, className }: CommentsLinkProps) {
  return (
    <Link
      href={`/post/${postId}#comments`}
      onClick={(e) => e.stopPropagation()}
      aria-label={`Перейти к комментариям (${count})`}
      className={cn(
        "flex items-center gap-1 rounded-full px-1 text-sm text-muted-foreground transition-colors hover:text-[#ffe066]",
        className,
      )}
    >
      <MessageSquare className="h-4 w-4" />
      <span className="tabular-nums">{formatCompact(count)}</span>
    </Link>
  );
}
