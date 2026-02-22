import {
  Controller,
  Post,
  Body,
  HttpCode,
} from "../common/decorators";
import type {
  LoginDto,
  RegisterDto,
  AuthTokenResponse,
  RefreshTokenDto,
} from "./auth.dto";

/**
 * Authentication controller
 * @tag Auth
 */
@Controller("auth")
export class AuthController {
  /**
   * Log in with email and password
   * @summary User login
   */
  @Post("login")
  @HttpCode(200)
  async login(@Body() body: LoginDto): Promise<AuthTokenResponse> {
    return {} as AuthTokenResponse;
  }

  /**
   * Register a new user account
   * @summary User registration
   */
  @Post("register")
  async register(@Body() body: RegisterDto): Promise<AuthTokenResponse> {
    return {} as AuthTokenResponse;
  }

  /**
   * Refresh an expired access token
   * @summary Token refresh
   */
  @Post("refresh")
  @HttpCode(200)
  async refresh(@Body() body: RefreshTokenDto): Promise<AuthTokenResponse> {
    return {} as AuthTokenResponse;
  }
}
