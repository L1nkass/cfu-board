import type { Post } from "@/lib/types";

// Карточка поста в стиле исходного макета: SVG-рамка (скруглённый rect +
// линия разделителя на 70% высоты), медиа сверху, аватар и текст снизу.

export function PostFrame() {
  return (
    <svg
      className="pointer-events-none absolute inset-0 z-20 h-full w-full"
      viewBox="0 0 300 200"
      preserveAspectRatio="none"
      aria-hidden
    >
      <rect
        x="2"
        y="2"
        width="296"
        height="196"
        rx="16"
        className="fill-none stroke-[#272e3e] dark:stroke-[#272e3e]"
        strokeWidth="3"
      />
      <line
        x1="2"
        y1="135"
        x2="298"
        y2="135"
        className="stroke-[#272e3e]"
        strokeWidth="3"
      />
    </svg>
  );
}
