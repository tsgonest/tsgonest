export interface ICategory {
  id: number;
  code: string;
  name: string;
  sequence: number;
  children: ICategory[];
}
