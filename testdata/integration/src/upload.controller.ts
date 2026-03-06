import { Body, Controller, Post, UseInterceptors } from "@nestjs/common";
import { FormDataBody, FormDataInterceptor } from "@tsgonest/runtime";
import multer from "multer";
import type { UploadDto, GalleryUploadDto } from "./dto";

function createMulter() {
  return multer({ storage: multer.memoryStorage() });
}

@Controller("upload")
export class UploadController {
  // Case 1: Single file + validated metadata fields
  @Post("single")
  @UseInterceptors(FormDataInterceptor)
  uploadSingle(
    @Body() @FormDataBody(() => createMulter()) body: UploadDto
  ): { fileName: string; title: string; category: number } {
    return {
      fileName: body.file instanceof File ? body.file.name : "unknown",
      title: body.title,
      category: body.category,
    };
  }

  // Case 2: Multiple files (File[])
  @Post("gallery")
  @UseInterceptors(FormDataInterceptor)
  uploadGallery(
    @Body() @FormDataBody(() => createMulter()) body: GalleryUploadDto
  ): { fileCount: number; albumName: string } {
    return {
      fileCount: Array.isArray(body.images) ? body.images.length : 0,
      albumName: body.albumName,
    };
  }

  // Case 3: Validation failure — bad metadata (empty title)
  @Post("validate")
  @UseInterceptors(FormDataInterceptor)
  uploadValidate(
    @Body() @FormDataBody(() => createMulter()) body: UploadDto
  ): { ok: boolean } {
    return { ok: true };
  }
}
