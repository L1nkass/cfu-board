// Типы REST API CFU-Board (по backend/docs/openapi.yaml).

export type PostStatus = "pending" | "approved" | "rejected";

export interface Post {
  id: number;
  author_id: number;
  author_name: string;
  author_avatar?: string;
  title: string;
  content: string;
  image_url?: string;
  video_url?: string;
  likes_count: number;
  liked_by_me: boolean;
  comments_count: number;
  views_count: number;
  created_at: string;
  updated_at: string;
  status: PostStatus;
  rejection_reason?: string;
  moderated_by?: number;
  moderated_at?: string;
}

export interface LikeResponse {
  likes_count: number;
}

export interface PublicProfile {
  id: number;
  username: string;
  avatar_url?: string;
  is_verified: boolean;
  is_admin: boolean;
  created_at: string;
  posts_count: number;
}

export interface OwnProfile {
  id: number;
  username: string;
  avatar_url?: string;
  email: string;
  is_verified: boolean;
  is_admin: boolean;
  created_at: string;
  updated_at: string;
  posts_count: number;
}

export interface UserObject {
  id: number;
  username: string;
  email: string;
  is_verified: boolean;
}

export interface LoginResponse {
  message?: string;
  access_token: string;
  refresh_token: string;
  user: UserObject;
}

export interface RefreshResponse {
  access_token: string;
  refresh_token: string;
}

export interface SuccessResponse {
  message: string;
}

export interface ModerationResponse {
  status: string;
  message: string;
  post_id: number;
  action: "approved" | "rejected";
}

export interface Comment {
  id: number;
  post_id: number;
  author_id: number;
  author_name: string;
  author_avatar?: string;
  content: string;
  parent_id: number | null;
  replies: Comment[];
  likes_count: number;
  liked_by_me: boolean;
  created_at: string;
  updated_at: string;
}
