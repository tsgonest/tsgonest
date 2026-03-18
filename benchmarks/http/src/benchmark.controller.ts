import { Controller, Get, Post, Body, Param } from '@nestjs/common';
import type { CreateUserDto, UserDto } from './benchmark.dto';

// Seed data: 20 users for the list endpoint
const USERS: UserDto[] = Array.from({ length: 20 }, (_, i) => ({
  id: `550e8400-e29b-41d4-a716-44665544${String(i).padStart(4, '0')}`,
  name: `User ${i}`,
  email: `user${i}@example.com`,
  age: 20 + i,
  isActive: true,
  role: 'user' as const,
  createdAt: new Date(2024, 0, i + 1).toISOString(),
}));

@Controller('users')
export class BenchmarkController {
  @Get()
  findAll(): UserDto[] {
    return USERS;
  }

  @Post()
  create(@Body() dto: CreateUserDto): UserDto {
    return {
      id: '550e8400-e29b-41d4-a716-446655440099',
      ...dto,
      createdAt: new Date().toISOString(),
    };
  }

  @Get(':id')
  findOne(@Param('id') id: string): UserDto {
    return USERS[0];
  }
}
