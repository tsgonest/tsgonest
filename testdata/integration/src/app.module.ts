import { Module } from "@nestjs/common";
import { SseController } from "./sse.controller";
import {
  SseAutoController,
  SseExtraController,
} from "./sse-no-companion.controller";
import { UploadController } from "./upload.controller";

@Module({
  controllers: [
    SseController,
    SseAutoController,
    SseExtraController,
    UploadController,
  ],
})
export class AppModule {}
