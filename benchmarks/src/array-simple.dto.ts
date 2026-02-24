export interface IHobby {
  name: string;
  /** @minimum 0 @maximum 10 */
  rank: number;
  body: string;
}

export interface IPerson {
  name: string;
  age: number;
  hobbies: IHobby[];
}
