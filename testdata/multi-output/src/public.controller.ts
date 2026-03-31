function Controller(path: string): ClassDecorator {
  return (target) => target;
}
function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}

interface PublicDto {
  id: string;
  name: string;
}

/** @tag public */
@Controller("public")
export class PublicController {
  @Get()
  async list(): Promise<PublicDto[]> {
    return [];
  }
}
