import "reflect-metadata";
import { NestFactory } from "@nestjs/core";
import {
  FastifyAdapter,
  NestFastifyApplication,
} from "@nestjs/platform-fastify";
import { AppModule } from "./app.module";

async function bootstrap() {
  const app = await NestFactory.create<NestFastifyApplication>(
    AppModule,
    new FastifyAdapter(),
    { logger: false }
  );
  const port = parseInt(process.env.PORT || "0", 10);
  await app.listen(port, "127.0.0.1");
  const url = await app.getUrl();
  process.stdout.write(`LISTENING:${url}\n`);
}
bootstrap();
