import "./global.css";

import { AuthProvider } from "@/auth/AuthContext";
import { RequireAuth } from "@/auth/RequireAuth";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import Login from "./pages/Login";
import LiveLogs from "./pages/LiveLogs";
import Incidents from "./pages/Incidents";
import Health from "./pages/Health";
import AlertRules from "./pages/AlertRules";
import UserAccess from "./pages/UserAccess";
import Settings from "./pages/Settings";
import NotFound from "./pages/NotFound";

const queryClient = new QueryClient();

const App = () => (
  <QueryClientProvider client={queryClient}>
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route element={<RequireAuth />}>
            <Route path="/logs" element={<LiveLogs />} />
            <Route path="/live-logs" element={<Navigate to="/logs" replace />} />
            <Route path="/incidents" element={<Incidents />} />
            <Route path="/health" element={<Health />} />
            <Route path="/settings" element={<Settings />} />
          </Route>
          <Route element={<RequireAuth adminOnly />}>
            <Route path="/alert-rules" element={<AlertRules />} />
            <Route path="/users" element={<UserAccess />} />
            <Route path="/user-access" element={<Navigate to="/users" replace />} />
          </Route>
          <Route path="/" element={<Navigate to="/logs" replace />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  </QueryClientProvider>
);

createRoot(document.getElementById("root")!).render(<App />);
