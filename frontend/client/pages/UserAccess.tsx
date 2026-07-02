import { authApi } from "@/api/client";
import type { User } from "@/api/types";
import { useAuth } from "@/auth/AuthContext";
import DashboardLayout from "@/components/DashboardLayout";
import { cn } from "@/lib/utils";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Boxes, Save, Search, UserPlus, Users } from "lucide-react";
import { useMemo, useState } from "react";

export default function UserAccess() {
  const { token } = useAuth();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [form, setForm] = useState({
    username: "",
    password: "auto-generated-123",
    role: "engineer",
    applications: [] as string[],
  });
  const [appForm, setAppForm] = useState({
    name: "",
    displayName: "",
  });

  const { data: users = [] } = useQuery({
    queryKey: ["admin-users", token],
    queryFn: () => authApi.users(token!),
    enabled: Boolean(token),
  });

  const { data: applications = [] } = useQuery({
    queryKey: ["admin-applications", token],
    queryFn: () => authApi.adminApplications(token!),
    enabled: Boolean(token),
  });

  const createUser = useMutation({
    mutationFn: () => authApi.createUser(token!, form),
    onSuccess: () => {
      setForm({
        username: "",
        password: "auto-generated-123",
        role: "engineer",
        applications: [],
      });
      queryClient.invalidateQueries({ queryKey: ["admin-users"] });
    },
  });

  const createApplication = useMutation({
    mutationFn: () => authApi.createApplication(token!, appForm),
    onSuccess: () => {
      setAppForm({ name: "", displayName: "" });
      queryClient.invalidateQueries({ queryKey: ["admin-applications"] });
      queryClient.invalidateQueries({ queryKey: ["applications"] });
      queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
      queryClient.invalidateQueries({ queryKey: ["admin-users"] });
    },
  });

  const replaceApps = useMutation({
    mutationFn: ({ userId, apps }: { userId: string; apps: string[] }) =>
      authApi.replaceApplications(token!, userId, apps),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin-users"] }),
  });

  const visibleUsers = useMemo(
    () => users.filter((user) => user.username.toLowerCase().includes(search.toLowerCase())),
    [search, users],
  );

  const toggleFormApp = (app: string) => {
    setForm((current) => ({
      ...current,
      applications: current.applications.includes(app)
        ? current.applications.filter((item) => item !== app)
        : [...current.applications, app],
    }));
  };

  return (
    <DashboardLayout
      onRefresh={() => {
        queryClient.invalidateQueries({ queryKey: ["admin-users"] });
        queryClient.invalidateQueries({ queryKey: ["admin-applications"] });
      }}
    >
      <div className="grid grid-cols-[330px_minmax(0,1fr)] gap-6 p-6">
        <section className="space-y-4">
          <div className="border border-border bg-white p-6">
            <div className="mb-6 flex items-center gap-2 border-b border-border pb-3">
              <UserPlus className="h-5 w-5" />
              <h1 className="text-xl font-bold">Create User</h1>
            </div>

            <form
              onSubmit={(event) => {
                event.preventDefault();
                createUser.mutate();
              }}
              className="space-y-4"
            >
              <Field label="Username">
                <input
                  value={form.username}
                  onChange={(event) => setForm({ ...form, username: event.target.value })}
                  placeholder="e.g. jdoe"
                  className="h-8 w-full border border-input bg-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                />
              </Field>

              <Field label="Temporary Password">
                <input
                  value={form.password}
                  onChange={(event) => setForm({ ...form, password: event.target.value })}
                  className="h-8 w-full border border-input bg-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                />
              </Field>

              <Field label="Role Assignment">
                <select
                  value={form.role}
                  onChange={(event) => setForm({ ...form, role: event.target.value })}
                  className="h-8 w-full border border-input bg-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                >
                  <option value="engineer">Engineer</option>
                  <option value="admin">Sys Admin</option>
                </select>
              </Field>

              <div>
                <div className="mb-2 border-b border-border pb-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
                  Application Access
                </div>
                <div className="space-y-2">
                  {applications.map((app) => (
                    <label key={app} className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={form.applications.includes(app)}
                        onChange={() => toggleFormApp(app)}
                        className="h-4 w-4 accent-primary"
                      />
                      {app}
                    </label>
                  ))}
                </div>
              </div>

              {createUser.error && (
                <p className="text-xs font-semibold text-red-700">
                  {createUser.error instanceof Error ? createUser.error.message : "Unable to create user"}
                </p>
              )}

              <button
                type="submit"
                disabled={!form.username || !form.password || createUser.isPending}
                className="flex h-9 w-full items-center justify-center gap-2 bg-primary text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-60"
              >
                <Save className="h-4 w-4" />
                Save Permissions
              </button>
            </form>
          </div>

          <div className="border border-border bg-white p-6">
            <div className="mb-4 flex items-center gap-2 border-b border-border pb-3">
              <Boxes className="h-5 w-5" />
              <h2 className="text-lg font-bold">Create Application</h2>
            </div>
            <form
              onSubmit={(event) => {
                event.preventDefault();
                createApplication.mutate();
              }}
              className="space-y-3"
            >
              <Field label="Application Name">
                <input
                  value={appForm.name}
                  onChange={(event) => setAppForm({ ...appForm, name: event.target.value })}
                  placeholder="payment-service"
                  className="h-8 w-full border border-input bg-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                />
              </Field>
              <Field label="Display Name">
                <input
                  value={appForm.displayName}
                  onChange={(event) => setAppForm({ ...appForm, displayName: event.target.value })}
                  placeholder="Payment Service"
                  className="h-8 w-full border border-input bg-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                />
              </Field>
              {createApplication.error && (
                <p className="text-xs font-semibold text-red-700">
                  {createApplication.error instanceof Error
                    ? createApplication.error.message
                    : "Unable to create application"}
                </p>
              )}
              <button
                type="submit"
                disabled={!appForm.name || createApplication.isPending}
                className="flex h-9 w-full items-center justify-center gap-2 bg-primary text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-60"
              >
                <Boxes className="h-4 w-4" />
                Save Application
              </button>
            </form>
          </div>
        </section>

        <section className="border border-border bg-white">
          <div className="flex h-16 items-center justify-between border-b border-border px-4">
            <div className="flex items-center gap-2">
              <Users className="h-5 w-5" />
              <h2 className="text-xl font-bold">User Directory</h2>
            </div>
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Filter users..."
                className="h-8 w-64 border border-input bg-white pl-9 pr-3 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
              />
            </div>
          </div>

          <div>
            {visibleUsers.map((user) => (
              <UserRow
                key={user.id}
                user={user}
                applications={applications}
                saving={replaceApps.isPending}
                onApplicationsChange={(apps) =>
                  replaceApps.mutate({ userId: user.id, apps })
                }
              />
            ))}
          </div>
        </section>
      </div>
    </DashboardLayout>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs font-bold uppercase tracking-wide text-muted-foreground">
        {label}
      </span>
      {children}
    </label>
  );
}

