export type IShape =
  | IPoint
  | ILine
  | ITriangle
  | IRectangle
  | IPolyline
  | IPolygon
  | ICircle;

export interface IPoint {
  type: "point";
  x: number;
  y: number;
}

export interface ILine {
  type: "line";
  p1: IPoint;
  p2: IPoint;
}

export interface ITriangle {
  type: "triangle";
  p1: IPoint;
  p2: IPoint;
  p3: IPoint;
}

export interface IRectangle {
  type: "rectangle";
  p1: IPoint;
  p2: IPoint;
  p3: IPoint;
  p4: IPoint;
}

export interface IPolyline {
  type: "polyline";
  points: IPoint[];
}

export interface IPolygon {
  type: "polygon";
  outer: IPolyline;
  inner: IPolyline[];
}

export interface ICircle {
  type: "circle";
  center: IPoint;
  radius: number;
}
