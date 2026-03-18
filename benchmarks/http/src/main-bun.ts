import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
// @ts-ignore — resolved at runtime from workspace
import { BunAdapter } from '@tsgonest/platform-bun';
import { AppModule } from './app.module';

async function bootstrap() {
  const app = await NestFactory.create(AppModule, new BunAdapter(), { logger: false });
  await app.listen(3000);
  process.stdout.write('LISTENING\n');
}
bootstrap();
