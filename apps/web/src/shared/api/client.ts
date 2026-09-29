import type {Activity, Position, Sample} from './types';

async function request<T>(
  method: string,
  path: string,
  body: unknown,
  signal: AbortSignal,
): Promise<T> {
  const response = await fetch(path, {
    method,
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(body),
    signal,
  });
  if (!response.ok) {
    const detail = (await response.text()).trim();
    throw new Error(`${response.status} : ${detail || response.statusText}`);
  }
  return response.status === 204 ? (undefined as T) : (response.json() as Promise<T>);
}

export const api = {
  createActivity: (signal: AbortSignal) =>
    request<Activity>('POST', '/api/v1/activities', {sport: 'running'}, signal),
  setRoute: (id: string, positions: Position[], signal: AbortSignal) =>
    request<void>('PUT', `/api/v1/activities/${encodeURIComponent(id)}/route`, {positions}, signal),
  sendSample: (id: string, sample: Sample, signal: AbortSignal) =>
    request<void>('POST', `/api/v1/activities/${encodeURIComponent(id)}/samples`, sample, signal),
};

export function liveURL(id: string): string {
  const url = new URL(`/api/v1/activities/${encodeURIComponent(id)}/live`, location.href);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}
