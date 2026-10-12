"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
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
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { useAuth } from "@/lib/auth";
import { api } from "@/lib/api";
import {
  registerSchema,
  resendSchema,
  type RegisterValues,
  type ResendValues,
} from "@/lib/schemas";
import type { SuccessResponse } from "@/lib/types";

export default function RegisterPage() {
  const { register } = useAuth();
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const [registeredEmail, setRegisteredEmail] = useState<string | null>(null);

  const form = useForm<RegisterValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: { username: "", email: "", password: "", confirm: "" },
  });

  const resendForm = useForm<ResendValues>({
    resolver: zodResolver(resendSchema),
    defaultValues: { email: "" },
  });

  const onSubmit = async (values: RegisterValues) => {
    setServerError(null);
    try {
      await register(values.username, values.email, values.password);
      setRegisteredEmail(values.email);
    } catch (err) {
      setServerError(err instanceof Error ? err.message : "Ошибка регистрации");
    }
  };

  const onResend = async (values: ResendValues) => {
    try {
      await api<SuccessResponse>("/resend-verification", {
        method: "POST",
        json: { email: values.email },
        noRefresh: true,
      });
      toast.success("Если аккаунт ожидает подтверждения — письмо отправлено");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Не удалось отправить");
    }
  };

  if (registeredEmail) {
    return (
      <main className="container relative z-10 mx-auto mt-12 mb-16 flex max-w-md justify-center px-5">
        <Card className="w-full bg-card shadow-[0_22px_27px_rgba(0,0,0,0.5)]">
          <CardHeader>
            <CardTitle className="text-2xl">Проверьте почту</CardTitle>
            <CardDescription>
              Мы отправили ссылку для подтверждения на{" "}
              <span className="text-foreground">{registeredEmail}</span>.
              Перейдите по ней, чтобы активировать аккаунт и получить право
              публиковать посты.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <Form {...resendForm}>
              <form
                onSubmit={resendForm.handleSubmit(onResend)}
                className="flex flex-col gap-3"
              >
                <FormField
                  control={resendForm.control}
                  name="email"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Не пришло письмо? Отправить ещё раз</FormLabel>
                      <FormControl>
                        <Input placeholder="ivan@example.com" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <Button
                  type="submit"
                  variant="secondary"
                  disabled={resendForm.formState.isSubmitting}
                  className="border border-[#323b4f] bg-[#272e3e]"
                >
                  Повторить отправку
                </Button>
              </form>
            </Form>
            <Button
              onClick={() => router.push("/login")}
              className="bg-[#ffe066] text-[#141722] hover:bg-[#ffd43b]"
            >
              Перейти ко входу
            </Button>
          </CardContent>
        </Card>
      </main>
    );
  }

  return (
    <main className="container relative z-10 mx-auto mt-12 mb-16 flex max-w-md justify-center px-5">
      <Card className="w-full bg-card shadow-[0_22px_27px_rgba(0,0,0,0.5)]">
        <CardHeader>
          <CardTitle className="text-2xl">Регистрация</CardTitle>
          <CardDescription>
            Латиница, цифры, «_» и «-»; пароль от 8 символов
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
                name="username"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Имя пользователя</FormLabel>
                    <FormControl>
                      <Input placeholder="ivan_petrov" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Email</FormLabel>
                    <FormControl>
                      <Input type="email" placeholder="ivan@example.com" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Пароль</FormLabel>
                    <FormControl>
                      <Input type="password" placeholder="Минимум 8 символов" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="confirm"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Повторите пароль</FormLabel>
                    <FormControl>
                      <Input type="password" placeholder="Ещё раз" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {serverError && (
                <p className="text-sm text-destructive">{serverError}</p>
              )}

              <Button
                type="submit"
                disabled={form.formState.isSubmitting}
                className="mt-1 bg-[#ffe066] text-[#141722] hover:bg-[#ffd43b]"
              >
                {form.formState.isSubmitting ? "Создаём…" : "Создать аккаунт"}
              </Button>

              <p className="text-center text-sm text-muted-foreground">
                Уже есть аккаунт?{" "}
                <Link href="/login" className="text-[#ffe066] hover:underline">
                  Войти
                </Link>
              </p>
            </form>
          </Form>
        </CardContent>
      </Card>
    </main>
  );
}
