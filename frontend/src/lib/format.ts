// Форматирование дат «5 минут назад» и абсолютных значений по-русски.

export function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString("ru-RU", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function timeAgo(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const diff = (Date.now() - d.getTime()) / 1000;
  if (diff < 60) return "только что";
  const rtf = new Intl.RelativeTimeFormat("ru", { numeric: "auto" });
  if (diff < 3600) return rtf.format(-Math.floor(diff / 60), "minute");
  if (diff < 86_400) return rtf.format(-Math.floor(diff / 3600), "hour");
  if (diff < 2_592_000) return rtf.format(-Math.floor(diff / 86_400), "day");
  return d.toLocaleDateString("ru-RU", { day: "numeric", month: "long" });
}
