import { z } from "zod";

// Правила валидации зеркалят бэкенд (см. backend/docs/openapi.yaml).

export const loginSchema = z.object({
  login: z
    .string()
    .min(1, "Укажите имя пользователя или email")
    .max(254, "Слишком длинный логин"),
  password: z.string().min(1, "Укажите пароль"),
});

export const registerSchema = z
  .object({
    username: z
      .string()
      .min(3, "Минимум 3 символа")
      .max(30, "Максимум 30 символов")
      .regex(/^[a-zA-Z0-9_-]+$/, "Латиница, цифры, '_' и '-'"),
    email: z.email("Некорректный email").max(254, "Слишком длинный email"),
    password: z
      .string()
      .min(8, "Минимум 8 символов")
      .max(72, "Максимум 72 символов"),
    confirm: z.string(),
  })
  .refine((v) => v.password === v.confirm, {
    message: "Пароли не совпадают",
    path: ["confirm"],
  });

export const resendSchema = z.object({
  email: z.email("Некорректный email"),
});

export const postSchema = z.object({
  title: z
    .string()
    .min(1, "Укажите заголовок")
    .max(255, "Максимум 255 символов"),
  content: z.string().min(1, "Напишите текст публикации"),
});

export const commentSchema = z.object({
  content: z
    .string()
    .min(1, "Комментарий не может быть пустым")
    .max(2000, "Максимум 2000 символов"),
});

export const rejectSchema = z.object({
  reason: z
    .string()
    .max(1000, "Максимум 1000 символов")
    .optional()
    .or(z.literal("")),
});

export type LoginValues = z.infer<typeof loginSchema>;
export type RegisterValues = z.infer<typeof registerSchema>;
export type ResendValues = z.infer<typeof resendSchema>;
export type PostValues = z.infer<typeof postSchema>;
export type CommentValues = z.infer<typeof commentSchema>;
export type RejectValues = z.infer<typeof rejectSchema>;

// Допустимые типы медиа (проверяется и содержимое на бэкенде).
export const IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp"];
export const VIDEO_TYPES = ["video/mp4", "video/quicktime", "video/x-msvideo"];
