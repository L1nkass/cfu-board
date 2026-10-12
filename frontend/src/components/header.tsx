"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { LogOut, ShieldCheck, User as UserIcon } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAuth } from "@/lib/auth";
import { cn } from "@/lib/utils";

type Theme = "dark" | "light";

function readTheme(): Theme {
  if (typeof window === "undefined") return "dark";
  return (localStorage.getItem("cfu_theme") as Theme) ?? "dark";
}

function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>("dark");
  useEffect(() => setTheme(readTheme()), []);

  const toggle = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    setTheme(next);
    localStorage.setItem("cfu_theme", next);
    document.documentElement.classList.toggle("dark", next === "dark");
  };

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label="Переключить тему"
      className="group flex cursor-pointer items-center justify-center bg-transparent p-0 outline-none"
    >
      {theme === "dark" ? (
        // Оригинальная луна из style.css/index.html (moon-svg).
        <svg
          className="moon-svg h-[42px] w-[42px] fill-white opacity-80 transition-all duration-[400ms] [transition-timing-function:cubic-bezier(0.25,1,0.5,1)] group-hover:scale-[1.12] group-hover:fill-[#ffe066] group-hover:opacity-100 group-hover:drop-shadow-[0_0_10px_rgba(255,224,102,0.85)]"
          viewBox="0 0 1534.87 1814.73"
          aria-hidden
        >
          <path d="m150.56,516.7c15.97,100.09,73.34,186.29,153.88,240.68-79.78,55.49-135.96,142.47-150.56,242.78-15.97-100.09-73.34-186.29-153.88-240.68,79.78-55.49,135.96-142.47,150.56-242.78Z" />
          <path d="m1374.37,836.63c0,400.56-324.72,725.28-725.28,725.28-261.55,0-490.78-138.45-618.36-346.12,92.8,76.34,211.59,122.09,341.13,122.09,296.94,0,537.66-240.71,537.66-537.66S668.8,262.57,371.86,262.57c-79.08,0-154.19,17.08-221.78,47.72,129.99-123.27,305.68-198.93,499.01-198.93,400.56,0,725.28,324.72,725.28,725.28Z" />
          <path d="m1372.47,1304.5c16.85,105.64,77.41,196.61,162.4,254.01-84.2,58.57-143.49,150.36-158.89,256.22-16.85-105.64-77.41-196.61-162.4-254.01,84.2-58.57,143.49-150.36,158.89-256.22Z" />
          <path d="m1370.54,0c7.18,44.98,32.96,83.71,69.14,108.15-35.85,24.94-61.09,64.02-67.65,109.09-7.18-44.98-32.96-83.71-69.14-108.15,35.85-24.94,61.09-64.02,67.65-109.09Z" />
        </svg>
      ) : (
        <svg
          className="h-[42px] w-[42px] fill-[#f5a623] transition-all duration-[400ms] group-hover:scale-[1.12] group-hover:drop-shadow-[0_0_10px_rgba(255,224,102,0.85)]"
          viewBox="0 0 24 24"
          aria-hidden
        >
          <circle cx="12" cy="12" r="5" />
          {Array.from({ length: 8 }).map((_, i) => (
            <rect
              key={i}
              x="11.25"
              y="1.5"
              width="1.5"
              height="4"
              rx="0.75"
              transform={`rotate(${i * 45} 12 12)`}
            />
          ))}
        </svg>
      )}
    </button>
  );
}

export function SiteHeader() {
  const { user, loading, logout } = useAuth();
  const pathname = usePathname();
  const router = useRouter();

  const handleLogout = async () => {
    await logout();
    router.push("/");
  };

  const navLink = (href: string, label: string) => (
    <Link
      href={href}
      className={cn(
        "rounded-md px-3 py-2 text-sm font-medium text-foreground/70 transition-colors hover:text-[#ffe066]",
        pathname === href && "bg-secondary text-foreground",
      )}
    >
      {label}
    </Link>
  );

  return (
    <header className="sticky top-0 z-50 flex h-[80px] w-full items-center justify-between border-b-4 border-[#272e3e] bg-card px-5 shadow-[0_22px_27px_rgba(0,0,0,0.5)]">
      {/* Слева — тема и название (в макете тема стояла первой слева). */}
      <div className="flex items-center gap-4">
        <ThemeToggle />
        <Link
          href="/"
          className="text-xl font-bold tracking-wide text-foreground transition-colors hover:text-[#ffe066]"
        >
          Микроволнивка
        </Link>
      </div>

      {/* Справа — навигация и профиль (в макете справа). */}
      <div className="flex items-center gap-2">
        {navLink("/", "Лента")}
        {user?.is_admin && navLink("/admin", "Модерация")}
        {navLink("/new", "Написать")}

        {!loading &&
          (user ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  aria-label="Профиль"
                  className="ml-1 cursor-pointer rounded-full border-4 border-[#272e3e] bg-secondary outline-none transition-transform hover:scale-105"
                >
                  <Avatar className="h-[52px] w-[52px]">
                    <AvatarImage
                      src={user.avatar_url || undefined}
                      alt={user.username}
                    />
                    <AvatarFallback className="bg-[#272e3e] text-lg text-foreground">
                      {user.username.slice(0, 1).toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-64">
                <DropdownMenuLabel>
                  <div className="flex flex-col">
                    <span>{user.username}</span>
                    <span className="text-xs font-normal text-muted-foreground">
                      {user.email}
                    </span>
                  </div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => router.push("/profile")}>
                  <UserIcon /> Мой профиль
                </DropdownMenuItem>
                {user.is_admin && (
                  <DropdownMenuItem onClick={() => router.push("/admin")}>
                    <ShieldCheck /> Модерация
                  </DropdownMenuItem>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={handleLogout}>
                  <LogOut /> Выйти
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <div className="ml-1 flex items-center gap-2">
              <Button
                asChild
                variant="secondary"
                className="border border-[#323b4f] bg-[#272e3e] text-foreground hover:bg-[#323b4f]"
              >
                <Link href="/login">Войти</Link>
              </Button>
              <Button asChild className="bg-[#ffe066] text-[#141722] hover:bg-[#ffd43b]">
                <Link href="/register">Регистрация</Link>
              </Button>
            </div>
          ))}
      </div>
    </header>
  );
}
