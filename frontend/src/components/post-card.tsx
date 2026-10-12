"use client";

import Link from "next/link";
import { MessageSquare } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { LikeButton } from "@/components/like-button";
import { PostFrame } from "@/components/post-frame";
import { StatusBadge } from "@/components/status-badge";
import { timeAgo } from "@/lib/format";
import type { Post } from "@/lib/types";

/** Карточка поста для сетки ленты (оригинальный макет: медио 70% + низ). */
export function PostCard({ post }: { post: Post }) {
  const showStatus = post.status !== "approved";

  return (
    <article className="group relative aspect-[4/3] overflow-hidden rounded-[20px] bg-card shadow-[0_22px_27px_rgba(0,0,0,0.5)] transition-transform duration-300 hover:-translate-y-1">
      <PostFrame />

      {/* Медиа (верхние 70%). */}
      <Link
        href={`/post/${post.id}`}
        className="absolute inset-x-0 top-0 z-10 block h-[70%] overflow-hidden"
        aria-label={post.title}
      >
        {post.image_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={post.image_url}
            alt={post.title}
            className="h-full w-full object-cover"
            loading="lazy"
          />
        ) : post.video_url ? (
          <video
            src={post.video_url}
            className="h-full w-full object-cover"
            muted
            preload="metadata"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center bg-[#1d222d] p-6 dark:bg-[#1d222d]">
            <span className="line-clamp-3 text-center text-lg font-semibold text-muted-foreground">
              {post.title}
            </span>
          </div>
        )}
      </Link>

      {/* Статус модерации (для pending/rejected своих постов). */}
      {showStatus && (
        <div className="absolute top-3 right-4 z-30">
          <StatusBadge status={post.status} />
        </div>
      )}

      {/* Нижняя зона: аватар, заголовок, счётчики. */}
      <div className="absolute inset-x-0 bottom-0 top-[70%] z-30">
        <Link
          href={`/user/${post.author_id}`}
          className="absolute -top-6 left-5 block h-16 w-16 rounded-full transition-transform hover:scale-105"
          aria-label={`Профиль ${post.author_name}`}
          onClick={(e) => e.stopPropagation()}
        >
          <Avatar className="h-16 w-16 border-[3px] border-[#272e3e] bg-card">
            <AvatarImage src={post.author_avatar || undefined} alt={post.author_name} />
            <AvatarFallback className="bg-[#272e3e] text-xl text-foreground">
              {post.author_name.slice(0, 1).toUpperCase()}
            </AvatarFallback>
          </Avatar>
        </Link>

        <div className="flex h-full flex-col justify-center gap-1 pr-4 pl-24">
          <Link
            href={`/post/${post.id}`}
            className="line-clamp-1 font-semibold text-foreground transition-colors group-hover:text-[#ffe066]"
          >
            {post.title}
          </Link>
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-xs text-muted-foreground">
              <Link
                href={`/user/${post.author_id}`}
                className="font-medium text-foreground/80 transition-colors hover:text-[#ffe066] hover:underline"
                onClick={(e) => e.stopPropagation()}
              >
                {post.author_name}
              </Link>{" "}
              · {timeAgo(post.created_at)}
            </span>
            <div className="flex shrink-0 items-center gap-1">
              <LikeButton
                postId={post.id}
                likesCount={post.likes_count}
                liked={post.liked_by_me}
              />
              <span className="flex items-center gap-1 px-1 text-sm text-muted-foreground">
                <MessageSquare className="h-4 w-4" />
                {post.comments_count}
              </span>
            </div>
          </div>
        </div>
      </div>
    </article>
  );
}
