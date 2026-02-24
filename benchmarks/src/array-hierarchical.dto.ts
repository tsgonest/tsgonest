export interface IEmployee {
  id: number;
  name: string;
  age: number;
  grade: number;
}

export interface IHierarchyDepartment {
  id: number;
  code: string;
  name: string;
  sales: number;
  employees: IEmployee[];
}

export interface ICompany {
  id: number;
  serial: number;
  name: string;
  established_at: string;
  departments: IHierarchyDepartment[];
}
