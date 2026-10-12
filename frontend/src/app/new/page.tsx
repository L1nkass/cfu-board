"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Loader2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { apiUpload } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import {
  IMAGE_TYPES,
  VIDEO_TYPES,
  postSchema,
  type PostValues,
} from "@/lib/schemas";
import type { SuccessResponse } from "@/lib/types";

export default function NewPostPage() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const [image, setImage] = useState<File | null>(null);
  const [video, setVideo] = useState<File | null>(null);

  const form = useForm<PostValues>({
    resolver: zodResolver(postSchema),
    defaultValues: { title: "", content: "" },
  });

  // Гостям и неверифицированным — сюда нельзя.
  useEffect(() => {
    if (loading) return;
    if (!user) {
      toast.info("Войдите, чтобы публиковать");
      router.replace("/login");
    } else if (!user.is_verified) {
      toast.warning("Подтвердите email, чтобы публиковать посты");
      router.replace("/profile");
    }
  }, [loading, user, router]);

  const pickFile = (
    e: React.ChangeEvent<HTMLInputElement>,
    kind: "image" | "video",
  ) => {
    const file = e.target.files?.[0] ?? null;
    const allowed =
      kind === "image" ? IMAGE_TYPES : VIDEO_TYPES;
    if (file && !allowed.includes(file.type)) {
      toast.error(
        kind === "image"
          ? "Изображение должно быть JPG, PNG или WEBP"
          : "Видео должно быть MP4, MOV или AVI",
      );
      e.target.value = "";
      return;
    }
    kind === "image" ? setImage(file) : setVideo(file);
  };

  const onSubmit = async (values: PostValues) => {
    setServerError(null);
    try {
      const fd = new FormData();
      fd.append("title", values.title);
      fd.append("content", values.content);
      if (image) fd.append("image", image);
      if (video) fd.append("video", video);
      await apiUpload<SuccessResponse>("/posts/create", fd);
      toast.success("Публикация отправлена на модерацию");
      router.push("/profile");
    } catch (err) {
      setServerError(
        err instanceof Error ? err.message : "Не удалось создать пост",
      );
    }
  };

  return (
    <main className="container relative z-10 mx-auto mt-10 mb-16 flex max-w-2xl justify-center px-5">
      <Card className="w-full bg-card shadow-[0_22px_27px_rgba(0,0,0,0.5)]">
        <CardHeader>
          <CardTitle className="text-2xl">Новая публикация</CardTitle>
          <CardDescription>
            Пост появится в ленте после проверки модератором
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form
              onSubmit={form.handleSubmit(onSubmit)}
              className="flex flex-col gap-4"
            >
              <FormField
                control={form.control}
                name="title"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Заголовок</FormLabel>
                    <FormControl>
                      <Input
                        placeholder="Продам велосипед"
                        maxLength={255}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="content"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Текст</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder="Расскажите подробнее…"
                        rows={5}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className="grid gap-4 sm:grid-cols-2">
                <FormItem>
                  <FormLabel>Изображение (необязательно)</FormLabel>
                  <Input
                    type="file"
                    accept={IMAGE_TYPES.join(",")}
                    onChange={(e) => pickFile(e, "image")}
                  />
                  <FormDescription>JPG, PNG или WEBP</FormDescription>
                </FormItem>
                <FormItem>
                  <FormLabel>Видео (необязательно)</FormLabel>
                  <Input
                    type="file"
                    accept={VIDEO_TYPES.join(",")}
                    onChange={(e) => pickFile(e, "video")}
                  />
                  <FormDescription>MP4, MOV или AVI</FormDescription>
                </FormItem>
              </div>

              {serverError && (
                <p className="text-sm text-destructive">{serverError}</p>
              )}

              <div className="flex gap-3">
                <Button
                  type="submit"
                  disabled={form.formState.isSubmitting}
                  className="bg-[#ffe066] text-[#141722] hover:bg-[#ffd43b]"
                >
                  {form.formState.isSubmitting && (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  )}
                  Отправить на модерацию
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => router.back()}
                  className="border border-[#323b4f] bg-[#272e3e]"
                >
                  Отмена
                </Button>
              </div>
            </form>
          </Form>
        </CardContent>
      </Card>
    </main>
  );
}
