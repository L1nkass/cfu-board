import type { Metadata } from "next";
import { Doodles } from "@/components/doodles";
import { SiteHeader } from "@/components/header";
import { Providers } from "@/components/providers";
import "./globals.css";

export const metadata: Metadata = {
  title: "Микроволнивка — CFU-Board",
  description: "Социальная сеть CFU-Board: лента, публикации, общение.",
  icons: [{ url: "/images/favicon.jpg", type: "image/jpeg" }],
};

// Восстановление темы до первой отрисовки (анти-мигание).
const themeScript = `(function(){try{var t=localStorage.getItem("cfu_theme");if(t==="light")document.documentElement.classList.remove("dark");else document.documentElement.classList.add("dark");}catch(e){}})();`;

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru" className="dark" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body className="min-h-screen antialiased">
        <Providers>
          <Doodles />
          <SiteHeader />
          <div className="relative z-10">{children}</div>
        </Providers>
      </body>
    </html>
  );
}
