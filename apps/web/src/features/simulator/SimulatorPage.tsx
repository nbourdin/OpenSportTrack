import {useRef, useState, type FormEvent} from 'react';
import {replayStatus} from './replay';
import {useReplay} from './useReplay';
import './simulator.css';

export function SimulatorPage() {
  const [file, setFile] = useState<File | null>(null);
  const [interval, setIntervalValue] = useState(500);
  const intervalRef = useRef(interval);
  const {state, start, stop} = useReplay();
  const running =
    state.phase === 'reading' || state.phase === 'creating' || state.phase === 'running';

  function changeInterval(value: number) {
    intervalRef.current = value;
    setIntervalValue(value);
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (file) void start(file, () => intervalRef.current);
  }

  const viewerPath = state.activityID ? `/live/${encodeURIComponent(state.activityID)}` : '';
  return (
    <>
      <header>
        <div>
          <strong>OpenSportTrack</strong>
          <span className="separator">/</span>
          <span>GPX simulator</span>
        </div>
      </header>
      <main className="simulator-main">
        <div className="intro">
          <h1>Replay a route</h1>
          <p>
            Choose a GPX file and a sending interval. You can change the interval during the replay.
          </p>
        </div>
        <form id="simulator-form" className="controls" onSubmit={submit}>
          <label>
            GPX file
            <input
              id="gpx-file"
              type="file"
              accept=".gpx,application/gpx+xml,application/xml,text/xml"
              required
              disabled={running}
              onChange={event => setFile(event.target.files?.[0] ?? null)}
            />
          </label>
          <label>
            Time between points
            <select
              id="send-interval"
              value={interval}
              onChange={event => changeInterval(Number(event.target.value))}
            >
              <option value="250">250 ms</option>
              <option value="500">500 ms</option>
              <option value="1000">1 s</option>
              <option value="2000">2 s</option>
            </select>
          </label>
          <button id="start" type="submit" disabled={running}>
            Start
          </button>
          <button id="stop" type="button" disabled={!running} onClick={stop}>
            Stop
          </button>
        </form>
        <p className="hint">
          The selected interval controls sending; GPX timestamps remain unchanged for metrics.
        </p>
        <div className="run-info">
          <output id="simulator-status">{replayStatus(state)}</output>
          {state.total > 0 && <progress id="progress" max={state.total} value={state.sent} />}
          {state.activityID && (
            <a id="viewer-link" href={viewerPath} target="_blank" rel="noopener">
              Open live view ↗
            </a>
          )}
        </div>
        {state.activityID && (
          <iframe
            key={state.activityID}
            id="viewer-frame"
            src={viewerPath}
            title="Live route view"
          />
        )}
      </main>
    </>
  );
}
