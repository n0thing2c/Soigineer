import { monitoringApi } from "@/api/client";
import type { LogLevel, ProcessedLogEvent } from "@/api/types";
import { useAuth } from "@/auth/AuthContext";
import DashboardLayout from "@/components/DashboardLayout";
import { useRealtimeStream } from "@/hooks/useRealtimeStream";
import { formatNumber, formatTime, truncateMiddle } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Search, SlidersHorizontal, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";

const levels: Array<LogLevel | ""> = ["", "INFO", "WARN", "ERROR", "CRITICAL"];
const logPageSize = 500;

export default function LiveLogs() {
  const { token } = useAuth();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const [selectedApp, setSelectedApp] = useState("");
  const [selectedLevel, setSelectedLevel] = useState<LogLevel | "">("");
  const [search, setSearch] = useState("");
  const [selectedEvent, setSelectedEvent] = useState<ProcessedLogEvent | null>(null);
  const [tailLogs, setTailLogs] = useState(true);
  const [realtimePaused, setRealtimePaused] = useState(false);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);

  const appFilter = selectedApp ? [selectedApp] : undefined;
  const levelFilter = selectedLevel ? [selectedLevel] : undefined;

  const { data: applications = [] } = useQuery({
    queryKey: ["applications", token],
    queryFn: () => monitoringApi.applications(token!),
    enabled: Boolean(token),
  });

  useEffect(() => {
    const app = searchParams.get("app") ?? "";
    if (app !== selectedApp) {
      setSelectedApp(app);
    }
  }, [searchParams, selectedApp]);

  const {
    data: historicalLogPages,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
  } = useInfiniteQuery({
    queryKey: ["logs", token, selectedApp, selectedLevel],
    queryFn: ({ pageParam }) =>
      monitoringApi.logs(token!, {
        app: appFilter,
        level: levelFilter,
        limit: logPageSize,
        offset: pageParam,
      }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) =>
      lastPage.length === logPageSize ? allPages.length * logPageSize : undefined,
    enabled: Boolean(token),
  });

  const historicalLogs = useMemo(
    () => historicalLogPages?.pages.flat() ?? [],
    [historicalLogPages],
  );

  const liveLogs = useRealtimeStream("logs", token, {
    app: appFilter,
    level: levelFilter,
  }, 200, tailLogs && !realtimePaused);
  const liveAlerts = useRealtimeStream("alerts", token, {
    app: appFilter,
    level: levelFilter,
  }, 50, !realtimePaused);

  const logs = useMemo(() => {
    const byId = new Map<string, ProcessedLogEvent>();
    [...liveLogs.events, ...historicalLogs].forEach((log) => {
      byId.set(log.eventId || `${log.timestamp}-${log.traceId}`, log);
    });
    return Array.from(byId.values())
      .filter((log) => {
        const text = `${log.applicationName} ${log.level} ${log.message} ${log.traceId}`.toLowerCase();
        const matchesSearch = text.includes(search.toLowerCase());
        const matchesApp = !selectedApp || log.applicationName === selectedApp;
        const matchesLevel = !selectedLevel || log.level === selectedLevel;
        return matchesSearch && matchesApp && matchesLevel;
      })
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());
  }, [historicalLogs, liveLogs.events, search, selectedApp, selectedLevel]);

  useEffect(() => {
    const target = loadMoreRef.current;
    if (!target || !hasNextPage || isFetchingNextPage) {
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          fetchNextPage();
        }
      },
      { rootMargin: "240px" },
    );
    observer.observe(target);

    return () => observer.disconnect();
  }, [fetchNextPage, hasNextPage, isFetchingNextPage, logs.length]);

  const counts = useMemo(() => {
    return logs.reduce(
      (summary, log) => {
        summary.total += 1;
        if (log.level === "WARN") summary.warn += 1;
        if (log.level === "ERROR") summary.error += 1;
        if (log.level === "CRITICAL") summary.critical += 1;
        return summary;
      },
      { total: 0, warn: 0, error: 0, critical: 0 },
    );
  }, [logs]);

  const latestAlert = liveAlerts.events[0];

  return (
    <DashboardLayout
      connected={liveLogs.connected || liveAlerts.connected}
      reconnecting={liveLogs.reconnecting || liveAlerts.reconnecting}
      realtimePaused={realtimePaused}
      onRefresh={() => {
        queryClient.invalidateQueries({ queryKey: ["logs"] });
        queryClient.invalidateQueries({ queryKey: ["applications"] });
      }}
      onToggleRealtime={() => setRealtimePaused((value) => !value)}
      headerControls={
        <div className="relative hidden md:block">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search logs..."
            className="h-8 w-64 border border-input bg-white pl-9 pr-3 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>
      }
    >
      {latestAlert && (
        <div className="flex h-9 items-center gap-2 border-b border-red-500 bg-red-100 px-6 text-sm font-semibold text-red-800">
          <AlertTriangle className="h-4 w-4" />
          <span>
            {latestAlert.level}: {latestAlert.applicationName} {latestAlert.message}
          </span>
        </div>
      )}

      <div className="p-6">
        <div className="mb-4 flex flex-wrap items-center gap-4">
          <label className="flex items-center gap-2 text-xs font-bold uppercase tracking-wide">
            APP:
            <select
              value={selectedApp}
              onChange={(event) => {
                const next = event.target.value;
                setSelectedApp(next);
                const params = new URLSearchParams(searchParams);
                if (next) {
                  params.set("app", next);
                } else {
                  params.delete("app");
                }
                setSearchParams(params, { replace: true });
              }}
              className="h-8 min-w-36 border border-input bg-white px-2 text-sm font-normal normal-case focus:outline-none focus:ring-1 focus:ring-primary"
            >
              <option value="">All Applications</option>
              {applications.map((app) => (
                <option key={app} value={app}>
                  {app}
                </option>
              ))}
            </select>
          </label>

          <label className="flex items-center gap-2 text-xs font-bold uppercase tracking-wide">
            LEVEL:
            <select
              value={selectedLevel}
              onChange={(event) => setSelectedLevel(event.target.value as LogLevel | "")}
              className="h-8 min-w-32 border border-input bg-white px-2 text-sm font-normal normal-case focus:outline-none focus:ring-1 focus:ring-primary"
            >
              {levels.map((level) => (
                <option key={level || "all"} value={level}>
                  {level || "All Levels"}
                </option>
              ))}
            </select>
          </label>

          <div className="ml-auto flex items-center gap-2 text-xs text-muted-foreground">
            <input
              type="checkbox"
              checked={tailLogs}
              onChange={(event) => setTailLogs(event.target.checked)}
              className="h-4 w-4 accent-primary"
            />
            Tail Logs
          </div>
        </div>

        <div className="mb-4 grid grid-cols-1 gap-4 md:grid-cols-4">
          <SummaryCard label="Total Events" sublabel="loaded" value={formatNumber(counts.total)} />
          <SummaryCard label="Warnings" value={formatNumber(counts.warn)} />
          <SummaryCard label="Errors" value={formatNumber(counts.error)} />
          <SummaryCard label="Critical Alerts" value={formatNumber(counts.critical)} critical />
        </div>

        <div className="min-h-[640px] border border-border bg-white">
          <table className="w-full table-fixed">
            <thead>
              <tr className="border-b border-border bg-secondary/60">
                <HeaderCell className="w-[160px]">Timestamp</HeaderCell>
                <HeaderCell className="w-[180px]">App</HeaderCell>
                <HeaderCell className="w-[90px]">Level</HeaderCell>
                <HeaderCell>Message</HeaderCell>
                <HeaderCell className="w-[130px]">Trace ID</HeaderCell>
              </tr>
            </thead>
            <tbody>
              {logs.map((log) => (
                <tr
                  key={log.eventId || `${log.timestamp}-${log.traceId}`}
                  onClick={() => setSelectedEvent(log)}
                  className="cursor-pointer border-b border-border transition-colors hover:bg-secondary/40"
                >
                  <td className="px-3 py-3 font-mono text-xs">{formatTime(log.timestamp)}</td>
                  <td className="px-3 py-3 font-mono text-xs">{log.applicationName}</td>
                  <td className="px-3 py-3">
                    <LevelBadge level={log.level} />
                  </td>
                  <td className="px-3 py-3 font-mono text-xs">{truncateMiddle(log.message, 70)}</td>
                  <td className="px-3 py-3 font-mono text-xs text-muted-foreground">
                    {log.traceId || "-"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          {!isLoading && logs.length === 0 && (
            <div className="flex min-h-[560px] flex-col items-center justify-center text-center">
              <div className="mb-5 flex h-16 w-16 items-center justify-center bg-secondary">
                <SlidersHorizontal className="h-7 w-7 text-muted-foreground" />
              </div>
              <h3 className="mb-2 text-lg font-bold">No logs found</h3>
              <p className="max-w-md text-sm text-muted-foreground">
                No logs match the current filters
                {selectedApp ? (
                  <>
                    {" "}
                    for <code className="bg-secondary px-1">{selectedApp}</code>
                  </>
                ) : null}
                . Adjust your search or wait for new events.
              </p>
            </div>
          )}

          {logs.length > 0 && (
            <div ref={loadMoreRef} className="flex min-h-14 items-center justify-center border-t border-border bg-white p-3">
              {hasNextPage ? (
                <button
                  onClick={() => fetchNextPage()}
                  disabled={isFetchingNextPage}
                  className="h-8 border border-border bg-white px-4 text-xs font-semibold hover:bg-secondary disabled:opacity-60"
                >
                  {isFetchingNextPage ? "Loading..." : "Load More"}
                </button>
              ) : (
                <span className="text-xs font-semibold text-muted-foreground">End of log history</span>
              )}
            </div>
          )}
        </div>
      </div>

      {selectedEvent && (
        <LogDetails event={selectedEvent} onClose={() => setSelectedEvent(null)} />
      )}
    </DashboardLayout>
  );
}

function HeaderCell({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <th className={cn("px-3 py-3 text-left text-xs font-bold uppercase tracking-wide", className)}>
      {children}
    </th>
  );
}

function SummaryCard({
  label,
  sublabel,
  value,
  critical,
}: {
  label: string;
  sublabel?: string;
  value: string;
  critical?: boolean;
}) {
  return (
    <div className={cn("border border-border bg-white p-4", critical && "border-red-500 bg-red-100")}>
      <p className={cn("text-xs font-bold uppercase tracking-wide", critical && "text-red-800")}>
        {label}
      </p>
      {sublabel && <p className="text-xs font-semibold text-muted-foreground">({sublabel})</p>}
      <p className={cn("mt-2 text-2xl font-bold", critical && "text-red-800")}>{value}</p>
    </div>
  );
}

function LevelBadge({ level }: { level: LogLevel }) {
  return (
    <span
      className={cn(
        "inline-flex px-2 py-1 text-[10px] font-bold",
        level === "CRITICAL" && "bg-purple-600 text-white",
        level === "ERROR" && "bg-red-500 text-white",
        level === "WARN" && "bg-yellow-400 text-primary",
        level === "INFO" && "bg-sky-500 text-white",
      )}
    >
      {level === "CRITICAL" ? "CRIT" : level}
    </span>
  );
}

function LogDetails({
  event,
  onClose,
}: {
  event: ProcessedLogEvent;
  onClose: () => void;
}) {
  return (
    <div className="fixed inset-y-0 right-0 z-40 flex h-dvh w-[360px] max-w-full flex-col border-l border-border bg-background shadow-xl">
      <div className="flex h-16 shrink-0 items-center justify-between border-b border-border px-4">
        <h3 className="text-lg font-bold">Log Event Details</h3>
        <button onClick={onClose} className="p-1 hover:bg-secondary" title="Close">
          <X className="h-5 w-5" />
        </button>
      </div>

      <div className="min-h-0 flex-1 space-y-5 overflow-y-auto p-4">
        <div className="flex items-center gap-2">
          <LevelBadge level={event.level} />
          <span className="font-mono text-xs text-muted-foreground">{formatTime(event.timestamp)} UTC</span>
        </div>

        <div className="border border-border bg-white p-3">
          <p className="mb-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
            Message
          </p>
          <p className="font-mono text-sm">{event.message}</p>
        </div>

        <DetailRow label="eventId" value={event.eventId} />
        <DetailRow label="applicationName" value={event.applicationName} />
        <DetailRow label="traceId" value={event.traceId} />
        <DetailRow label="fingerprint" value={event.fingerprint} shaded />

        <div>
          <p className="mb-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
            Raw JSON
          </p>
          <pre className="max-h-80 overflow-auto bg-slate-900 p-3 text-xs text-slate-100">
            {JSON.stringify(event, null, 2)}
          </pre>
        </div>
      </div>
    </div>
  );
}

function DetailRow({
  label,
  value,
  shaded,
}: {
  label: string;
  value: string;
  shaded?: boolean;
}) {
  return (
    <div className="border-b border-border pb-2">
      <div className="flex items-center justify-between gap-4 text-sm">
        <span className="shrink-0 font-bold text-muted-foreground">{label}</span>
        <span className={cn("min-w-0 text-right font-mono text-xs", shaded && "bg-secondary px-2 py-1")}>
          {value || "-"}
        </span>
      </div>
    </div>
  );
}
