import {liveURL} from '../../shared/api/client';
import type {LiveMessage, Position, Sample} from '../../shared/api/types';

export type ConnectionPhase = 'connecting' | 'live' | 'reconnecting' | 'not_found' | 'unavailable';

function isPosition(value: unknown): value is Position {
  if (!value || typeof value !== 'object') return false;
  const position = value as Record<string, unknown>;
  return (
    typeof position.latitude === 'number' &&
    Number.isFinite(position.latitude) &&
    Math.abs(position.latitude) <= 90 &&
    typeof position.longitude === 'number' &&
    Number.isFinite(position.longitude) &&
    Math.abs(position.longitude) <= 180 &&
    (position.altitude === undefined ||
      (typeof position.altitude === 'number' && Number.isFinite(position.altitude)))
  );
}

function isSample(value: unknown): value is Sample {
  if (!value || typeof value !== 'object') return false;
  const sample = value as Record<string, unknown>;
  if (
    typeof sample.timestamp !== 'string' ||
    !Number.isFinite(Date.parse(sample.timestamp)) ||
    !isPosition(sample.position)
  )
    return false;
  if (sample.next === undefined) return true;
  if (!sample.next || typeof sample.next !== 'object') return false;
  const next = sample.next as Record<string, unknown>;
  return (
    typeof next.timestamp === 'string' &&
    Number.isFinite(Date.parse(next.timestamp)) &&
    isPosition(next.position) &&
    typeof next.after_ms === 'number' &&
    Number.isFinite(next.after_ms) &&
    next.after_ms > 0
  );
}

function isLiveMessage(value: unknown): value is LiveMessage {
  // Narrow untrusted WebSocket JSON before it reaches the typed map renderer.
  if (!value || typeof value !== 'object') return false;
  const message = value as Record<string, unknown>;
  if (message.version !== 1 || typeof message.activity_id !== 'string') return false;
  if (message.type === 'snapshot') {
    return (
      (message.samples === undefined ||
        (Array.isArray(message.samples) && message.samples.every(isSample))) &&
      (message.route === undefined ||
        (Array.isArray(message.route) && message.route.every(isPosition)))
    );
  }
  if (message.type === 'route')
    return Array.isArray(message.route) && message.route.every(isPosition);
  return message.type === 'sample' && isSample(message.sample);
}

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
    const delay = Math.min(10000, 500 * 2 ** Math.min(retries++, 5));
    retryTimer = setTimeout(() => {
      void open();
    }, delay);
  }

  async function open() {
    if (controller.signal.aborted) return;
    try {
      const response = await fetch(`/api/v1/activities/${encodeURIComponent(id)}`, {
        signal: controller.signal,
      });
      if (controller.signal.aborted) return;
      if (response.status === 404) {
        onPhase('not_found');
        return;
      }
      if (!response.ok) {
        retry();
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
          if (isLiveMessage(value) && value.activity_id === id) onMessage(value);
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
