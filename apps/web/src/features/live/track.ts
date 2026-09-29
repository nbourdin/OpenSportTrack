import type {Position} from '../../shared/api/types';

export type Metrics = {
  distance: number;
  duration: number;
  speed: number | null;
  pace: number | null;
};
export const emptyMetrics: Metrics = {distance: 0, duration: 0, speed: null, pace: null};

export function haversine(a: Position, b: Position): number {
  const rad = Math.PI / 180;
  const dLat = (b.latitude - a.latitude) * rad;
  const dLon = (b.longitude - a.longitude) * rad;
  const h =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(a.latitude * rad) * Math.cos(b.latitude * rad) * Math.sin(dLon / 2) ** 2;
  return 2 * 6371000 * Math.asin(Math.sqrt(h));
}

export function metricsAt(firstTime: number | null, time: number, distance: number): Metrics {
  const seconds = Math.max(0, (time - (firstTime ?? time)) / 1000);
  return {
    distance,
    duration: seconds,
    speed: seconds > 0 && distance > 0 ? (distance / seconds) * 3.6 : null,
    pace: seconds > 0 && distance > 0 ? seconds / (distance / 1000) : null,
  };
}

export function durationText(seconds: number): string {
  const value = Math.floor(seconds);
  return `${String(Math.floor(value / 3600)).padStart(2, '0')}:${String(Math.floor(value / 60) % 60).padStart(2, '0')}:${String(value % 60).padStart(2, '0')}`;
}

export function paceText(pace: number | null): string {
  if (pace === null) return '—';
  return `${Math.floor(pace / 60)}:${String(Math.floor(pace % 60)).padStart(2, '0')}/km`;
}
