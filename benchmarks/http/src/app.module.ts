import { Module } from '@nestjs/common';
import { BenchmarkController } from './benchmark.controller';
import { PlainController } from './plain.controller';

@Module({
  controllers: [BenchmarkController, PlainController],
})
export class AppModule {}
