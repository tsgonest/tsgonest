/** Dashboard statistics */
export interface AdminDashboardStats {
  totalUsers: number;
  totalArticles: number;
  totalPayments: number;
  revenue: number;
  /** Metrics by day: key is ISO date string */
  dailyMetrics: Record<string, DailyMetric>;
}

/** Daily metric data point */
export interface DailyMetric {
  signups: number;
  activeUsers: number;
  articlesPublished: number;
  revenue: number;
}

/** System configuration — key-value pairs */
export interface SystemConfig {
  siteName: string;
  maintenanceMode: boolean;
  maxUploadSizeMb: number;
  allowedOrigins: string[];
  /** Arbitrary feature flags */
  featureFlags: Record<string, boolean>;
}

/** Partial update of system config */
export type UpdateSystemConfigDto = Partial<SystemConfig>;

/** Audit log entry */
export interface AuditLogEntry {
  id: number;
  userId: number;
  action: string;
  resource: string;
  resourceId: string;
  metadata: Record<string, string | number | boolean>;
  createdAt: string;
}

/** Coordinate pair as tuple */
export type Coordinates = [latitude: number, longitude: number];
