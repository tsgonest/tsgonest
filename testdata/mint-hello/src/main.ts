import { createApp, defineModule } from "@mintkit/core";
import { HelloController } from "./hello.controller";
import { registerHelloController } from "./hello.controller.HelloController.tsgonest.js";

const app = await createApp({
  imports: [defineModule({ controllers: [HelloController] })],
});

registerHelloController(app);

const server = Bun.serve({
  port: Number(process.env.PORT ?? 0),
  fetch: (req) => app.fetch(req),
});

process.stdout.write(`LISTENING:${server.url}\n`);
