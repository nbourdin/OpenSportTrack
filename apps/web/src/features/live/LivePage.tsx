import {useEffect, useRef, useState} from 'react';
import {connectLive, type ConnectionPhase} from './liveConnection';
import {createLiveMap} from './liveMap';
import {durationText, emptyMetrics, paceText, type Metrics} from './track';

const phaseText: Record<ConnectionPhase, string> = {
  connecting: 'Connecting…',
  live: 'Live',
  reconnecting: 'Reconnecting…',
  not_found: 'Activity not found',
  unavailable: 'Server unavailable',
};

export function LivePage({id}: {id: string}) {
  const mapNode = useRef<HTMLDivElement>(null);
  const [phase, setPhase] = useState<ConnectionPhase>('connecting');
  const [metrics, setMetrics] = useState<Metrics>(emptyMetrics);

  useEffect(() => {
    if (!mapNode.current) return;
    const liveMap = createLiveMap(mapNode.current, setMetrics);
    const disconnect = connectLive(id, liveMap.apply, nextPhase => {
      if (nextPhase === 'not_found') liveMap.reset();
      setPhase(nextPhase);
    });
    return () => {
      disconnect();
      liveMap.dispose();
    };
  }, [id]);

  return (
    <>
      <header>
        <div>
          <strong>OpenSportTrack</strong>
          <span className="separator">/</span>
          <span>Live tracking</span>
          <a className="header-link" href="/simulator">
            Simulate a GPX file
          </a>
        </div>
        <output id="status" className={phase === 'live' ? 'connected' : ''}>
          {phaseText[phase]}
        </output>
      </header>
      <main>
        <section className="metrics" aria-label="Activity metrics">
          <div>
            <span>Distance</span>
            <strong id="distance">
              {(metrics.distance / 1000).toLocaleString('en-US', {
                minimumFractionDigits: 2,
                maximumFractionDigits: 2,
              })}{' '}
              km
            </strong>
          </div>
          <div>
            <span>Duration</span>
            <strong id="duration">{durationText(metrics.duration)}</strong>
          </div>
          <div>
            <span>Speed</span>
            <strong id="speed">
              {metrics.speed === null
                ? '—'
                : `${metrics.speed.toLocaleString('en-US', {maximumFractionDigits: 1})} km/h`}
            </strong>
          </div>
          <div>
            <span>Pace</span>
            <strong id="pace">{paceText(metrics.pace)}</strong>
          </div>
        </section>
        <section id="map" ref={mapNode} aria-label="Route map" />
      </main>
      <footer>
        Activity <code id="activity-id">{id}</code>
      </footer>
    </>
  );
}
