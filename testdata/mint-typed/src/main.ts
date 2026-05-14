import { createApp, defineModule } from "@mintkit/core";
import { UsersController } from "./users.controller";
import { registerUsersController } from "./users.controller.UsersController.tsgonest.js";

const app = await createApp({
  imports: [defineModule({ controllers: [UsersController] })],
});

registerUsersController(app);

const server = Bun.serve({
  port: Number(process.env.PORT ?? 0),
  fetch: (req) => app.fetch(req),
});

process.stdout.write(`LISTENING:${server.url}\n`);
