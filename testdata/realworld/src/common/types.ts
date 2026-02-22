// Common shared types used across controllers

/** Generic paginated response wrapper */
export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
}

/** Standard API error response */
export interface ApiError {
  statusCode: number;
  message: string;
  error: string;
  details?: Record<string, string[]>;
}

/** Sort direction enum */
export enum SortOrder {
  ASC = "asc",
  DESC = "desc",
}

/** Pagination query parameters */
export interface PaginationQuery {
  /** @minimum 1 */
  page?: number;
  /** @minimum 1 @maximum 100 */
  limit?: number;
  sortBy?: string;
  sortOrder?: SortOrder;
}

/** Geographic coordinates */
export interface GeoLocation {
  /** @minimum -90 @maximum 90 */
  latitude: number;
  /** @minimum -180 @maximum 180 */
  longitude: number;
}

/** Physical address */
export interface Address {
  street: string;
  city: string;
  state?: string;
  country: string;
  /** @pattern ^[0-9]{5}(-[0-9]{4})?$ */
  zipCode: string;
  geo?: GeoLocation;
}
