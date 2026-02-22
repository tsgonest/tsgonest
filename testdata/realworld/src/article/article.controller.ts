import {
  Controller,
  Get,
  Post,
  Put,
  Delete,
  Param,
  Body,
  Query,
  HttpCode,
} from "../common/decorators";
import type {
  CreateArticleDto,
  UpdateArticleDto,
  ArticleResponse,
  ArticleListQuery,
  CreateCommentDto,
  Comment,
} from "./article.dto";
import type { PaginatedResponse } from "../common/types";

/**
 * Article management controller
 * @tag Articles
 */
@Controller("articles")
export class ArticleController {
  /**
   * List articles with pagination and filters
   * @summary List articles
   */
  @Get()
  async findAll(
    @Query() query: ArticleListQuery
  ): Promise<PaginatedResponse<ArticleResponse>> {
    return {} as PaginatedResponse<ArticleResponse>;
  }

  /**
   * Get a single article by slug
   * @summary Get article
   */
  @Get(":slug")
  async findOne(@Param("slug") slug: string): Promise<ArticleResponse> {
    return {} as ArticleResponse;
  }

  /**
   * Create a new article
   * @summary Create article
   */
  @Post()
  async create(@Body() body: CreateArticleDto): Promise<ArticleResponse> {
    return {} as ArticleResponse;
  }

  /**
   * Update an existing article
   * @summary Update article
   */
  @Put(":id")
  async update(
    @Param("id") id: string,
    @Body() body: UpdateArticleDto
  ): Promise<ArticleResponse> {
    return {} as ArticleResponse;
  }

  /**
   * Delete an article
   * @summary Delete article
   */
  @Delete(":id")
  @HttpCode(204)
  async remove(@Param("id") id: string): Promise<void> {}

  /**
   * Add a comment to an article
   * @summary Add comment
   */
  @Post(":id/comments")
  async addComment(
    @Param("id") id: string,
    @Body() body: CreateCommentDto
  ): Promise<Comment> {
    return {} as Comment;
  }

  /**
   * Get comments for an article
   * @summary List comments
   */
  @Get(":id/comments")
  async getComments(@Param("id") id: string): Promise<Comment[]> {
    return [];
  }
}
