export interface IPoint3D {
  x: number;
  y: number;
  z: number;
}

export interface IBox3D {
  scale: IPoint3D;
  position: IPoint3D;
  rotate: IPoint3D;
  pivot: IPoint3D;
}
