"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { api } from "@/lib/api";
import type { SuccessResponse } from "@/lib/types";

type State =
  | { kind: "checking" }
  | { kind: "ok"; message: string }
  | { kind: "fail"; message: string };

function VerifyInner() {
  const token = useSearchParams().get("token");
  const [state, setState] = useState<State>({ kind: "checking" });

  useEffect(() => {
    if (!token) {
      setState({ kind: "fail", message: "В ссылке нет токена подтверждения" });
      return;
    }
    let alive = true;
    (async () => {
      try {
        const res = await api<SuccessResponse>(
          `/verify?token=${encodeURIComponent(token)}`,
          { noRefresh: true },
        );
        if (alive) setState({ kind: "ok", message: res.message });
      } catch (err) {
        if (alive)
          setState({
            kind: "fail",
            message:
              err instanceof Error ? err.message : "Ссылка недействительна",
          });
      }
    })();
    return () => {
      alive = false;
    };
  }, [token]);

  return (
    <main className="container relative z-10 mx-auto mt-12 mb-16 flex max-w-md justify-center px-5">
      <Card className="w-full bg-card shadow-[0_22px_27px_rgba(0,0,0,0.5)]">
        <CardHeader>
          <CardTitle className="text-2xl">Подтверждение email</CardTitle>
          <CardDescription>
            {state.kind === "checking" && "Проверяем ссылку из письма…"}
            {state.kind === "ok" && "Аккаунт активирован"}
            {state.kind === "fail" && "Не получилось подтвердить"}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-4">
          {state.kind === "checking" && (
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          )}
          {state.kind === "ok" && (
            <Alert>
              <AlertDescription>{state.message}</AlertDescription>
            </Alert>
          )}
          {state.kind === "fail" && (
            <Alert variant="destructive">
              <AlertDescription>{state.message}</AlertDescription>
            </Alert>
          )}
          <Button
            asChild
            className="bg-[#ffe066] text-[#141722] hover:bg-[#ffd43b]"
          >
            <Link href={state.kind === "ok" ? "/new" : "/login"}>
              {state.kind === "ok" ? "Написать пост" : "Ко входу"}
            </Link>
          </Button>
        </CardContent>
      </Card>
    </main>
  );
}

export default function VerifyPage() {
  return (
    <Suspense
      fallback={
        <main className="container relative z-10 mx-auto mt-12 flex justify-center">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </main>
      }
    >
      <VerifyInner />
    </Suspense>
  );
}
