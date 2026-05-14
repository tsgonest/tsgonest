import { tags } from "@tsgonest/types";

export interface AvatarUpload {
  title: string & tags.MinLength<1>;
  image: File & tags.MaxSize<10_000> & tags.MimeTypes<"image/png" | "image/jpeg">;
}

export interface LargeUpload {
  filename: string & tags.MinLength<1>;
  file: tags.FileStream & tags.MaxSize<5_000_000>;
}
