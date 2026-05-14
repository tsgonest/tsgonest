import { Body, Controller, Post } from "@mintkit/core";
import type { AvatarUpload, LargeUpload } from "./upload.dto";

interface UploadResponse {
  ok: true;
  name: string;
  type: string;
  size: number;
}

interface StreamResponse {
  ok: true;
  filename: string;
  type: string;
  bytes: number;
}

@Controller("/uploads")
export class UploadController {
  @Post("/avatar")
  async avatar(@Body() data: AvatarUpload): Promise<UploadResponse> {
    return {
      ok: true,
      name: data.title,
      type: data.image.type,
      size: data.image.size,
    };
  }

  @Post("/large")
  async large(@Body() data: LargeUpload): Promise<StreamResponse> {
    // Drain the stream, counting bytes (representative of a write-to-disk handler).
    const reader = data.file.stream.getReader();
    let total = 0;
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      if (value) total += value.length;
    }
    return {
      ok: true,
      filename: data.filename,
      type: data.file.type,
      bytes: total,
    };
  }
}
