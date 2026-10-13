"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, MessageSquare, Trash2 } from "lucide-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { CommentSection } from "@/components/comment-section";
import { LikeButton } from "@/components/like-button";
import { PostFrame } from "@/components/post-frame";
import { StatusBadge } from "@/components/status-badge";
import { ViewsCount } from "@/components/views-count";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuth } from "@/lib/auth";
import { formatCompact } from "@/lib/format";
import { formatDate, timeAgo } from "@/lib/format";
import { deletePost, fetchPost, qk } from "@/lib/queries";

export default function PostPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params.id);
  const router = useRouter();
  const queryClient = useQueryClient();
  const { user } = useAuth();

  const { data: post, isLoading, isError } = useQuery({
    queryKey: qk.post(id),
    queryFn: () => fetchPost(id),
    enabled: Number.isFinite(id),
  });

  // Локальный счётчик просмотров — мгновенно обновляется после лайка
  // (бэк отдаёт свежий views_count в ответе POST /likes).
  const [views, setViews] = useState<number | null>(null);
  useEffect(() => {
    if (post) setViews(post.views_count);
  }, [post?.id, post?.views_count]);

  const remove = useMutation({
    mutationFn: () => deletePost(id),
    onSuccess: () => {
      toast.success("Публикация удалена");
      void queryClient.invalidateQueries({ queryKey: qk.feed });
      router.push("/");
    },
    onError: (e) => toast.error(e.message),
  });

  if (isLoading) {
    return (
      <main className="container relative z-10 mx-auto mt-10 mb-16 max-w-3xl px-5">
        <Skeleton className="aspect-[4/3] rounded-[20px] bg-card" />
        <Skeleton className="mt-6 h-8 w-2/3 bg-card" />
        <Skeleton className="mt-3 h-24 w-full bg-card" />
      </main>
    );
  }

  if (isError || !post) {
    return (
      <main className="container relative z-10 mx-auto mt-16 mb-16 px-5 text-center">
        <p className="text-lg text-muted-foreground">
          Публикация не найдена или скрыта.
        </p>
        <Button
          onClick={() => router.push("/")}
          className="mt-4 bg-[#ffe066] text-[#141722] hover:bg-[#ffd43b]"
        >
          К ленте
        </Button>
      </main>
    );
  }

  const isAuthor = user?.id === post.author_id;
  const showStatus = post.status !== "approved" && (isAuthor || user?.is_admin);

  return (
    <main className="container relative z-10 mx-auto mt-10 mb-16 max-w-3xl px-5">
      <article className="overflow-hidden rounded-[20px] bg-card shadow-[0_22px_27px_rgba(0,0,0,0.5)]">
        <div className="relative">
          <PostFrame />
          {post.image_url && (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={post.image_url}
              alt={post.title}
              className="max-h-[480px] w-full object-cover"
            />
          )}
          {post.video_url && (
            <video src={post.video_url} controls className="max-h-[480px] w-full" />
          )}
          {!post.image_url && !post.video_url && (
            <div className="h-40 w-full bg-[#1d222d]" />
          )}
        </div>

        <div className="flex flex-col gap-4 p-6">
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-2xl font-bold">{post.title}</h1>
            {showStatus && <StatusBadge status={post.status} />}
          </div>

          {showStatus && post.status === "pending" && (
            <Alert>
              <AlertDescription>
                Публикация на модерации — она появится в ленте после одобрения.
              </AlertDescription>
            </Alert>
          )}
          {showStatus && post.status === "rejected" && (
            <Alert variant="destructive">
              <AlertDescription>
                Публикация отклонена модератором.
                {post.rejection_reason && <> Причина: {post.rejection_reason}</>}
              </AlertDescription>
            </Alert>
          )}

          <p className="whitespace-pre-wrap text-foreground/90">
            {post.content}
          </p>

          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-4">
            <div className="flex items-center gap-3">
              <Link
                href={`/user/${post.author_id}`}
                aria-label={`Профиль ${post.author_name}`}
                className="block shrink-0 rounded-full transition-transform hover:scale-105"
              >
                <Avatar className="h-10 w-10 border-2 border-[#272e3e]">
                  <AvatarImage src={post.author_avatar || undefined} alt={post.author_name} />
                  <AvatarFallback className="bg-[#272e3e] text-sm">
                    {post.author_name.slice(0, 1).toUpperCase()}
                  </AvatarFallback>
                </Avatar>
              </Link>
              <div className="flex flex-col">
                <Link
                  href={`/user/${post.author_id}`}
                  className="font-medium transition-colors hover:text-[#ffe066] hover:underline"
                >
                  {post.author_name}
                </Link>
                <span className="text-xs text-muted-foreground" title={formatDate(post.created_at)}>
                  {timeAgo(post.created_at)}
                </span>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <ViewsCount count={views ?? post.views_count} className="text-base" />
              <a
                href="#comments"
                className="flex items-center gap-1.5 rounded-full px-2 py-1 text-base text-muted-foreground transition-colors hover:text-[#ffe066]"
                aria-label={`Перейти к комментариям (${post.comments_count})`}
              >
                <MessageSquare className="h-4 w-4" />
                <span className="tabular-nums">{formatCompact(post.comments_count)}</span>
              </a>
              <LikeButton
                postId={post.id}
                likesCount={post.likes_count}
                liked={post.liked_by_me}
                className="text-base"
                onDone={(_likes, _liked, viewsCount) => setViews(viewsCount)}
              />
              {isAuthor && (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={remove.isPending}
                  onClick={() => remove.mutate()}
                  className="border border-destructive/40 text-destructive hover:bg-destructive/10"
                >
                  {remove.isPending ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Trash2 className="h-4 w-4" />
                  )}
                  Удалить
                </Button>
              )}
            </div>
          </div>
        </div>
      </article>

      <div
        id="comments"
        className="mt-8 scroll-mt-24 rounded-[20px] bg-card p-6 shadow-[0_22px_27px_rgba(0,0,0,0.5)]"
      >
        <CommentSection postId={post.id} authorId={post.author_id} />
      </div>
    </main>
  );
}
