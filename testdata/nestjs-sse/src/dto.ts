export interface UserDto {
  id: number;
  /** @minLength 1 */
  name: string;
  /** @format email */
  email: string;
}

export interface NotificationDto {
  id: string;
  message: string;
  /** @format date-time */
  timestamp: string;
}

export interface DeletePayload {
  id: string;
  /** @format date-time */
  deletedAt: string;
}
