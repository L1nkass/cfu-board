"use client";

import Link from "next/link";
import { Plus } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { toast } from "sonner";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";

/** Пунктирная кнопка «+» из исходного макета (add_button). */
export function AddPostButton() {
  const { user, loading } = useAuth();
  const router = useRouter();

  const onClick = () => {
    if (loading) return;
    if (!user) {
      toast.info("Войдите, чтобы публиковать посты");
      router.push("/login");
      return;
    }
    if (!user.is_verified) {
      toast.warning("Подтвердите email, чтобы публиковать");
      router.push("/profile");
      return;
    }
    router.push("/new");
  };

  return (
    <button
      type="button"
      onClick={onClick}
      className="h-[50px] w-full cursor-pointer rounded-xl bg-card p-1.5 shadow-[0_22px_27px_rgba(0,0,0,0.5)] transition-transform hover:-translate-y-0.5"
    >
      <span className="flex h-full w-full items-center justify-center rounded-lg border-[3px] border-dashed border-[#323b4f]">
        <Plus className="h-6 w-6 text-[#323b4f]" strokeWidth={3} />
      </span>
    </button>
  );
}

/** Ссылка «Написать пост» для пустой ленты. */
export function EmptyFeedLink() {
  return (
    <Button
      asChild
      className="gap-2 bg-[#ffe066] px-5 py-2.5 font-medium text-[#141722] hover:bg-[#ffd43b]"
    >
      <Link href="/new">
        <Plus className="h-4 w-4" /> Написать первый пост
      </Link>
    </Button>
  );
}
