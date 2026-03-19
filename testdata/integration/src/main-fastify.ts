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

  // Register a raw content-type parser for multipart/form-data so Fastify
  // doesn't reject it with 415. The actual parsing is done by
  // FormDataInterceptor (via multer on the raw IncomingMessage stream).
  const fastifyInstance = app.getHttpAdapter().getInstance();
  fastifyInstance.addContentTypeParser(
    "multipart/form-data",
    (_req: any, _payload: any, done: any) => done(null)
  );

  const port = parseInt(process.env.PORT || "0", 10);
  await app.listen(port, "127.0.0.1");
  const url = await app.getUrl();
  process.stdout.write(`LISTENING:${url}\n`);
}
bootstrap();
