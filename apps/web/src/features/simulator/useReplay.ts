import {useEffect, useRef, useState} from 'react';
import {initialReplayState, replayGPX, type ReplayState} from './replay';

export function useReplay() {
  const [state, setState] = useState<ReplayState>(initialReplayState);
  const controller = useRef<AbortController | null>(null);
  const latestState = useRef(state);
  const disposed = useRef(false);

  useEffect(() => {
    disposed.current = false;
    // Cancel outstanding work when the page unmounts and ignore its late callbacks.
    return () => {
      disposed.current = true;
      controller.current?.abort();
    };
  }, []);

  function update(next: ReplayState) {
    // Keep the latest progress available to the async error handler before React rerenders.
    latestState.current = next;
    if (!disposed.current) setState(next);
  }

  async function start(file: File, getInterval: () => number) {
    if (controller.current) return;
    const abort = new AbortController();
    controller.current = abort;
    update(initialReplayState);
    try {
      await replayGPX(file, getInterval, abort.signal, update);
    } catch (error) {
      const stopped = error instanceof DOMException && error.name === 'AbortError';
      update({
        ...latestState.current,
        phase: stopped ? 'stopped' : 'error',
        error: stopped ? undefined : error instanceof Error ? error.message : String(error),
      });
    } finally {
      controller.current = null;
    }
  }

  return {state, start, stop: () => controller.current?.abort()};
}
