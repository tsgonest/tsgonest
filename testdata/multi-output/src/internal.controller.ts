function Controller(path: string): ClassDecorator {
  return (target) => target;
}
function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}

interface InternalDto {
  secret: string;
  debug: boolean;
}

/** @tag internal */
@Controller("internal")
export class InternalController {
  @Get()
  async status(): Promise<InternalDto> {
    return {} as InternalDto;
  }
}
