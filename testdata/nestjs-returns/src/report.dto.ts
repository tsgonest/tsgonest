export interface ReportResponse {
  id: string;
  title: string;
  generatedAt: string;
}

export interface ReportSummary {
  totalReports: number;
  lastGenerated: string;
}
