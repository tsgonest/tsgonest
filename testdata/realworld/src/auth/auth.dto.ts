/** Login request DTO */
export interface LoginDto {
  /** @format email */
  email: string;
  /**
   * @minLength 8
   * @maxLength 128
   */
  password: string;
}

/** Registration request DTO */
export interface RegisterDto {
  /**
   * @minLength 2
   * @maxLength 50
   */
  username: string;
  /** @format email */
  email: string;
  /**
   * @minLength 8
   * @maxLength 128
   */
  password: string;
  /**
   * @minLength 8
   * @maxLength 128
   */
  confirmPassword: string;
}

/** JWT token pair response */
export interface AuthTokenResponse {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
  tokenType: "Bearer";
}

/** Refresh token request */
export interface RefreshTokenDto {
  refreshToken: string;
}
