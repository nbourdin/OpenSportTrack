import {api} from '../../shared/api/client';
import {parseGPX} from './gpx';
import {sendScheduledSamples} from './replaySchedule';

export type ReplayState = {
  phase: 'idle' | 'reading' | 'creating' | 'running' | 'done' | 'stopped' | 'error';
  sent: number;
  total: number;
  activityID: string | null;
  error?: string;
};

export const initialReplayState: ReplayState = {phase: 'idle', sent: 0, total: 0, activityID: null};

export function replayStatus(state: ReplayState): string {
  switch (state.phase) {
    case 'idle':
      return 'Prêt à démarrer.';
    case 'reading':
      return 'Lecture du GPX…';
    case 'creating':
      return `Création de l’activité (${state.total} points)…`;
    case 'running':
      return `Points envoyés : ${state.sent} / ${state.total}`;
    case 'done':
      return `Replay terminé : ${state.sent} points envoyés.`;
    case 'stopped':
      return 'Replay arrêté.';
    case 'error':
      return `Erreur : ${state.error}`;
  }
}

export async function replayGPX(
  file: File,
  getInterval: () => number,
  signal: AbortSignal,
  onUpdate: (state: ReplayState) => void,
): Promise<void> {
  onUpdate({...initialReplayState, phase: 'reading'});
  const samples = parseGPX(await file.text());
  if (signal.aborted) throw new DOMException('Arrêté', 'AbortError');
  onUpdate({phase: 'creating', sent: 0, total: samples.length, activityID: null});
  const activity = await api.createActivity(signal);
  await api.setRoute(
    activity.id,
    samples.map(sample => sample.position),
    signal,
  );
  let state: ReplayState = {
    phase: 'running',
    sent: 0,
    total: samples.length,
    activityID: activity.id,
  };
  onUpdate(state);
  await sendScheduledSamples(
    samples,
    getInterval,
    signal,
    sample => api.sendSample(activity.id, sample, signal),
    sent => {
      state = {...state, sent};
      onUpdate(state);
    },
  );
  onUpdate({...state, phase: 'done'});
}
