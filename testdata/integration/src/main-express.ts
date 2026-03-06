import "reflect-metadata";
import { NestFactory } from "@nestjs/core";
import { AppModule } from "./app.module";

async function bootstrap() {
  const app = await NestFactory.create(AppModule, { logger: false });
  const port = parseInt(process.env.PORT || "0", 10);
  await app.listen(port);
  const url = await app.getUrl();
  // Print the URL so the test harness can read it
  process.stdout.write(`LISTENING:${url}\n`);
}
bootstrap();
