export interface IDepartment {
  id: number;
  code: string;
  name: string;
  sales: number;
  created_at: string;
  children: IDepartment[];
}
