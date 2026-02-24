import { ApiTags, ApiBearerAuth, ApiOkResponse } from '@nestjs/swagger';
import { Controller, Get, Post, Body, UseGuards } from '@nestjs/common';
import { CreateUserDto } from './user.dto';

@ApiTags('Users')
@ApiBearerAuth()
@Controller('users')
export class UsersController {
  @ApiOkResponse({ type: String })
  @Get()
  findAll() {
    return [];
  }

  @Post()
  create(@Body() dto: CreateUserDto) {
    return dto;
  }
}
