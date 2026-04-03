import { UserService } from "./service.js";

const svc = new UserService();
console.log(svc.greet("Test", "User"));
console.log("sum:", svc.sum(1, 2));
console.log("DEV_READY");
