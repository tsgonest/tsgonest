import { createApp, defineModule } from "@mintkit/core";
import { UploadController } from "./upload.controller";
import { registerUploadController } from "./upload.controller.UploadController.tsgonest.js";

const app = await createApp({
  imports: [defineModule({ controllers: [UploadController] })],
});

registerUploadController(app);

const server = Bun.serve({
  port: Number(process.env.PORT ?? 0),
  fetch: (req) => app.fetch(req),
});

process.stdout.write(`LISTENING:${server.url}\n`);
