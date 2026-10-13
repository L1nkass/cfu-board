"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Heart, MessageSquare, Trash2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { useAuth } from "@/lib/auth";
import { timeAgo } from "@/lib/format";
import {
  commentSchema,
  type CommentValues,
} from "@/lib/schemas";
import {
  createComment,
  deleteComment,
  likeComment,
  qk,
  fetchComments,
} from "@/lib/queries";
import { cn } from "@/lib/utils";
import type { Comment } from "@/lib/types";

function CommentItem({
  postId,
  comment,
  canModeratePost,
  onReply,
}: {
  postId: number;
  comment: Comment;
  canModeratePost: boolean;
  onReply: (root: Comment) => void;
}) {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [liked, setLiked] = useState({
    on: comment.liked_by_me,
    count: comment.likes_count,
  });

  const canDelete = user && (user.id === comment.author_id || canModeratePost);

  const del = useMutation({
    mutationFn: () => deleteComment(postId, comment.id),
    onSuccess: () => {
      toast.success("Комментарий удалён");
      void queryClient.invalidateQueries({ queryKey: qk.comments(postId) });
    },
    onError: (e) => toast.error(e.message),
  });

  const like = useMutation({
    mutationFn: (next: boolean) => likeComment(postId, comment.id, next),
    onSuccess: (data, next) => {
      setLiked({ on: next, count: data.likes_count });
      void queryClient.invalidateQueries({ queryKey: qk.comments(postId) });
    },
    onError: (e) => toast.error(e.message),
  });

  return (
    <div className="flex gap-3">
      <Link
        href={`/user/${comment.author_id}`}
        aria-label={`Профиль ${comment.author_name}`}
        className="block h-9 w-9 shrink-0 rounded-full transition-transform hover:scale-105"
      >
        <Avatar className="h-9 w-9 border-2 border-[#272e3e]">
          <AvatarImage src={comment.author_avatar || undefined} />
          <AvatarFallback className="bg-[#272e3e] text-sm">
            {comment.author_name.slice(0, 1).toUpperCase()}
          </AvatarFallback>
        </Avatar>
      </Link>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-2">
          <Link
            href={`/user/${comment.author_id}`}
            className="text-sm font-semibold transition-colors hover:text-[#ffe066] hover:underline"
          >
            {comment.author_name}
          </Link>
          <span className="text-xs text-muted-foreground">
            {timeAgo(comment.created_at)}
          </span>
        </div>
        <p className="text-sm whitespace-pre-wrap break-words">
          {comment.content}
        </p>
        <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className={cn(
              "h-auto gap-1 px-0 text-xs text-muted-foreground hover:bg-transparent hover:text-[#ffe066]",
              liked.on && "text-[#ffe066]",
            )}
            onClick={() => {
              if (!user) {
                toast.info("Войдите, чтобы ставить лайки");
                return;
              }
              if (!like.isPending) like.mutate(!liked.on);
            }}
          >
            <Heart className={cn("h-3.5 w-3.5", liked.on && "fill-current")} />
            {liked.count}
          </Button>
          {comment.parent_id === null && user && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-auto gap-1 px-0 text-xs text-muted-foreground hover:bg-transparent hover:text-[#ffe066]"
              onClick={() => onReply(comment)}
            >
              <MessageSquare className="h-3.5 w-3.5" /> Ответить
            </Button>
          )}
          {canDelete && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-auto gap-1 px-0 text-xs text-muted-foreground hover:bg-transparent hover:text-destructive"
              onClick={() => del.mutate()}
              disabled={del.isPending}
            >
              <Trash2 className="h-3.5 w-3.5" /> Удалить
            </Button>
          )}
        </div>

        {comment.replies.length > 0 && (
          <div className="mt-3 flex flex-col gap-3 border-l-2 border-border pl-3">
            {comment.replies.map((reply) => (
              <CommentItem
                key={reply.id}
                postId={postId}
                comment={reply}
                canModeratePost={canModeratePost}
                onReply={onReply}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

/** Список комментариев + форма ответа/создания. */
export function CommentSection({
  postId,
  authorId,
}: {
  postId: number;
  authorId: number;
}) {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [replyTo, setReplyTo] = useState<Comment | null>(null);

  const { data = [], isLoading } = useQuery({
    queryKey: qk.comments(postId),
    queryFn: () => fetchComments(postId),
  });

  const form = useForm<CommentValues>({
    resolver: zodResolver(commentSchema),
    defaultValues: { content: "" },
  });

  const create = useMutation({
    mutationFn: (values: CommentValues) =>
      createComment(postId, values.content, replyTo?.id),
    onSuccess: () => {
      toast.success(replyTo ? "Ответ отправлен" : "Комментарий отправлен");
      form.reset({ content: "" });
      setReplyTo(null);
      void queryClient.invalidateQueries({ queryKey: qk.comments(postId) });
      void queryClient.invalidateQueries({ queryKey: qk.post(postId) });
      void queryClient.invalidateQueries({ queryKey: qk.feed });
    },
    onError: (e) => toast.error(e.message),
  });

  return (
    <section className="flex flex-col gap-5">
      <h2 className="text-lg font-semibold">
        Комментарии{" "}
        <span className="text-muted-foreground">({data.length})</span>
      </h2>

      {user ? (
        <form
          onSubmit={form.handleSubmit((v) => create.mutate(v))}
          className="flex flex-col gap-2"
        >
          {replyTo && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span>
                Ответ для <b className="text-foreground">{replyTo.author_name}</b>
              </span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-auto px-0 text-xs text-muted-foreground underline hover:bg-transparent hover:text-[#ffe066]"
                onClick={() => setReplyTo(null)}
              >
                отменить
              </Button>
            </div>
          )}
          <Textarea
            placeholder={
              replyTo ? "Ваш ответ…" : "Напишите комментарий…"
            }
            rows={3}
            maxLength={2000}
            {...form.register("content")}
          />
          {form.formState.errors.content && (
            <p className="text-sm text-destructive">
              {form.formState.errors.content.message}
            </p>
          )}
          <div>
            <Button
              type="submit"
              disabled={create.isPending}
              className="bg-[#ffe066] text-[#141722] hover:bg-[#ffd43b]"
            >
              {create.isPending ? "Отправка…" : "Отправить"}
            </Button>
          </div>
        </form>
      ) : (
        <p className="text-sm text-muted-foreground">
          Войдите, чтобы оставлять комментарии.
        </p>
      )}

      {isLoading ? (
        <p className="text-sm text-muted-foreground">Загружаем…</p>
      ) : data.length === 0 ? (
        <p className="text-sm text-muted-foreground">Комментариев пока нет.</p>
      ) : (
        <div className="flex flex-col gap-5">
          {data.map((root) => (
            <CommentItem
              key={root.id}
              postId={postId}
              comment={root}
              canModeratePost={user?.id === authorId}
              onReply={setReplyTo}
            />
          ))}
        </div>
      )}
    </section>
  );
}

