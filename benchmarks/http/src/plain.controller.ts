import { Controller, Get, Post, Body, Param } from '@nestjs/common';

/**
 * Plain controller WITHOUT tsgonest transforms.
 * Uses native JSON.stringify for serialization, no validation.
 * Used to isolate framework overhead from tsgonest's contribution.
 */

interface PlainUserDto {
  id: string;
  name: string;
  email: string;
  age: number;
  isActive: boolean;
  role: 'admin' | 'user' | 'moderator';
  createdAt: string;
}

const USERS: PlainUserDto[] = Array.from({ length: 20 }, (_, i) => ({
  id: `550e8400-e29b-41d4-a716-44665544${String(i).padStart(4, '0')}`,
  name: `User ${i}`,
  email: `user${i}@example.com`,
  age: 20 + i,
  isActive: true,
  role: 'user' as const,
  createdAt: new Date(2024, 0, i + 1).toISOString(),
}));

@Controller('plain')
export class PlainController {
  @Get()
  findAll(): PlainUserDto[] {
    return USERS;
  }

  @Post()
  create(@Body() dto: any): PlainUserDto {
    return {
      id: '550e8400-e29b-41d4-a716-446655440099',
      ...dto,
      createdAt: new Date().toISOString(),
    };
  }

  @Get(':id')
  findOne(@Param('id') id: string): PlainUserDto {
    return USERS[0];
  }
}
