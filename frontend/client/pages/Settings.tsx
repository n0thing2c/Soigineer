import { getApiConfig } from "@/api/client";
import { useAuth } from "@/auth/AuthContext";
import DashboardLayout from "@/components/DashboardLayout";
import { LogOut, RefreshCw } from "lucide-react";
import { useState } from "react";

export default function Settings() {
  const { user, token, refreshToken, refreshSession, logout } = useAuth();
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);
  const apiConfig = getApiConfig();

  const handleRefresh = async () => {
    setSaving(true);
    setMessage("");
    try {
      await refreshSession();
      setMessage("Session refreshed.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to refresh session.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <DashboardLayout onRefresh={handleRefresh}>
      <div className="grid max-w-5xl grid-cols-1 gap-6 p-6 md:grid-cols-2">
        <section className="border border-border bg-white p-6">
          <h1 className="mb-4 text-xl font-bold">Current Session</h1>
          <div className="space-y-3 text-sm">
            <SettingRow label="Username" value={user?.username ?? "-"} />
            <SettingRow label="Role" value={user?.role ?? "-"} />
            <SettingRow label="Applications" value={user?.role === "admin" ? "All Apps" : user?.applications.join(", ") || "-"} />
            <SettingRow label="Access Token" value={token ? "Present" : "Missing"} />
            <SettingRow label="Refresh Token" value={refreshToken ? "Present" : "Missing"} />
          </div>
          {message && <p className="mt-4 text-xs font-semibold text-muted-foreground">{message}</p>}
          <div className="mt-6 flex gap-3">
            <button
              onClick={handleRefresh}
              disabled={saving || !refreshToken}
              className="flex h-9 items-center gap-2 bg-primary px-4 text-sm font-semibold text-primary-foreground hover:bg-primary/90 disabled:opacity-60"
            >
              <RefreshCw className="h-4 w-4" />
              Refresh Session
            </button>
            <button
              onClick={logout}
              className="flex h-9 items-center gap-2 border border-red-300 px-4 text-sm font-semibold text-red-700 hover:bg-red-50"
            >
              <LogOut className="h-4 w-4" />
              Logout
            </button>
          </div>
        </section>

        <section className="border border-border bg-white p-6">
          <h2 className="mb-4 text-xl font-bold">API Endpoints</h2>
          <div className="space-y-3 text-sm">
            <SettingRow label="Auth API" value={apiConfig.authBaseUrl} />
            <SettingRow label="Monitoring API" value={apiConfig.monitoringBaseUrl} />
          </div>
        </section>
      </div>
    </DashboardLayout>
  );
}

function SettingRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-b border-border pb-2">
      <p className="text-xs font-bold uppercase tracking-wide text-muted-foreground">{label}</p>
      <p className="mt-1 break-all font-mono text-xs text-foreground">{value}</p>
    </div>
  );
}
