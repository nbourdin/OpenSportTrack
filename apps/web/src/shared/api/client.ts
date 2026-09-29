import ky from 'ky';
import {activitySchema} from './schemas';
import type {Position, Sample} from './types';

const http = ky.create({
  // The simulator owns its send cadence; implicit retries would delay subsequent points.
  retry: {limit: 0},
  timeout: false,
  throwHttpErrors: false,
});

async function checked(response: Response): Promise<Response> {
  // Preserve server error details after callers handle expected statuses such as 404.
  if (response.ok) return response;
  const detail = (await response.text()).trim();
  throw new Error(`${response.status}: ${detail || response.statusText}`);
}

function activityPath(id: string): string {
  return `/api/v1/activities/${encodeURIComponent(id)}`;
}

export const api = {
  async createActivity(signal: AbortSignal) {
    const response = await checked(
      await http.post('/api/v1/activities', {json: {sport: 'running'}, signal}),
    );
    return activitySchema.parse(await response.json());
  },
  async getActivity(id: string, signal: AbortSignal) {
    const response = await http.get(activityPath(id), {signal});
    if (response.status === 404) return null;
    return activitySchema.parse(await (await checked(response)).json());
  },
  async setRoute(id: string, positions: Position[], signal: AbortSignal) {
    await checked(await http.put(`${activityPath(id)}/route`, {json: {positions}, signal}));
  },
  async sendSample(id: string, sample: Sample, signal: AbortSignal) {
    await checked(await http.post(`${activityPath(id)}/samples`, {json: sample, signal}));
  },
};

export function liveURL(id: string): string {
  const url = new URL(`${activityPath(id)}/live`, location.href);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}
