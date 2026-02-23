export interface CreateItemDto {
  /** @minLength 1 */
  name: string;
  /** @minimum 0 */
  price: number;
}

export interface ItemResponse {
  id: number;
  name: string;
  price: number;
}
