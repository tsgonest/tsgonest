// Mock NestJS decorators for testing
function Controller(path: string): ClassDecorator {
  return (target) => target;
}
function Get(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Post(path?: string): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}
function Param(name: string): ParameterDecorator {
  return () => {};
}
function Res(): ParameterDecorator {
  return () => {};
}
function HttpCode(code: number): MethodDecorator {
  return (target, key, descriptor) => descriptor;
}

// Import @Returns from @tsgonest/runtime (resolved via ambient declare module in tsgonest-runtime.d.ts)
import { Returns } from "@tsgonest/runtime";

import type { ReportResponse, ReportSummary } from "./report.dto";

@Controller("reports")
export class ReportController {
  // Case 1: @Res() + @Returns<T>() — typed JSON response via raw response
  @Returns<ReportResponse>()
  @Get(":id")
  async getReport(@Param("id") id: string, @Res() res: any): Promise<void> {
    // In reality: res.json(report)
  }

  // Case 2: @Res() + @Returns<Uint8Array>({ contentType: 'application/pdf' }) — binary response
  @Returns<Uint8Array>({ contentType: "application/pdf", description: "PDF report document" })
  @Get(":id/pdf")
  async getReportPdf(@Param("id") id: string, @Res() res: any): Promise<void> {
    // In reality: res.header('Content-Type', 'application/pdf'); res.send(pdfBuffer)
  }

  // Case 3: @Res() + @Returns<string>({ contentType: 'text/csv' }) — text response
  @Returns<string>({ contentType: "text/csv", description: "CSV export" })
  @Get(":id/csv")
  async getReportCsv(@Param("id") id: string, @Res() res: any): Promise<void> {
    // In reality: res.header('Content-Type', 'text/csv'); res.send(csvString)
  }

  // Case 4: @Res() WITHOUT @Returns — should produce void + warning
  @Get(":id/raw")
  async getRawReport(@Param("id") id: string, @Res() res: any): Promise<void> {
    // No @Returns — warning expected
  }

  // Case 5: @Res() with @tsgonest-ignore — suppresses warning
  /**
   * Stream raw bytes to client.
   * @tsgonest-ignore uses-raw-response
   */
  @Get(":id/stream")
  async streamReport(@Param("id") id: string, @Res() res: any): Promise<void> {
    // @tsgonest-ignore suppresses the uses-raw-response warning
  }

  // Case 6: Normal route (no @Res) — should work as before
  @Get("summary")
  async getSummary(): Promise<ReportSummary> {
    return { totalReports: 0, lastGenerated: "" };
  }

  // Case 7: @Returns with status override
  @Returns<ReportResponse>({ status: 200 })
  @Post(":id/regenerate")
  @HttpCode(202)
  async regenerateReport(@Param("id") id: string, @Res() res: any): Promise<void> {
    // @Returns status override: 200 instead of HttpCode 202
  }
}
