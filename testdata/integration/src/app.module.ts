import { Module } from "@nestjs/common";
import { SseController } from "./sse.controller";
import { UploadController } from "./upload.controller";

@Module({
  controllers: [SseController, UploadController],
})
export class AppModule {}
