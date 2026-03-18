import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';

async function bootstrap() {
  const app = await NestFactory.create(AppModule, { logger: false });
  await app.listen(3000);
  process.stdout.write('LISTENING\n');
}
bootstrap();
