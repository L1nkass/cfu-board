"use client";

import { useQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { useParams } from "next/navigation";
import { ProfileView } from "@/components/profile-view";
import { fetchUser, qk } from "@/lib/queries";

export default function UserPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params.id);

  const { data: profile, isLoading, isError } = useQuery({
    queryKey: qk.user(id),
    queryFn: () => fetchUser(id),
    enabled: Number.isFinite(id),
  });

  if (isLoading) {
    return (
      <main className="container relative z-10 mx-auto mt-16 flex justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </main>
    );
  }

  if (isError || !profile) {
    return (
      <main className="container relative z-10 mx-auto mt-16 text-center">
        <p className="text-lg text-muted-foreground">Пользователь не найден.</p>
      </main>
    );
  }

  return <ProfileView profile={profile} />;
}
