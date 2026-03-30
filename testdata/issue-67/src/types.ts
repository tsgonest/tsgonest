// Bug 1: Boolean discriminated union as a property of a DTO
export interface SuccessResponse {
  ok: true;
  data: string;
}

export interface ErrorResponse {
  ok: false;
  error: string;
}

export type ApiResult = SuccessResponse | ErrorResponse;

// Wrapper DTO that contains the boolean-discriminated union
export interface SubmitResultDto {
  result: ApiResult;
}

// DTO for Bug 2: destructured body param
export interface UpdatePromptDto {
  content: string;
}

// DTO with optional fields for destructured-with-defaults test
export interface PatchSettingsDto {
  theme?: string;
  fontSize?: number;
}
