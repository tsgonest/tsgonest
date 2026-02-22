import type { PaginationQuery } from "../common/types";

/** Article status */
export enum ArticleStatus {
  DRAFT = "draft",
  PUBLISHED = "published",
  ARCHIVED = "archived",
}

/** A comment on an article — recursive (replies contain comments) */
export interface Comment {
  id: number;
  authorId: number;
  authorName: string;
  /** @maxLength 5000 */
  content: string;
  createdAt: string;
  /** Nested replies */
  replies: Comment[];
}

/** Create article request */
export interface CreateArticleDto {
  /**
   * @minLength 5
   * @maxLength 200
   */
  title: string;
  /** @maxLength 50000 */
  content: string;
  /** @minItems 1 @maxItems 10 */
  tags: string[];
  status: ArticleStatus;
  /** ISO date string for scheduled publishing */
  publishAt?: string | null;
}

/** Update article — partial of create */
export type UpdateArticleDto = Partial<CreateArticleDto>;

/** Article response with nested comments */
export interface ArticleResponse {
  id: number;
  title: string;
  /** @maxLength 300 */
  slug: string;
  content: string;
  tags: string[];
  status: ArticleStatus;
  authorId: number;
  authorName: string;
  comments: Comment[];
  publishAt: string | null;
  createdAt: string;
  updatedAt: string;
}

/** Article search/filter query */
export interface ArticleListQuery extends PaginationQuery {
  status?: ArticleStatus;
  tag?: string;
  authorId?: number;
  search?: string;
}

/** Create comment request */
export interface CreateCommentDto {
  /** @maxLength 5000 */
  content: string;
  /** If replying to a comment */
  parentCommentId?: number;
}
