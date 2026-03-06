// SSE event payloads
export interface UserEvent {
  id: number;
  /** @minLength 1 */
  name: string;
  action: string;
}

export interface NotificationEvent {
  message: string;
  level: "info" | "warn" | "error";
}

// File upload DTO
export interface UploadDto {
  file: File;
  /** @minLength 1 */
  title: string;
  /** @minimum 1 */
  category: number;
}

// Multi-file upload DTO
export interface GalleryUploadDto {
  images: File[];
  /** @minLength 1 */
  albumName: string;
}
