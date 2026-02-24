export interface ICustomer {
  id: number;
  channel: IChannel;
  member: IMember | null;
  account: IAccount | null;
}

export interface IChannel {
  id: number;
  code: string;
  name: string;
  sequence: number;
  exclusive: boolean;
  priority: number;
}

export interface IMember {
  id: number;
  account: IAccount;
  name: string;
  age: number;
  sex: "male" | "female" | "other";
  deceased: boolean;
}

export interface IAccount {
  id: number;
  code: string;
  balance: number;
}
