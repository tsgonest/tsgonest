// Stub decorators for testing
function Controller(path: string): ClassDecorator {
  return (target) => target;
}
function Post(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Patch(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Param(name: string): ParameterDecorator {
  return () => {};
}
function Body(): ParameterDecorator {
  return () => {};
}

import type {
  SubmitResultDto,
  UpdatePromptDto,
  PatchSettingsDto,
} from "./types";

@Controller("prompts")
export class PromptController {
  // Bug 1: Body param contains boolean discriminated union
  @Post("results")
  async submitResult(@Body() body: SubmitResultDto): Promise<void> {
    console.log(body);
  }

  // Bug 2: @Body() is NOT the first parameter, AND it's destructured
  @Patch(":companyID")
  async setPrompt(
    @Param("companyID") companyID: string,
    @Body() { content }: UpdatePromptDto
  ): Promise<void> {
    console.log(companyID, content);
  }

  // Bug 2 variant: destructured with multiple fields
  @Patch(":orgId/settings")
  async patchSettings(
    @Param("orgId") orgId: string,
    @Body() { theme, fontSize }: PatchSettingsDto
  ): Promise<void> {
    console.log(orgId, theme, fontSize);
  }
}
