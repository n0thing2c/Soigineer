import { monitoringApi } from "@/api/client";
import type { AlertEvent, Incident, IncidentStatus } from "@/api/types";
import { useAuth } from "@/auth/AuthContext";
import DashboardLayout from "@/components/DashboardLayout";
import { useRealtimeStream } from "@/hooks/useRealtimeStream";
import { formatNumber, formatTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, ChevronDown, Clock, Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

const statuses: Array<IncidentStatus | ""> = ["OPEN", "ACKED", "RESOLVED", ""];
const severities = ["", "ERROR", "CRITICAL"];

export default function Incidents() {
  const { token, isAdmin } = useAuth();
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<IncidentStatus | "OPEN" | "">("OPEN");
  const [level, setLevel] = useState("");
  const [search, setSearch] = useState("");
  const [realtimePaused, setRealtimePaused] = useState(false);
  const seenAlertIds = useRef(new Set<string>());
  const refetchTimer = useRef<number>();

  const incidentsQueryKey = useMemo(
    () => ["incidents", token, status, level],
    [level, status, token],
  );

  const { data: incidents = [], isLoading } = useQuery({
    queryKey: incidentsQueryKey,
    queryFn: () =>
      monitoringApi.incidents(token!, {
        status: status || undefined,
        level: level ? [level] : undefined,
        limit: 100,
      }),
    enabled: Boolean(token),
  });

  const updateStatus = useMutation({
    mutationFn: ({ id, nextStatus }: { id: string; nextStatus: IncidentStatus }) =>
      monitoringApi.updateIncidentStatus(token!, id, nextStatus),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["incidents"] }),
  });

  const liveAlerts = useRealtimeStream("alerts", token, {
    level: level ? [level] : undefined,
  }, 100, !realtimePaused);

  useEffect(() => {
    const scheduleRefetch = () => {
      if (refetchTimer.current) {
        window.clearTimeout(refetchTimer.current);
      }
      refetchTimer.current = window.setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ["incidents"] });
      }, 600);
    };

    liveAlerts.events.forEach((alert) => {
      const key = alert.eventId || `${alert.applicationName}-${alert.fingerprint}-${alert.timestamp}`;
      if (seenAlertIds.current.has(key)) {
        return;
      }
      seenAlertIds.current.add(key);

      let matched = false;
      queryClient.setQueryData<Incident[]>(incidentsQueryKey, (current = []) =>
        current.map((incident) => {
          if (!sameIncident(incident, alert)) {
            return incident;
          }
          matched = true;
          return {
            ...incident,
            occurrenceCount: Number(incident.occurrenceCount) + 1,
            lastSeenAt: alert.timestamp,
          };
        }),
      );

      const alertMatchesFilters =
        (!level || alert.level === level) && (!status || status === "OPEN");
      if (!matched && alertMatchesFilters) {
        scheduleRefetch();
      } else if (matched) {
        scheduleRefetch();
      }
    });

    return () => {
      if (refetchTimer.current) {
        window.clearTimeout(refetchTimer.current);
      }
    };
  }, [incidentsQueryKey, level, liveAlerts.events, queryClient, status]);

  const visibleIncidents = useMemo(
    () =>
      incidents.filter((incident) => {
        const text = `${incident.applicationName} ${incident.level} ${incident.status} ${incident.title}`.toLowerCase();
        return text.includes(search.toLowerCase());
      }),
    [incidents, search],
  );

  return (
    <DashboardLayout
      connected={liveAlerts.connected}
      reconnecting={liveAlerts.reconnecting}
      realtimePaused={realtimePaused}
      onRefresh={() => queryClient.invalidateQueries({ queryKey: ["incidents"] })}
      onToggleRealtime={() => setRealtimePaused((value) => !value)}
      headerControls={
        <div className="relative hidden md:block">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search system logs..."
            className="h-8 w-64 border border-input bg-white pl-9 pr-3 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>
      }
    >
      <div className="p-6">
        <div className="mb-2 flex items-start justify-between gap-4 border-b border-border pb-3">
          <div>
            <h1 className="text-2xl font-bold text-foreground">Active Incidents</h1>
            <p className="text-sm text-muted-foreground">
              Manage and resolve high-priority system alerts.
            </p>
          </div>
          <div className="flex gap-2">
            <SelectFilter value={level} onChange={setLevel} options={severities} allLabel="All Severities" />
            <SelectFilter
              value={status}
              onChange={(value) => setStatus(value as IncidentStatus | "")}
              options={statuses}
              allLabel="All Statuses"
              prefix="Status: "
            />
          </div>
        </div>

        <div className="mt-6 grid grid-cols-1 gap-4 md:grid-cols-3">
          {visibleIncidents.map((incident) => (
            <IncidentCard
              key={incident.id}
              incident={incident}
              canEdit={isAdmin}
              saving={updateStatus.isPending}
              onStatusChange={(nextStatus) =>
                updateStatus.mutate({ id: incident.id, nextStatus })
              }
            />
          ))}
        </div>

        {!isLoading && visibleIncidents.length === 0 && (
          <div className="mt-12 border border-border bg-white p-10 text-center">
            <h3 className="text-lg font-bold">No incidents found</h3>
            <p className="mt-2 text-sm text-muted-foreground">
              The current filters do not match any open incident.
            </p>
          </div>
        )}
      </div>
    </DashboardLayout>
  );
}

