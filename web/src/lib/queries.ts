import { api } from "./api";
import type {
  Comment,
  LikeResponse,
  ModerationResponse,
  Post,
  PostLikeResponse,
  PublicProfile,
  SuccessResponse,
  OwnProfile,
} from "./types";

// Ключи кэшей TanStack Query.
export const qk = {
  feed: ["feed"] as const,
  post: (id: number) => ["post", id] as const,
  comments: (id: number) => ["comments", id] as const,
  me: ["me"] as const,
  user: (id: number) => ["user", id] as const,
  userPosts: (id: number) => ["userPosts", id] as const,
  pending: ["pending"] as const,
};

export const PAGE_SIZE = 12;

export function fetchFeed(offset: number, limit = PAGE_SIZE): Promise<Post[]> {
  return api<Post[]>(`/posts?limit=${limit}&offset=${offset}`);
}

export function fetchPost(id: number): Promise<Post> {
  return api<Post>(`/posts/${id}`);
}

export function fetchComments(postId: number): Promise<Comment[]> {
  return api<Comment[]>(`/posts/${postId}/comments?limit=100&offset=0`);
}

export function fetchMe(): Promise<OwnProfile> {
  return api<OwnProfile>("/users/me");
}

export function fetchUser(id: number): Promise<PublicProfile> {
  return api<PublicProfile>(`/users/${id}`);
}

export function fetchUserPosts(id: number): Promise<Post[]> {
  return api<Post[]>(`/users/${id}/posts?limit=100&offset=0`);
}

export function fetchPending(): Promise<Post[]> {
  return api<Post[]>("/admin/posts/pending?limit=100&offset=0");
}

export function likePost(id: number, on: boolean): Promise<PostLikeResponse> {
  return api<PostLikeResponse>(`/posts/${id}/likes`, {
    method: on ? "POST" : "DELETE",
  });
}

export function likeComment(
  postId: number,
  commentId: number,
  on: boolean,
): Promise<LikeResponse> {
  return api<LikeResponse>(`/posts/${postId}/comments/${commentId}/likes`, {
    method: on ? "POST" : "DELETE",
  });
}

export function deletePost(id: number): Promise<SuccessResponse> {
  return api<SuccessResponse>(`/posts/${id}`, { method: "DELETE" });
}

export function deleteComment(
  postId: number,
  commentId: number,
): Promise<SuccessResponse> {
  return api<SuccessResponse>(`/posts/${postId}/comments/${commentId}`, {
    method: "DELETE",
  });
}

export function createComment(
  postId: number,
  content: string,
  parentId?: number,
): Promise<Comment> {
  return api<Comment>(`/posts/${postId}/comments`, {
    method: "POST",
    json: { content, parent_id: parentId ?? null },
  });
}

export function moderate(
  id: number,
  action: "approve" | "reject",
  reason?: string,
): Promise<ModerationResponse> {
  return api<ModerationResponse>(`/admin/posts/${id}/${action}`, {
    method: "POST",
    json: reason ? { reason } : {},
  });
}
