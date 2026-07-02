export type Role = "admin" | "engineer";
export type LogLevel = "INFO" | "WARN" | "ERROR" | "CRITICAL";
export type IncidentStatus = "OPEN" | "ACKED" | "RESOLVED";

export interface User {
  id: string;
  username: string;
  role: Role;
  applications: string[];
}

export interface LoginResult {
  token: string;
  refreshToken: string;
  user: User;
}

export interface Application {
  id: string;
  name: string;
  displayName: string;
}

export interface ProcessedLogEvent {
  eventId: string;
  applicationName: string;
  level: LogLevel;
  category?: string;
  message: string;
  normalizedMessage: string;
  timestamp: string;
  receivedAt: string;
  traceId: string;
  fingerprint: string;
}

export interface AlertEvent {
  eventId: string;
  applicationName: string;
  level: LogLevel;
  category?: string;
  message: string;
  fingerprint: string;
  traceId: string;
  timestamp: string;
}

export interface Incident {
  id: string;
  applicationName: string;
  fingerprint: string;
  level: "ERROR" | "CRITICAL";
  category: string;
  title: string;
  firstSeenAt: string;
  lastSeenAt: string;
  occurrenceCount: number;
  suppressedCount: number;
  status: IncidentStatus;
}

export interface HealthRow {
  applicationName: string;
  totalCount: number;
  warnCount: number;
  errorCount: number;
  criticalCount: number;
  lastSeenAt: string;
}

export interface AlertRule {
  id: string;
  applicationName: string;
  level: "ERROR" | "CRITICAL";
  enabled: boolean;
  dedupWindowSeconds: number;
  telegramEnabled: boolean;
}

export interface AlertRuleCreate {
  applicationName: string;
  level: "ERROR" | "CRITICAL";
  enabled: boolean;
  dedupWindowSeconds: number;
  telegramEnabled: boolean;
}