function UserRow({
  user,
  applications,
  saving,
  onApplicationsChange,
}: {
  user: User;
  applications: string[];
  saving: boolean;
  onApplicationsChange: (apps: string[]) => void;
}) {
  const initials = user.username
    .split(/[.-]/)
    .map((part) => part[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();

  const toggle = (app: string) => {
    const next = user.applications.includes(app)
      ? user.applications.filter((item) => item !== app)
      : [...user.applications, app];
    onApplicationsChange(next);
  };

  return (
    <div className="flex items-center gap-4 border-b border-border px-4 py-4">
      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-[#d7e7ff] text-sm font-bold text-muted-foreground">
        {initials}
      </div>
      <div className="min-w-[220px]">
        <p className="text-sm font-medium">{user.username}</p>
        <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
          <span
            className={cn(
              "px-2 py-0.5 font-bold",
              user.role === "admin" ? "bg-[#d7e7ff] text-primary" : "bg-secondary text-muted-foreground",
            )}
          >
            {user.role === "admin" ? "Sys Admin" : "Engineer"}
          </span>
          <span>|</span>
          <span>{user.role === "admin" ? "All Apps" : user.applications.join(", ") || "No Apps"}</span>
        </div>
      </div>

      {user.role !== "admin" && (
        <div className="ml-auto flex flex-wrap justify-end gap-3">
          {applications.map((app) => (
            <label key={app} className="flex items-center gap-1 text-xs">
              <input
                type="checkbox"
                disabled={saving}
                checked={user.applications.includes(app)}
                onChange={() => toggle(app)}
                className="h-3.5 w-3.5 accent-primary"
              />
              {app}
            </label>
          ))}
        </div>
      )}
    </div>
  );
}
