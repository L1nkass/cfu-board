"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Loader2, ShieldAlert, X } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { useAuth } from "@/lib/auth";
import { timeAgo } from "@/lib/format";
import { rejectSchema, type RejectValues } from "@/lib/schemas";
import { fetchPending, moderate, qk } from "@/lib/queries";
import type { Post } from "@/lib/types";

function RejectDialog({
  post,
  open,
  onOpenChange,
}: {
  post: Post | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const form = useForm<RejectValues>({
    resolver: zodResolver(rejectSchema),
    defaultValues: { reason: "" },
  });

  const reject = useMutation({
    mutationFn: (values: RejectValues) =>
      moderate(post!.id, "reject", values.reason?.trim() || undefined),
    onSuccess: () => {
      toast.success("Публикация отклонена");
      onOpenChange(false);
      form.reset();
      void queryClient.invalidateQueries({ queryKey: qk.pending });
      void queryClient.invalidateQueries({ queryKey: qk.feed });
    },
    onError: (e) => toast.error(e.message),
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="bg-card">
        <DialogHeader>
          <DialogTitle>Отклонить публикацию</DialogTitle>
          <DialogDescription>
            {post?.title} — автор увидит причину отказа.
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((v) => reject.mutate(v))}
            className="flex flex-col gap-4"
          >
            <FormField
              control={form.control}
              name="reason"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Причина (необязательно)</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder="Например: нарушение правил сообщества"
                      rows={3}
                      maxLength={1000}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter>
              <Button
                type="button"
                variant="secondary"
                onClick={() => onOpenChange(false)}
                className="border border-[#323b4f] bg-[#272e3e]"
              >
                Отмена
              </Button>
              <Button
                type="submit"
                disabled={reject.isPending}
                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              >
                {reject.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
                Отклонить
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}

export default function AdminPage() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [rejectPost, setRejectPost] = useState<Post | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);

  useEffect(() => {
    if (loading) return;
    if (!user) {
      toast.info("Войдите, чтобы открыть модерацию");
      router.replace("/login");
    } else if (!user.is_admin) {
      toast.error("Раздел доступен только администраторам");
      router.replace("/");
    }
  }, [loading, user, router]);

  const { data: pending, isLoading } = useQuery({
    queryKey: qk.pending,
    queryFn: fetchPending,
    enabled: !!user?.is_admin,
  });

  const approve = useMutation({
    mutationFn: (id: number) => moderate(id, "approve"),
    onSuccess: () => {
      toast.success("Публикация одобрена");
      void queryClient.invalidateQueries({ queryKey: qk.pending });
      void queryClient.invalidateQueries({ queryKey: qk.feed });
    },
    onError: (e) => toast.error(e.message),
  });

  if (loading || !user?.is_admin) {
    return (
      <main className="container relative z-10 mx-auto mt-16 flex justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </main>
    );
  }

  return (
    <main className="container relative z-10 mx-auto mt-10 mb-16 max-w-4xl px-5">
      <div className="mb-6 flex items-center gap-3">
        <ShieldAlert className="h-6 w-6 text-[#ffe066]" />
        <h1 className="text-2xl font-bold">Очередь модерации</h1>
        <span className="text-muted-foreground">
          ({pending?.length ?? 0} ожидает)
        </span>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40 rounded-[20px] bg-card" />
          ))}
        </div>
      ) : !pending || pending.length === 0 ? (
        <p className="rounded-[20px] bg-card py-16 text-center text-muted-foreground shadow-[0_22px_27px_rgba(0,0,0,0.5)]">
          Очередь пуста — все публикации проверены.
        </p>
      ) : (
        <div className="flex flex-col gap-4">
          {pending.map((post) => (
            <article
              key={post.id}
              className="flex flex-col gap-4 rounded-[20px] bg-card p-5 shadow-[0_22px_27px_rgba(0,0,0,0.5)] sm:flex-row"
            >
              <div className="h-40 w-full shrink-0 overflow-hidden rounded-xl bg-[#1d222d] sm:h-28 sm:w-44">
                {post.image_url ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={post.image_url}
                    alt={post.title}
                    className="h-full w-full object-cover"
                  />
                ) : (
                  <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
                    без медиа
                  </div>
                )}
              </div>

              <div className="flex min-w-0 flex-1 flex-col gap-2">
                <div className="flex flex-wrap items-center gap-2">
                  <h2 className="line-clamp-1 font-semibold">{post.title}</h2>
                  <span className="text-xs text-muted-foreground">
                    {post.author_name} · {timeAgo(post.created_at)}
                  </span>
                </div>
                <p className="line-clamp-3 text-sm text-muted-foreground">
                  {post.content}
                </p>
                <div className="mt-auto flex gap-2">
                  <Button
                    size="sm"
                    disabled={approve.isPending}
                    onClick={() => approve.mutate(post.id)}
                    className="bg-[#ffe066] text-[#141722] hover:bg-[#ffd43b]"
                  >
                    <Check className="h-4 w-4" /> Одобрить
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => {
                      setRejectPost(post);
                      setDialogOpen(true);
                    }}
                    className="border border-destructive/40 text-destructive hover:bg-destructive/10"
                  >
                    <X className="h-4 w-4" /> Отклонить
                  </Button>
                </div>
              </div>
            </article>
          ))}
        </div>
      )}

      <RejectDialog
        post={rejectPost}
        open={dialogOpen}
        onOpenChange={setDialogOpen}
      />
    </main>
  );
}

