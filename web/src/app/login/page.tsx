"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
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
import { loginSchema, type LoginValues } from "@/lib/schemas";

export default function LoginPage() {
  const { login } = useAuth();
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);

  const form = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { login: "", password: "" },
  });

  const onSubmit = async (values: LoginValues) => {
    setServerError(null);
    try {
      await login(values.login, values.password);
      toast.success("С возвращением!");
      router.push("/");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Не удалось войти";
      setServerError(message);
    }
  };

  return (
    <main className="container relative z-10 mx-auto mt-12 mb-16 flex max-w-md justify-center px-5">
      <Card className="w-full bg-card shadow-[0_22px_27px_rgba(0,0,0,0.5)]">
        <CardHeader>
          <CardTitle className="text-2xl">Вход</CardTitle>
          <CardDescription>
            Войдите по имени пользователю или email
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
                name="login"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Логин или email</FormLabel>
                    <FormControl>
                      <Input placeholder="ivan_petrov" {...field} />
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
                      <Input type="password" placeholder="••••••••" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {serverError && (
                <Alert variant="destructive">
                  <AlertDescription>{serverError}</AlertDescription>
                </Alert>
              )}

              <Button
                type="submit"
                disabled={form.formState.isSubmitting}
                className="mt-1 bg-[#ffe066] text-[#141722] hover:bg-[#ffd43b]"
              >
                {form.formState.isSubmitting ? "Входим…" : "Войти"}
              </Button>

              <p className="text-center text-sm text-muted-foreground">
                Нет аккаунта?{" "}
                <Link href="/register" className="text-[#ffe066] hover:underline">
                  Зарегистрироваться
                </Link>
              </p>
            </form>
          </Form>
        </CardContent>
      </Card>
    </main>
  );
}
