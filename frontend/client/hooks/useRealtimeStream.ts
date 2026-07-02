import { realtimeUrl } from "@/api/client";
import type { AlertEvent, ProcessedLogEvent } from "@/api/types";
import { useEffect, useMemo, useRef, useState } from "react";

type StreamPayload = {
  logs: ProcessedLogEvent;
  alerts: AlertEvent;
};

interface RealtimeState<T> {
  events: T[];
  connected: boolean;
  reconnecting: boolean;
  lastMessageAt: Date | null;
}

export function useRealtimeStream<TStream extends keyof StreamPayload>(
  stream: TStream,
  token: string | null,
  filters: { app?: string[]; level?: string[] } = {},
  limit = 200,
  enabled = true,
): RealtimeState<StreamPayload[TStream]> {
  const [events, setEvents] = useState<StreamPayload[TStream][]>([]);
  const [connected, setConnected] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const [lastMessageAt, setLastMessageAt] = useState<Date | null>(null);
  const retryRef = useRef<number>();

  const url = useMemo(() => {
    if (!token) {
      return null;
    }
    return realtimeUrl(stream, token, filters);
  }, [filters.app?.join(","), filters.level?.join(","), stream, token]);

  useEffect(() => {
    if (!url || !enabled) {
      setConnected(false);
      setReconnecting(false);
      return;
    }

    let socket: WebSocket | null = null;
    let stopped = false;

    function connect() {
      socket = new WebSocket(url);

      socket.onopen = () => {
        if (stopped) {
          return;
        }
        setConnected(true);
        setReconnecting(false);
      };

      socket.onmessage = (message) => {
        if (stopped) {
          return;
        }
        const event = JSON.parse(message.data) as StreamPayload[TStream];
        setLastMessageAt(new Date());
        setEvents((current) => [event, ...current].slice(0, limit));
      };

      socket.onclose = () => {
        if (stopped) {
          return;
        }
        setConnected(false);
        setReconnecting(true);
        retryRef.current = window.setTimeout(connect, 1500);
      };

      socket.onerror = () => {
        socket?.close();
      };
    }

    connect();

    return () => {
      stopped = true;
      setConnected(false);
      setReconnecting(false);
      if (retryRef.current) {
        window.clearTimeout(retryRef.current);
      }
      socket?.close();
    };
  }, [enabled, limit, url]);

  return { events, connected, reconnecting, lastMessageAt };
}
