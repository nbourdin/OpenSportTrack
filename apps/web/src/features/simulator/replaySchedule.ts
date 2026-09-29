import type {Sample} from '../../shared/api/types';

type Clock = {
  now: () => number;
  waitUntil: (deadline: number, signal: AbortSignal) => Promise<void>;
};

const realClock: Clock = {
  now: () => performance.now(),
  waitUntil: (deadline, signal) =>
    new Promise((resolve, reject) => {
      if (signal.aborted) return reject(new DOMException('Stopped', 'AbortError'));
      const timer = setTimeout(
        () => {
          signal.removeEventListener('abort', abort);
          resolve();
        },
        Math.max(0, deadline - performance.now()),
      );
      function abort() {
        clearTimeout(timer);
        reject(new DOMException('Stopped', 'AbortError'));
      }
      signal.addEventListener('abort', abort, {once: true});
    }),
};

export async function sendScheduledSamples(
  samples: Sample[],
  getInterval: () => number,
  signal: AbortSignal,
  send: (sample: Sample) => Promise<void>,
  onSent: (count: number) => void,
  clock: Clock = realClock,
): Promise<void> {
  // Cumulative deadlines absorb request latency instead of adding it to each interval.
  let nextSendAt = clock.now();
  for (let index = 0; index < samples.length; index++) {
    if (signal.aborted) throw new DOMException('Stopped', 'AbortError');
    const afterMS = getInterval();
    // Read the current interval on every point so changes affect the next send.
    const next =
      index + 1 < samples.length
        ? {
            timestamp: samples[index + 1].timestamp,
            position: samples[index + 1].position,
            after_ms: afterMS,
          }
        : undefined;
    await send({...samples[index], next});
    onSent(index + 1);
    if (next) {
      nextSendAt += afterMS;
      await clock.waitUntil(nextSendAt, signal);
    }
  }
}
