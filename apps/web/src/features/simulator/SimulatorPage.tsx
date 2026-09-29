import {useRef, useState, type FormEvent} from 'react';
import {api} from '../../shared/api/client';
import {parseGPX} from './gpx';
import './simulator.css';

function wait(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) return reject(new DOMException('Arrêté', 'AbortError'));
    const timer = setTimeout(() => { signal.removeEventListener('abort', abort); resolve(); }, ms);
    function abort() {
      clearTimeout(timer);
      reject(new DOMException('Arrêté', 'AbortError'));
    }
    signal.addEventListener('abort', abort, {once: true});
  });
}

export function SimulatorPage() {
  const [file, setFile] = useState<File | null>(null);
  const [interval, setIntervalValue] = useState(500);
  const intervalRef = useRef(interval);
  const controller = useRef<AbortController | null>(null);
  const [running, setRunning] = useState(false);
  const [status, setStatus] = useState('Prêt à démarrer.');
  const [progress, setProgress] = useState({value: 0, total: 0});
  const [activityID, setActivityID] = useState<string | null>(null);

  function changeInterval(value: number) {
    intervalRef.current = value;
    setIntervalValue(value);
  }

  async function start(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (controller.current || !file) return;
    const abort = new AbortController();
    controller.current = abort;
    const signal = abort.signal;
    setRunning(true);
    setActivityID(null);
    setProgress({value: 0, total: 0});
    setStatus('Lecture du GPX…');
    try {
      const samples = parseGPX(await file.text());
      if (signal.aborted) throw new DOMException('Arrêté', 'AbortError');
      setStatus(`Création de l’activité (${samples.length} points)…`);
      const activity = await api.createActivity(signal);
      await api.setRoute(activity.id, samples.map(sample => sample.position), signal);
      setActivityID(activity.id);
      setProgress({value: 0, total: samples.length});
      for (let index = 0; index < samples.length; index++) {
        const afterMS = intervalRef.current;
        const next = index + 1 < samples.length ? {
          timestamp: samples[index + 1].timestamp,
          position: samples[index + 1].position,
          after_ms: afterMS,
        } : undefined;
        await api.sendSample(activity.id, {...samples[index], next}, signal);
        setProgress({value: index + 1, total: samples.length});
        setStatus(`Points envoyés : ${index + 1} / ${samples.length}`);
        if (next) await wait(afterMS, signal);
      }
      setStatus(`Replay terminé : ${samples.length} points envoyés.`);
    } catch (error) {
      setStatus(error instanceof DOMException && error.name === 'AbortError'
        ? 'Replay arrêté.' : `Erreur : ${error instanceof Error ? error.message : String(error)}`);
    } finally {
      controller.current = null;
      setRunning(false);
    }
  }

  const viewerPath = activityID ? `/live/${encodeURIComponent(activityID)}` : '';
  return <>
    <header><div><strong>OpenSportTrack</strong><span className="separator">/</span><span>Simulateur GPX</span></div></header>
    <main className="simulator-main">
      <div className="intro">
        <h1>Rejouer un parcours</h1>
        <p>Choisis un fichier GPX puis l'intervalle d'envoi. Tu peux changer l'intervalle pendant la simulation.</p>
      </div>
      <form id="simulator-form" className="controls" onSubmit={start}>
        <label>Fichier GPX
          <input id="gpx-file" type="file" accept=".gpx,application/gpx+xml,application/xml,text/xml" required
            disabled={running} onChange={event => setFile(event.target.files?.[0] ?? null)} />
        </label>
        <label>Intervalle entre les points
          <select id="send-interval" value={interval} onChange={event => changeInterval(Number(event.target.value))}>
            <option value="250">250 ms</option><option value="500">500 ms</option>
            <option value="1000">1 s</option><option value="2000">2 s</option>
          </select>
        </label>
        <button id="start" type="submit" disabled={running}>Démarrer</button>
        <button id="stop" type="button" disabled={!running} onClick={() => controller.current?.abort()}>Arrêter</button>
      </form>
      <p className="hint">La cadence est fixe ; les horodatages GPX restent inchangés pour les métriques.</p>
      <div className="run-info">
        <span id="simulator-status" role="status">{status}</span>
        {progress.total > 0 && <progress id="progress" max={progress.total} value={progress.value} />}
        {activityID && <a id="viewer-link" href={viewerPath} target="_blank" rel="noopener">Ouvrir la vue live ↗</a>}
      </div>
      {activityID && <iframe key={activityID} id="viewer-frame" src={viewerPath} title="Vue live du parcours" />}
    </main>
  </>;
}
