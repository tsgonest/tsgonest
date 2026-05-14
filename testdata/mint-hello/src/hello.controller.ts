import { Controller, Get } from "@mintkit/core";

@Controller("/hello")
export class HelloController {
  @Get()
  hello(): string {
    return "Hello from Mint!";
  }
}