function SelectFilter({
  value,
  options,
  allLabel,
  prefix = "",
  onChange,
}: {
  value: string;
  options: string[];
  allLabel: string;
  prefix?: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="relative">
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-8 min-w-40 appearance-none border border-input bg-white px-9 pr-8 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
      >
        {options.map((option) => (
          <option key={option || "all"} value={option}>
            {option ? `${prefix}${option}` : allLabel}
          </option>
        ))}
      </select>
      <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
    </label>
  );
}

function sameIncident(incident: Incident, alert: AlertEvent) {
  return (
    incident.fingerprint === alert.fingerprint ||
    (incident.applicationName === alert.applicationName &&
      incident.level === alert.level &&
      incident.title === alert.message)
  );
}

function IncidentCard({
  incident,
  canEdit,
  saving,
  onStatusChange,
}: {
  incident: Incident;
  canEdit: boolean;
  saving: boolean;
  onStatusChange: (status: IncidentStatus) => void;
}) {
  const isCritical = incident.level === "CRITICAL";

  return (
    <article className="border border-border bg-white p-4">
      <div className="mb-4 flex items-start gap-2">
        <span
          className={cn(
            "inline-flex h-9 min-w-[88px] shrink-0 items-center justify-center gap-1 whitespace-nowrap px-2 text-xs font-bold",
            isCritical ? "bg-red-100 text-red-800" : "bg-amber-100 text-amber-900",
          )}
        >
          <span className={cn("h-1.5 w-1.5 rounded-full", isCritical ? "bg-red-700" : "bg-amber-900")} />
          {incident.level}
        </span>
        <span className="inline-flex min-h-9 min-w-0 items-center bg-secondary px-2 font-mono text-xs font-bold uppercase">
          {incident.applicationName}
        </span>
        <select
          disabled={!canEdit || saving}
          value={incident.status}
          onChange={(event) => onStatusChange(event.target.value as IncidentStatus)}
          className={cn(
            "ml-auto h-9 border px-2 text-xs font-bold",
            incident.status === "OPEN"
              ? "border-red-500 bg-red-50 text-red-700"
              : "border-border bg-white text-foreground",
          )}
        >
          <option value="OPEN">OPEN</option>
          <option value="ACKED">ACKED</option>
          <option value="RESOLVED">RESOLVED</option>
        </select>
      </div>

      <h3 className="mb-4 min-h-[56px] text-xl font-bold leading-snug">
        {incident.title || incident.fingerprint}
      </h3>

      <div className="space-y-2 border-t border-border pt-3 text-sm">
        <Metric icon={Clock} label="First seen:" value={`${formatTime(incident.firstSeenAt)} UTC`} />
        <Metric icon={Clock} label="Last seen:" value={`${formatTime(incident.lastSeenAt)} UTC`} />
        <Metric
          icon={Activity}
          label="Occurrences:"
          value={formatNumber(incident.occurrenceCount)}
          danger={isCritical && incident.occurrenceCount > 100}
        />
      </div>
    </article>
  );
}

function Metric({
  icon: Icon,
  label,
  value,
  danger,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  danger?: boolean;
}) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="flex items-center gap-1 text-muted-foreground">
        <Icon className="h-3.5 w-3.5" />
        {label}
      </span>
      <span className={cn("font-mono text-xs", danger && "text-red-700")}>{value}</span>
    </div>
  );
}
