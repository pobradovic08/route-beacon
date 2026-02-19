import { useCallback, useRef } from "react";

interface SSECallbacks<T extends Record<string, unknown>> {
  onEvent: (eventType: string, data: T[keyof T]) => void;
  onError: (error: string) => void;
  onComplete: () => void;
}

/**
 * Reads an SSE stream from a fetch Response.
 * Returns a start function that kicks off parsing and an abort function.
 */
export function useSSE<T extends Record<string, unknown>>() {
  const abortRef = useRef<AbortController | null>(null);

  const abort = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
  }, []);

  const start = useCallback(
    async (
      fetcher: (signal: AbortSignal) => Promise<Response>,
      callbacks: SSECallbacks<T>,
    ) => {
      abort();
      const controller = new AbortController();
      abortRef.current = controller;

      let res: Response;
      try {
        res = await fetcher(controller.signal);
      } catch (err: unknown) {
        if (controller.signal.aborted) return;
        callbacks.onError(
          err instanceof Error ? err.message : "Network error",
        );
        return;
      }

      if (!res.ok) {
        let detail = `HTTP ${res.status}`;
        try {
          const body = await res.json();
          detail = body.detail || body.title || detail;
        } catch {
          /* keep default */
        }
        callbacks.onError(detail);
        return;
      }

      const reader = res.body?.getReader();
      if (!reader) {
        callbacks.onError("No response body");
        return;
      }

      const decoder = new TextDecoder();
      let buffer = "";
      let currentEvent = "";

      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() || "";

          for (const line of lines) {
            if (line.startsWith("event:")) {
              currentEvent = line.slice(6).trim();
            } else if (line.startsWith("data:")) {
              const raw = line.slice(5).trim();
              if (!raw) continue;
              try {
                const data = JSON.parse(raw);
                if (currentEvent === "complete") {
                  callbacks.onComplete();
                } else if (currentEvent === "error") {
                  callbacks.onError(data.message || data.code || "Unknown");
                } else if (currentEvent) {
                  callbacks.onEvent(currentEvent, data);
                }
              } catch {
                /* skip malformed JSON */
              }
              currentEvent = "";
            }
          }
        }
      } catch (err: unknown) {
        if (!controller.signal.aborted) {
          callbacks.onError(
            err instanceof Error ? err.message : "Stream error",
          );
        }
      }
    },
    [abort],
  );

  return { start, abort };
}
