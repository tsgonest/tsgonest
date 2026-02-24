export interface PaginationQuery {
  /** @minimum 1 */
  page: number;
  /** @minimum 1 @maximum 100 */
  limit: number;
  ascending?: boolean;
}

export interface OrderResponse {
  id: number;
  total: number;
  status: string;
}
