"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Heart } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { likePost, qk } from "@/lib/queries";
import { useAuth } from "@/lib/auth";
import { cn } from "@/lib/utils";

interface LikeButtonProps {
  postId: number;
  likesCount: number;
  liked: boolean;
  /** Обновление списка лайков после мутации (опционально). */
  onDone?: (likesCount: number, liked: boolean) => void;
  className?: string;
}

export function LikeButton({
  postId,
  likesCount,
  liked,
  onDone,
  className,
}: LikeButtonProps) {
  const { user, loading } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  // Локальное состояние — мгновенный отклик, сервер синхронизируется мутацией.
  const [state, setState] = useState({ count: likesCount, liked });

  const mutation = useMutation({
    mutationFn: (next: boolean) => likePost(postId, next),
    onSuccess: (data, next) => {
      setState({ count: data.likes_count, liked: next });
      onDone?.(data.likes_count, next);
      void queryClient.invalidateQueries({ queryKey: qk.feed });
      void queryClient.invalidateQueries({ queryKey: qk.post(postId) });
    },
    onError: (err) => toast.error(err.message),
  });

  const toggle = () => {
    if (loading) return;
    if (!user) {
      toast.info("Войдите, чтобы ставить лайки");
      router.push("/login");
      return;
    }
    if (mutation.isPending) return;
    const next = !state.liked;
    setState((s) => ({
      count: Math.max(0, s.count + (next ? 1 : -1)),
      liked: next,
    }));
    mutation.mutate(next);
  };

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={state.liked ? "Убрать лайк" : "Поставить лайк"}
      className={cn(
        "flex cursor-pointer items-center gap-1.5 rounded-full px-2 py-1 text-sm text-muted-foreground transition-colors hover:text-[#ffe066]",
        state.liked && "text-[#ffe066]",
        className,
      )}
    >
      <Heart
        className={cn("h-4 w-4 transition-transform", state.liked && "fill-current")}
      />
      <span className="tabular-nums">{state.count}</span>
    </button>
  );
}
