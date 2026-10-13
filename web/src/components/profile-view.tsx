"use client";

import { useQuery } from "@tanstack/react-query";
import { BadgeCheck, ShieldCheck } from "lucide-react";
import { toast } from "sonner";
import { PostCard } from "@/components/post-card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { fetchUserPosts, qk } from "@/lib/queries";
import type { SuccessResponse } from "@/lib/types";

export interface ProfileHead {
  id: number;
  username: string;
  avatar_url?: string;
  email?: string;
  is_verified: boolean;
  is_admin: boolean;
  created_at: string;
  posts_count: number;
}

/** Шапка профиля + сетка публикаций автора. */
export function ProfileView({
  profile,
  isOwn,
}: {
  profile: ProfileHead;
  isOwn?: boolean;
}) {
  const { data: posts, isLoading } = useQuery({
    queryKey: qk.userPosts(profile.id),
    queryFn: () => fetchUserPosts(profile.id),
  });

  const resend = async () => {
    try {
      await api<SuccessResponse>("/resend-verification", {
        method: "POST",
        json: { email: profile.email },
        noRefresh: true,
      });
      toast.success("Письмо отправлено (если аккаунт не подтверждён)");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Не удалось отправить");
    }
  };

  return (
    <main className="container relative z-10 mx-auto mt-10 mb-16 px-5">
      <section className="rounded-[20px] bg-card p-6 shadow-[0_22px_27px_rgba(0,0,0,0.5)]">
        <div className="flex flex-wrap items-center gap-5">
          <Avatar className="h-20 w-20 border-4 border-[#272e3e]">
            <AvatarImage src={profile.avatar_url || undefined} />
            <AvatarFallback className="bg-[#272e3e] text-2xl">
              {profile.username.slice(0, 1).toUpperCase()}
            </AvatarFallback>
          </Avatar>
          <div className="flex min-w-0 flex-col gap-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-bold">{profile.username}</h1>
              {profile.is_verified && (
                <Badge
                  variant="outline"
                  className="gap-1 border-emerald-500/40 text-emerald-600 dark:text-emerald-400"
                >
                  <BadgeCheck className="h-3.5 w-3.5" /> Подтверждён
                </Badge>
              )}
              {profile.is_admin && (
                <Badge
                  variant="outline"
                  className="gap-1 border-[#ffe066]/50 text-[#ffe066]"
                >
                  <ShieldCheck className="h-3.5 w-3.5" /> Админ
                </Badge>
              )}
            </div>
            {profile.email && (
              <span className="text-sm text-muted-foreground">
                {profile.email}
              </span>
            )}
            <span className="text-sm text-muted-foreground">
              На сайте с {formatDate(profile.created_at)} · Публикаций:{" "}
              {profile.posts_count}
            </span>
          </div>
        </div>

        {isOwn && !profile.is_verified && (
          <Alert className="mt-4 flex flex-wrap items-center gap-3 border-amber-500/30 bg-amber-500/10">
            <AlertDescription className="flex flex-wrap items-center gap-3 text-amber-600 dark:text-amber-400">
              Email не подтверждён — публиковать пока нельзя.
              <Button
                size="sm"
                variant="secondary"
                onClick={resend}
                className="border border-[#323b4f] bg-[#272e3e]"
              >
                Отправить письмо ещё раз
              </Button>
            </AlertDescription>
          </Alert>
        )}
      </section>

      <section className="mt-8">
        <h2 className="mb-4 text-lg font-semibold">Публикации</h2>
        {isLoading ? (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="aspect-[4/3] rounded-[20px] bg-card" />
            ))}
          </div>
        ) : !posts || posts.length === 0 ? (
          <p className="py-8 text-center text-muted-foreground">
            Публикаций пока нет.
          </p>
        ) : (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {posts.map((post) => (
              <PostCard key={post.id} post={post} />
            ))}
          </div>
        )}
      </section>
    </main>
  );
}
