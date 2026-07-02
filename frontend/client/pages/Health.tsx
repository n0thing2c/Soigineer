import { monitoringApi } from "@/api/client";
import type { HealthRow, LogLevel } from "@/api/types";
import { useAuth } from "@/auth/AuthContext";
import DashboardLayout from "@/components/DashboardLayout";
import { formatNumber, formatRelative } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Filter, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

const levels: Array<LogLevel | ""> = ["", "INFO", "WARN", "ERROR", "CRITICAL"];

export default function Health() {
  const { token } = useAuth();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [filterOpen, setFilterOpen] = useState(false);
  const [selectedApp, setSelectedApp] = useState("");
  const [selectedLevel, setSelectedLevel] = useState<LogLevel | "">("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [draftApp, setDraftApp] = useState("");
  const [draftLevel, setDraftLevel] = useState<LogLevel | "">("");
  const [draftFrom, setDraftFrom] = useState("");
  const [draftTo, setDraftTo] = useState("");

  const { data: rows = [], isLoading } = useQuery({
    queryKey: ["health", token, selectedApp, selectedLevel, from, to],
    queryFn: () =>
      monitoringApi.health(token!, {
        app: selectedApp ? [selectedApp] : undefined,
        level: selectedLevel ? [selectedLevel] : undefined,
        from: from ? new Date(from).toISOString() : undefined,
        to: to ? new Date(to).toISOString() : undefined,
        limit: 100,
      }),
    enabled: Boolean(token),
  });

  const { data: applications = [] } = useQuery({
    queryKey: ["applications", token],
    queryFn: () => monitoringApi.applications(token!),
    enabled: Boolean(token),
  });

  const visibleRows = useMemo(
    () => rows.filter((row) => row.applicationName.toLowerCase().includes(search.toLowerCase())),
    [rows, search],
  );

  const summary = useMemo(() => {
    const critical = rows.reduce((total, row) => total + Number(row.criticalCount), 0);
    const warnings = rows.reduce((total, row) => total + Number(row.warnCount), 0);
    const errors = rows.reduce((total, row) => total + Number(row.errorCount), 0);
    const total = rows.reduce((sum, row) => sum + Number(row.totalCount), 0);
    const stability = total ? Math.max(0, Math.round(100 - ((critical * 8 + errors * 3 + warnings) / total) * 100)) : 100;

    return {
      apps: rows.length,
      critical,
      warnings,
      stability,
    };
  }, [rows]);

  return (
    <DashboardLayout
      onRefresh={() => {
        queryClient.invalidateQueries({ queryKey: ["health"] });
        queryClient.invalidateQueries({ queryKey: ["applications"] });
      }}
      headerControls={
        <div className="relative hidden md:block">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search applications..."
            className="h-8 w-64 border border-input bg-white pl-9 pr-3 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>
      }
    >
      <div className="p-6">
        <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-4">
          <MetricCard label="Total Apps Monitored" value={formatNumber(summary.apps)} />
          <MetricCard label="Critical Incidents" value={formatNumber(summary.critical)} danger />
          <MetricCard label="Warning State" value={formatNumber(summary.warnings)} warning />
          <MetricCard label="System Stability" value={`${summary.stability}%`} progress={summary.stability} />
        </div>

        <section className="border border-border bg-white">
          <div className="flex h-12 items-center justify-between border-b border-border px-4">
            <h2 className="text-lg font-bold">Application Status</h2>
            <button
              onClick={() => setFilterOpen((value) => !value)}
              className="flex items-center gap-1 text-xs font-semibold text-foreground"
            >
              <Filter className="h-3.5 w-3.5" />
              Filter
            </button>
          </div>

          {filterOpen && (
            <div className="grid grid-cols-1 gap-3 border-b border-border bg-secondary/40 p-4 md:grid-cols-6">
              <select
                value={draftApp}
                onChange={(event) => setDraftApp(event.target.value)}
                className="h-8 border border-input bg-white px-2 text-sm"
              >
                <option value="">All applications</option>
                {applications.map((app) => (
                  <option key={app} value={app}>
                    {app}
                  </option>
                ))}
              </select>
              <select
                value={draftLevel}
                onChange={(event) => setDraftLevel(event.target.value as LogLevel | "")}
                className="h-8 border border-input bg-white px-2 text-sm"
              >
                {levels.map((level) => (
                  <option key={level || "all"} value={level}>
                    {level || "All levels"}
                  </option>
                ))}
              </select>
              <input
                type="datetime-local"
                value={draftFrom}
                onChange={(event) => setDraftFrom(event.target.value)}
                className="h-8 border border-input bg-white px-2 text-sm"
              />
              <input
                type="datetime-local"
                value={draftTo}
                onChange={(event) => setDraftTo(event.target.value)}
                className="h-8 border border-input bg-white px-2 text-sm"
              />
              <button
                onClick={() => {
                  setSelectedApp("");
                  setSelectedLevel("");
                  setFrom("");
                  setTo("");
                  setDraftApp("");
                  setDraftLevel("");
                  setDraftFrom("");
                  setDraftTo("");
                }}
                className="h-8 border border-border bg-white px-3 text-sm font-semibold hover:bg-secondary"
              >
                Clear
              </button>
              <button
                onClick={() => {
                  setSelectedApp(draftApp);
                  setSelectedLevel(draftLevel);
                  setFrom(draftFrom);
                  setTo(draftTo);
                }}
                className="h-8 bg-primary px-3 text-sm font-semibold text-primary-foreground hover:bg-primary/90"
              >
                Apply
              </button>
            </div>
          )}

          <table className="w-full table-fixed">
            <thead>
              <tr className="border-b border-border bg-secondary/50">
                <Header className="w-[280px]">Application Name</Header>
                <Header>Total Logs</Header>
                <Header>Warn</Header>
                <Header>Error</Header>
                <Header>Critical</Header>
                <Header>Risk Score</Header>
                <Header>Last Seen</Header>
                <Header>Actions</Header>
              </tr>
            </thead>
            <tbody>
              {visibleRows.map((row) => (
                <ApplicationRow
                  key={row.applicationName}
                  row={row}
                  onViewLogs={() => navigate(`/logs?app=${encodeURIComponent(row.applicationName)}`)}
                />
              ))}
            </tbody>
          </table>

          {!isLoading && visibleRows.length === 0 && (
            <div className="p-10 text-center text-sm text-muted-foreground">
              No monitored applications match the current filter.
            </div>
          )}
        </section>
      </div>
    </DashboardLayout>
  );
}

function MetricCard({
  label,
  value,
  danger,
  warning,
  progress,
}: {
  label: string;
  value: string;
  danger?: boolean;
  warning?: boolean;
  progress?: number;
}) {
  return (
    <div className="min-h-[90px] border border-border bg-white p-4">
      <p className="mb-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">{label}</p>
      <div className="flex items-center gap-2">
        <p className={cn("text-3xl font-bold", danger && "text-red-700", warning && "text-amber-950")}>
          {value}
        </p>
        {progress !== undefined && (
          <div className="h-2 flex-1 bg-secondary">
            <div className="h-2 bg-primary" style={{ width: `${Math.min(100, Math.max(0, progress))}%` }} />
          </div>
        )}
      </div>
    </div>
  );
}

function Header({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <th className={cn("px-3 py-3 text-left text-xs font-bold uppercase tracking-wide", className)}>
      {children}
    </th>
  );
}

function ApplicationRow({ row, onViewLogs }: { row: HealthRow; onViewLogs: () => void }) {
  const risk = riskScore(row);
  const riskColor = risk >= 70 ? "bg-red-700" : risk >= 30 ? "bg-amber-950" : "bg-emerald-500";
  const dotColor = risk >= 70 ? "bg-red-700" : risk >= 30 ? "bg-amber-950" : "bg-emerald-500";

  return (
    <tr className="border-b border-border hover:bg-secondary/40">
      <td className="px-3 py-3">
        <div className="flex items-center gap-2">
          <span className={cn("h-2 w-2 rounded-full", dotColor)} />
          <span className="font-mono text-xs">{row.applicationName}</span>
        </div>
      </td>
      <td className="px-3 py-3 font-mono text-xs">{formatNumber(Number(row.totalCount))}</td>
      <td className="px-3 py-3 font-mono text-xs">{formatNumber(Number(row.warnCount))}</td>
      <td className="px-3 py-3 font-mono text-xs text-red-200">{formatNumber(Number(row.errorCount))}</td>
      <td className="px-3 py-3 font-mono text-xs text-red-700">{formatNumber(Number(row.criticalCount))}</td>
      <td className="px-3 py-3">
        <div className="h-2 w-24 bg-secondary">
          <div className={cn("h-2", riskColor)} style={{ width: `${risk}%` }} />
        </div>
      </td>
      <td className="px-3 py-3 font-mono text-xs text-muted-foreground">{formatRelative(row.lastSeenAt)}</td>
      <td className="px-3 py-3">
        <button onClick={onViewLogs} className="text-xs font-semibold text-primary hover:underline">
          View Logs
        </button>
      </td>
    </tr>
  );
}

function riskScore(row: HealthRow) {
  const total = Math.max(1, Number(row.totalCount));
  const weighted = Number(row.warnCount) + Number(row.errorCount) * 3 + Number(row.criticalCount) * 8;
  return Math.min(100, Math.max(2, Math.round((weighted / total) * 1000)));
}
