import { describe, it, expect, beforeAll } from "vitest";
import { existsSync, readFileSync, rmSync } from "fs";
import { resolve } from "path";
import { runTsgonest, FIXTURES_DIR } from "./helpers";

describe("@Returns<T>() decorator", () => {
  const fixtureDir = resolve(FIXTURES_DIR, "nestjs-returns");
  const openapiFile = resolve(fixtureDir, "dist/openapi.json");

  beforeAll(() => {
    const distDir = resolve(fixtureDir, "dist");
    if (existsSync(distDir)) {
      rmSync(distDir, { recursive: true });
    }
    const cacheFile = resolve(fixtureDir, "tsconfig.tsgonest-cache");
    if (existsSync(cacheFile)) rmSync(cacheFile);
    const buildInfoFile = resolve(fixtureDir, "tsconfig.tsbuildinfo");
    if (existsSync(buildInfoFile)) rmSync(buildInfoFile);

    const { exitCode, stderr } = runTsgonest([
      "--project",
      "testdata/nestjs-returns/tsconfig.json",
      "--config",
      "testdata/nestjs-returns/tsgonest.config.json",
      "--clean",
    ]);
    expect(exitCode).toBe(0);
    expect(stderr).toContain("controller");
    expect(stderr).toContain("OpenAPI");
    expect(existsSync(openapiFile)).toBe(true);
  });

  it("should generate OpenAPI for @Returns<ReportResponse>() with @Res()", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const getReport = doc.paths["/reports/{id}"]?.get;
    expect(getReport).toBeDefined();
    expect(getReport.operationId).toBe("getReport");

    const response200 = getReport.responses["200"];
    expect(response200.content).toBeDefined();
    expect(response200.content["application/json"]).toBeDefined();
    expect(response200.content["application/json"].schema.$ref).toBe(
      "#/components/schemas/ReportResponse"
    );
  });

  it("should use custom contentType from @Returns options", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    const pdfRoute = doc.paths["/reports/{id}/pdf"]?.get;
    expect(pdfRoute).toBeDefined();
    const pdfResponse = pdfRoute.responses["200"];
    expect(pdfResponse.content["application/pdf"]).toBeDefined();
    expect(pdfResponse.content["application/json"]).toBeUndefined();

    const csvRoute = doc.paths["/reports/{id}/csv"]?.get;
    expect(csvRoute).toBeDefined();
    const csvResponse = csvRoute.responses["200"];
    expect(csvResponse.content["text/csv"]).toBeDefined();
    expect(csvResponse.content["text/csv"].schema.type).toBe("string");
    expect(csvResponse.content["application/json"]).toBeUndefined();
  });

  it("should use custom description from @Returns options", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));

    const pdfResponse = doc.paths["/reports/{id}/pdf"]?.get?.responses["200"];
    expect(pdfResponse.description).toBe("PDF report document");

    const csvResponse = doc.paths["/reports/{id}/csv"]?.get?.responses["200"];
    expect(csvResponse.description).toBe("CSV export");

    const summaryResponse = doc.paths["/reports/summary"]?.get?.responses["200"];
    expect(summaryResponse.description).toBe("OK");
  });

  it("should produce void response for @Res() without @Returns", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const rawRoute = doc.paths["/reports/{id}/raw"]?.get;
    expect(rawRoute).toBeDefined();
    const rawResponse = rawRoute.responses["200"];
    expect(rawResponse.content).toBeUndefined();
    expect(rawResponse.description).toBe("OK");
  });

  it("should emit warning for @Res() without @Returns", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/nestjs-returns/tsconfig.json",
      "--config",
      "testdata/nestjs-returns/tsgonest.config.json",
    ]);
    expect(stderr).toContain("getRawReport");
    expect(stderr).toContain("uses-raw-response");
    expect(stderr).toContain("@Returns<YourType>()");
    expect(stderr).toContain("@tsgonest-ignore");
  });

  it("should NOT emit warning for @Res() with @Returns", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/nestjs-returns/tsconfig.json",
      "--config",
      "testdata/nestjs-returns/tsgonest.config.json",
    ]);
    expect(stderr).not.toContain("getReport —");
    expect(stderr).not.toContain("getReportPdf —");
    expect(stderr).not.toContain("getReportCsv —");
  });

  it("should suppress warning with @tsgonest-ignore uses-raw-response", () => {
    const { stderr } = runTsgonest([
      "--project",
      "testdata/nestjs-returns/tsconfig.json",
      "--config",
      "testdata/nestjs-returns/tsgonest.config.json",
    ]);
    expect(stderr).not.toContain("streamReport");
  });

  it("should handle @Returns status override", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const regenerateRoute = doc.paths["/reports/{id}/regenerate"]?.post;
    expect(regenerateRoute).toBeDefined();
    expect(regenerateRoute.responses["200"]).toBeDefined();
    expect(regenerateRoute.responses["202"]).toBeUndefined();
    expect(
      regenerateRoute.responses["200"].content["application/json"].schema.$ref
    ).toBe("#/components/schemas/ReportResponse");
  });

  it("should not affect normal routes without @Res()", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const summaryRoute = doc.paths["/reports/summary"]?.get;
    expect(summaryRoute).toBeDefined();
    expect(summaryRoute.responses["200"].content["application/json"]).toBeDefined();
    expect(
      summaryRoute.responses["200"].content["application/json"].schema.$ref
    ).toBe("#/components/schemas/ReportSummary");
  });

  it("should include ReportResponse schema in components", () => {
    const doc = JSON.parse(readFileSync(openapiFile, "utf-8"));
    const schema = doc.components?.schemas?.ReportResponse;
    expect(schema).toBeDefined();
    expect(schema.type).toBe("object");
    expect(schema.properties.id).toEqual({ type: "string" });
    expect(schema.properties.title).toEqual({ type: "string" });
    expect(schema.properties.generatedAt).toEqual({ type: "string" });
    expect(schema.required).toContain("id");
    expect(schema.required).toContain("title");
    expect(schema.required).toContain("generatedAt");
  });
});
