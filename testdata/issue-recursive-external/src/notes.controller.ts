// Stub decorators — mirror the pattern used by other tsgonest fixtures.
function Controller(path: string): ClassDecorator {
  return (target) => target;
}
function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Post(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Body(): ParameterDecorator {
  return () => {};
}

import type { NotesResponse, UpdateNotesDTO } from "./notes.dto";

@Controller("notes")
export class NotesController {
  @Get()
  async getNotes(): Promise<NotesResponse> {
    return {
      notes: {
        type: "doc",
        content: [
          { type: "paragraph", content: [{ type: "text", text: "hi" }] },
        ],
      },
    };
  }

  @Post()
  async updateNotes(@Body() body: UpdateNotesDTO): Promise<UpdateNotesDTO> {
    return body;
  }
}
