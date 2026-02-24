export type IBucket = IDirectory | ISharedDirectory | IImageFile | ITextFile | IZipFile;

export interface IDirectory {
  type: "directory";
  id: number;
  name: string;
  path: string;
  children: IBucket[];
}

export interface ISharedDirectory {
  type: "shared-directory";
  id: number;
  name: string;
  path: string;
  access: "read" | "write";
  children: IBucket[];
}

export interface IImageFile {
  type: "image";
  id: number;
  name: string;
  path: string;
  width: number;
  height: number;
  url: string;
  size: number;
}

export interface ITextFile {
  type: "text";
  id: number;
  name: string;
  path: string;
  size: number;
  content: string;
  encoding: string;
}

export interface IZipFile {
  type: "zip";
  id: number;
  name: string;
  path: string;
  size: number;
  count: number;
}
