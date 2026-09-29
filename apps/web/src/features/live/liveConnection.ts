import {api, liveURL} from '../../shared/api/client';
import {liveMessageSchema} from '../../shared/api/schemas';
import type {LiveMessage} from '../../shared/api/types';

export type ConnectionPhase = 'connecting' | 'live' | 'reconnecting' | 'not_found' | 'unavailable';

export function connectLive(
  id: string,
  onMessage: (message: LiveMessage) => void,
  onPhase: (phase: ConnectionPhase) => void,
): () => void {
  const controller = new AbortController();
  let socket: WebSocket | null = null;
  let retryTimer: ReturnType<typeof setTimeout> | undefined;
  let retries = 0;
  let openedAt = 0;
  let hasConnected = false;

  function retry() {
    if (controller.signal.aborted) return;
    onPhase(hasConnected ? 'reconnecting' : 'unavailable');
    // Cap exponential backoff while keeping brief outages responsive.
    const delay = Math.min(10000, 500 * 2 ** Math.min(retries++, 5));
    retryTimer = setTimeout(() => {
      void open();
    }, delay);
  }

  async function open() {
    if (controller.signal.aborted) return;
    try {
      // Resolve 404 before opening the socket so a missing activity does not retry forever.
      const activity = await api.getActivity(id, controller.signal);
      if (controller.signal.aborted) return;
      if (activity === null) {
        onPhase('not_found');
        return;
      }

      const connection = new WebSocket(liveURL(id));
      socket = connection;
      connection.onopen = () => {
        if (controller.signal.aborted) return;
        openedAt = performance.now();
        hasConnected = true;
        onPhase('live');
      };
      connection.onmessage = event => {
        try {
          const value: unknown = JSON.parse(event.data);
          // Parse untrusted WebSocket data before passing it to the map.
          const parsed = liveMessageSchema.safeParse(value);
          if (parsed.success && parsed.data.activity_id === id) onMessage(parsed.data);
          else connection.close();
        } catch {
          connection.close();
        }
      };
      connection.onclose = () => {
        // Reset backoff only after a stable connection, not every brief upgrade.
        if (openedAt && performance.now() - openedAt > 30000) retries = 0;
        openedAt = 0;
        if (!controller.signal.aborted) retry();
      };
    } catch {
      if (!controller.signal.aborted) retry();
    }
  }

  onPhase('connecting');
  void open();
  return () => {
    controller.abort();
    if (retryTimer !== undefined) clearTimeout(retryTimer);
    socket?.close();
  };
}
