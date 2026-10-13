"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { PostCard } from "@/components/post-card";
import { fetchFeed, PAGE_SIZE, qk } from "@/lib/queries";
import type { Post } from "@/lib/types";

/** Сетка постов (3 колонки, как в макете) с подгрузкой страниц. */
export function PostGrid({
  initialData,
  emptyHint,
}: {
  initialData?: Post[];
  emptyHint?: React.ReactNode;
}) {
  const {
    data,
    isLoading,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
    isError,
  } = useInfiniteQuery({
    queryKey: qk.feed,
    initialPageParam: 0,
    queryFn: ({ pageParam }) => fetchFeed(pageParam),
    getNextPageParam: (lastPage, pages) =>
      lastPage.length === PAGE_SIZE
        ? pages.reduce((n, p) => n + p.length, 0)
        : undefined,
    initialData: initialData
      ? { pages: [initialData], pageParams: [0] }
      : undefined,
  });

  if (isLoading) {
    return (
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton
            key={i}
            className="aspect-[4/3] rounded-[20px] bg-card"
          />
        ))}
      </div>
    );
  }

  const posts = data?.pages.flat() ?? [];

  if (isError) {
    return (
      <p className="py-12 text-center text-muted-foreground">
        Не удалось загрузить ленту. Обновите страницу.
      </p>
    );
  }

  if (posts.length === 0) {
    return (
      <div className="py-16 text-center">
        <p className="mb-6 text-lg text-muted-foreground">
          Здесь пока пусто — будьте первым!
        </p>
        {emptyHint}
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-8">
      <div className="grid w-full grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
        {posts.map((post) => (
          <PostCard key={post.id} post={post} />
        ))}
      </div>
      {hasNextPage && (
        <Button
          variant="secondary"
          onClick={() => fetchNextPage()}
          disabled={isFetchingNextPage}
          className="border border-[#323b4f] bg-card"
        >
          {isFetchingNextPage && <Loader2 className="h-4 w-4 animate-spin" />}
          Показать ещё
        </Button>
      )}
    </div>
  );
}
