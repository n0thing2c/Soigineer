import { Link, useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";
import { useAuth } from "@/auth/AuthContext";
import {
  TerminalSquare,
  AlertTriangle,
  BarChart3,
  Bell,
  Users,
  Settings,
  LogOut,
  RotateCw,
  Radio,
} from "lucide-react";
import { ReactNode } from "react";

interface DashboardLayoutProps {
  children: ReactNode;
  connected?: boolean;
  reconnecting?: boolean;
  realtimePaused?: boolean;
  headerControls?: ReactNode;
  onRefresh?: () => void;
  onToggleRealtime?: () => void;
}

export default function DashboardLayout({
  children,
  connected = true,
  reconnecting = false,
  realtimePaused = false,
  headerControls,
  onRefresh,
  onToggleRealtime,
}: DashboardLayoutProps) {
  const location = useLocation();
  const { user, isAdmin, logout } = useAuth();

  const isActive = (path: string) => location.pathname === path;

  return (
    <div className="flex h-screen bg-background text-foreground">
      <div className="flex w-[240px] shrink-0 flex-col border-r border-sidebar-border bg-sidebar">
        <div className="border-b border-sidebar-border px-6 py-6">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground">
              <TerminalSquare className="h-4 w-4" />
            </div>
            <h1 className="text-xl font-bold text-sidebar-primary">Soigineer</h1>
          </div>
        </div>

        <div className="border-b border-sidebar-border px-6 py-4">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-md border border-sidebar-border bg-white">
              <span className="text-xs font-bold text-sidebar-primary">
                {user?.username?.slice(0, 2).toUpperCase() ?? "SA"}
              </span>
            </div>
            <div className="flex-1 min-w-0">
              <p className="truncate text-sm font-semibold text-sidebar-primary">
                {user?.username ?? "System Admin"}
              </p>
              <p className="truncate text-xs text-sidebar-foreground">
                {user?.role === "admin" ? "System Admin" : "Engineer"}
              </p>
            </div>
          </div>
        </div>

        <nav className="flex-1 space-y-2 px-2 py-4">
          <NavItem
            to="/logs"
            icon={TerminalSquare}
            label="Live Logs"
            active={isActive("/logs") || isActive("/live-logs")}
          />
          <NavItem
            to="/incidents"
            icon={AlertTriangle}
            label="Incidents"
            active={isActive("/incidents")}
          />
          <NavItem
            to="/health"
            icon={BarChart3}
            label="Health"
            active={isActive("/health")}
          />
          {isAdmin && (
            <>
              <NavItem
                to="/alert-rules"
                icon={Bell}
                label="Alert Rules"
                active={isActive("/alert-rules")}
              />
              <NavItem
                to="/users"
                icon={Users}
                label="User Access"
                active={isActive("/users") || isActive("/user-access")}
              />
            </>
          )}
        </nav>

        <div className="space-y-2 border-t border-sidebar-border p-5">
          <NavItem to="/settings" icon={Settings} label="Settings" active={isActive("/settings")} />
          <button
            onClick={logout}
            className="flex w-full items-center gap-3 px-3 py-2 text-sm font-semibold text-destructive transition-colors hover:bg-red-50"
          >
            <LogOut className="w-4 h-4" />
            <span>Logout</span>
          </button>
        </div>
      </div>

      <div className="flex-1 flex flex-col overflow-hidden">
        <header className="flex h-16 items-center justify-between border-b border-border bg-background px-6">
          <div className="flex items-center gap-4">
            <h2 className="text-xl font-bold text-foreground">
              {getPageTitle(location.pathname)}
            </h2>
          </div>
          <div className="flex items-center gap-4">
            {headerControls}
            <button
              onClick={onRefresh}
              className="p-2 transition-colors hover:bg-secondary disabled:opacity-40"
              disabled={!onRefresh}
              title="Refresh"
            >
              <RotateCw className="w-4 h-4 text-foreground" />
            </button>
            <button
              onClick={onToggleRealtime}
              className={cn(
                "p-2 transition-colors hover:bg-secondary disabled:opacity-40",
                realtimePaused && "bg-secondary",
              )}
              disabled={!onToggleRealtime}
              aria-pressed={!realtimePaused}
              title={realtimePaused ? "Resume realtime" : "Pause realtime"}
            >
              <Radio className="w-4 h-4 text-foreground" />
            </button>
            <div className="flex items-center gap-2 rounded-full border border-border px-3 py-1 text-xs font-semibold text-foreground">
              <div
                className={cn(
                  "h-2 w-2 rounded-full",
                  connected ? "bg-emerald-500" : reconnecting ? "bg-red-500" : "bg-slate-400",
                )}
              />
              <span>{reconnecting ? "Reconnecting..." : connected ? "WebSocket: Live" : "Offline"}</span>
            </div>
          </div>
        </header>

        <main className="flex-1 overflow-auto bg-background">{children}</main>
      </div>
    </div>
  );
}

interface NavItemProps {
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  active: boolean;
}

function NavItem({ to, icon: Icon, label, active }: NavItemProps) {
  return (
    <Link
      to={to}
      className={cn(
        "flex items-center gap-3 px-3 py-2 text-sm font-semibold transition-colors",
        active
          ? "bg-[#d7e7ff] text-sidebar-primary"
          : "text-sidebar-foreground hover:bg-sidebar-accent",
      )}
    >
      <Icon className="w-4 h-4" />
      <span>{label}</span>
    </Link>
  );
}

function getPageTitle(pathname: string): string {
  const titles: Record<string, string> = {
    "/logs": "Live Logs",
    "/live-logs": "Live Logs",
    "/incidents": "Incidents",
    "/health": "Health",
    "/alert-rules": "Alert Rules",
    "/users": "User Access Management",
    "/user-access": "User Access Management",
    "/settings": "Settings",
  };
  return titles[pathname] || "Dashboard";
}
