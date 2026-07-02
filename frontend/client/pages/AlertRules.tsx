import { authApi, monitoringApi } from "@/api/client";
import type { AlertRule, AlertRuleCreate } from "@/api/types";
import { useAuth } from "@/auth/AuthContext";
import DashboardLayout from "@/components/DashboardLayout";
import { cn } from "@/lib/utils";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Plus, X } from "lucide-react";
import { useState } from "react";

interface DraftRule {
  dedupWindowSeconds: number;
  enabled: boolean;
  telegramEnabled: boolean;
}

const emptyNewRule: AlertRuleCreate = {
  applicationName: "",
  level: "ERROR",
  dedupWindowSeconds: 60,
  enabled: true,
  telegramEnabled: true,
};

export default function AlertRules() {
  const { token } = useAuth();
  const queryClient = useQueryClient();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<DraftRule | null>(null);
  const [creating, setCreating] = useState(false);
  const [newRule, setNewRule] = useState<AlertRuleCreate>(emptyNewRule);

  const { data: rules = [], isLoading } = useQuery({
    queryKey: ["alert-rules", token],
    queryFn: () => monitoringApi.alertRules(token!),
    enabled: Boolean(token),
  });

  const { data: applications = [] } = useQuery({
    queryKey: ["admin-applications", token],
    queryFn: () => authApi.adminApplications(token!),
    enabled: Boolean(token),
  });

  const updateRule = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: DraftRule }) =>
      monitoringApi.updateAlertRule(token!, id, payload),
    onSuccess: () => {
      setEditingId(null);
      setDraft(null);
      queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
    },
  });

  const createRule = useMutation({
    mutationFn: () => monitoringApi.createAlertRule(token!, newRule),
    onSuccess: () => {
      setCreating(false);
      setNewRule(emptyNewRule);
      queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
    },
  });

  const startEditing = (rule: AlertRule) => {
    setEditingId(rule.id);
    setDraft({
      dedupWindowSeconds: rule.dedupWindowSeconds,
      enabled: rule.enabled,
      telegramEnabled: rule.telegramEnabled,
    });
  };

  return (
    <DashboardLayout
      onRefresh={() => {
        queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
        queryClient.invalidateQueries({ queryKey: ["admin-applications"] });
      }}
    >
      <div className="p-6">
        <div className="mb-6 flex items-start justify-between">
          <div>
            <h1 className="text-lg font-bold">Routing Configuration</h1>
            <p className="text-sm text-muted-foreground">
              Manage global alerting thresholds and notification destinations.
            </p>
          </div>
          <button
            onClick={() => {
              setCreating(true);
              setNewRule({
                ...emptyNewRule,
                applicationName: applications[0] ?? "",
              });
            }}
            disabled={creating || applications.length === 0}
            className="flex h-8 items-center gap-2 bg-primary px-4 text-sm font-semibold text-primary-foreground disabled:opacity-60"
            title={applications.length === 0 ? "Create an application first" : "New Rule"}
          >
            <Plus className="h-4 w-4" />
            New Rule
          </button>
        </div>

        <div className="border border-border bg-white">
          <table className="w-full table-fixed">
            <thead>
              <tr className="border-b border-border bg-secondary/50">
                <Header className="w-[260px]">App Name</Header>
                <Header>Level</Header>
                <Header>Dedup Window (s)</Header>
                <Header>Enabled</Header>
                <Header>Telegram</Header>
                <Header>Actions</Header>
              </tr>
            </thead>
            <tbody>
              {creating && (
                <NewRuleRow
                  applications={applications}
                  rule={newRule}
                  saving={createRule.isPending}
                  error={createRule.error}
                  onChange={setNewRule}
                  onCancel={() => {
                    setCreating(false);
                    setNewRule(emptyNewRule);
                  }}
                  onSave={() => createRule.mutate()}
                />
              )}
              {rules.map((rule) => (
                <RuleRow
                  key={rule.id}
                  rule={rule}
                  editing={editingId === rule.id}
                  draft={editingId === rule.id ? draft : null}
                  saving={updateRule.isPending}
                  onEdit={() => startEditing(rule)}
                  onCancel={() => {
                    setEditingId(null);
                    setDraft(null);
                  }}
                  onDraftChange={setDraft}
                  onSave={() => draft && updateRule.mutate({ id: rule.id, payload: draft })}
                />
              ))}
            </tbody>
          </table>

          {!isLoading && rules.length === 0 && (
            <div className="p-10 text-center text-sm text-muted-foreground">
              No alert rules are configured.
            </div>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

function NewRuleRow({
  applications,
  rule,
  saving,
  error,
  onChange,
  onCancel,
  onSave,
}: {
  applications: string[];
  rule: AlertRuleCreate;
  saving: boolean;
  error: unknown;
  onChange: (rule: AlertRuleCreate) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  return (
    <>
      <tr className="border-b border-primary bg-secondary/30 outline outline-1 outline-primary">
        <td className="px-4 py-3">
          <select
            value={rule.applicationName}
            onChange={(event) => onChange({ ...rule, applicationName: event.target.value })}
            className="h-8 w-full border border-input bg-white px-2 text-sm"
          >
            {applications.map((app) => (
              <option key={app} value={app}>
                {app}
              </option>
            ))}
          </select>
        </td>
        <td className="px-4 py-3">
          <select
            value={rule.level}
            onChange={(event) =>
              onChange({ ...rule, level: event.target.value as AlertRuleCreate["level"] })
            }
            className="h-8 w-32 border border-input bg-white px-2 text-sm"
          >
            <option value="ERROR">ERROR</option>
            <option value="CRITICAL">CRITICAL</option>
          </select>
        </td>
        <td className="px-4 py-3">
          <input
            type="number"
            min={1}
            value={rule.dedupWindowSeconds}
            onChange={(event) =>
              onChange({ ...rule, dedupWindowSeconds: Math.max(1, Number(event.target.value)) })
            }
            className="h-8 w-24 border border-input bg-white px-2 text-sm"
          />
        </td>
        <td className="px-4 py-3">
          <Toggle
            enabled={rule.enabled}
            onChange={(enabled) => onChange({ ...rule, enabled })}
          />
        </td>
        <td className="px-4 py-3">
          <Toggle
            enabled={rule.telegramEnabled}
            onChange={(telegramEnabled) => onChange({ ...rule, telegramEnabled })}
          />
        </td>
        <td className="px-4 py-3">
          <div className="flex gap-2">
            <button
              onClick={onCancel}
              disabled={saving}
              className="flex h-8 w-8 items-center justify-center border border-border bg-white hover:bg-secondary"
              title="Cancel"
            >
              <X className="h-4 w-4" />
            </button>
            <button
              onClick={onSave}
              disabled={saving || !rule.applicationName}
              className="flex h-8 w-8 items-center justify-center bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-60"
              title="Save"
            >
              <Check className="h-4 w-4" />
            </button>
          </div>
        </td>
      </tr>
      {error && (
        <tr className="border-b border-border">
          <td colSpan={6} className="px-4 py-2 text-xs font-semibold text-red-700">
            {error instanceof Error ? error.message : "Unable to create alert rule"}
          </td>
        </tr>
      )}
    </>
  );
}

function RuleRow({
  rule,
  editing,
  draft,
  saving,
  onEdit,
  onCancel,
  onDraftChange,
  onSave,
}: {
  rule: AlertRule;
  editing: boolean;
  draft: DraftRule | null;
  saving: boolean;
  onEdit: () => void;
  onCancel: () => void;
  onDraftChange: (draft: DraftRule) => void;
  onSave: () => void;
}) {
  const current = draft ?? {
    dedupWindowSeconds: rule.dedupWindowSeconds,
    enabled: rule.enabled,
    telegramEnabled: rule.telegramEnabled,
  };

  return (
    <tr className={cn("border-b border-border", editing && "outline outline-1 outline-primary")}>
      <td className="px-4 py-3 font-mono text-sm">{rule.applicationName}</td>
      <td className="px-4 py-3">
        <LevelBadge level={rule.level} />
      </td>
      <td className="px-4 py-3">
        {editing ? (
          <input
            type="number"
            min={1}
            value={current.dedupWindowSeconds}
            onChange={(event) =>
              onDraftChange({
                ...current,
                dedupWindowSeconds: Math.max(1, Number(event.target.value)),
              })
            }
            className="h-8 w-24 border border-input bg-white px-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
          />
        ) : (
          <span className="font-mono text-sm">{rule.dedupWindowSeconds}</span>
        )}
      </td>
      <td className="px-4 py-3">
        <Toggle
          enabled={current.enabled}
          disabled={!editing}
          onChange={(enabled) => onDraftChange({ ...current, enabled })}
        />
      </td>
      <td className="px-4 py-3">
        <Toggle
          enabled={current.telegramEnabled}
          disabled={!editing}
          onChange={(telegramEnabled) => onDraftChange({ ...current, telegramEnabled })}
        />
      </td>
      <td className="px-4 py-3">
        {editing ? (
          <div className="flex gap-2">
            <button
              onClick={onCancel}
              disabled={saving}
              className="flex h-8 w-8 items-center justify-center border border-border bg-white hover:bg-secondary"
              title="Cancel"
            >
              <X className="h-4 w-4" />
            </button>
            <button
              onClick={onSave}
              disabled={saving}
              className="flex h-8 w-8 items-center justify-center bg-primary text-primary-foreground hover:bg-primary/90"
              title="Save"
            >
              <Check className="h-4 w-4" />
            </button>
          </div>
        ) : (
          <button onClick={onEdit} className="text-sm font-semibold text-primary hover:underline">
            Edit
          </button>
        )}
      </td>
    </tr>
  );
}

function Header({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <th className={cn("px-4 py-3 text-left text-xs font-bold uppercase tracking-wide", className)}>
      {children}
    </th>
  );
}

function LevelBadge({ level }: { level: AlertRule["level"] }) {
  return (
    <span
      className={cn(
        "inline-flex px-2 py-1 text-xs font-bold",
        level === "CRITICAL" ? "bg-red-700 text-white" : "bg-red-100 text-red-700",
      )}
    >
      {level}
    </span>
  );
}

function Toggle({
  enabled,
  disabled,
  onChange,
}: {
  enabled: boolean;
  disabled?: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={() => onChange(!enabled)}
      className={cn(
        "relative inline-flex h-5 w-9 items-center rounded-full transition-colors disabled:cursor-default",
        enabled ? "bg-primary" : "bg-muted",
        disabled && "opacity-70",
      )}
    >
      <span
        className={cn(
          "inline-block h-4 w-4 transform rounded-full bg-white transition-transform",
          enabled ? "translate-x-4" : "translate-x-1",
        )}
      />
    </button>
  );
}
