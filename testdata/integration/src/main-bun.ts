import "reflect-metadata";
import { NestFactory } from "@nestjs/core";
import { BunAdapter } from "@tsgonest/platform-bun";
import { AppModule } from "./app.module";

async function bootstrap() {
  const app = await NestFactory.create(AppModule, new BunAdapter(), {
    logger: false,
  });
  const port = parseInt(process.env.PORT || "0", 10);
  await app.listen(port);
  const url = await app.getUrl();
  process.stdout.write(`LISTENING:${url}\n`);
}
bootstrap();
