// This file has a type error — noEmitOnError should prevent JS output
export function multiply(a: number, b: number): number {
  return a * b;
}

const result: number = multiply(1, "three");
