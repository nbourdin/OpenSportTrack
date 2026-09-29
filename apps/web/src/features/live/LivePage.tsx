import {useEffect, useRef, useState} from 'react';
import {connectLive, type ConnectionPhase} from './liveConnection';
import {createLiveMap} from './liveMap';
import {durationText, emptyMetrics, paceText, type Metrics} from './track';

const phaseText: Record<ConnectionPhase, string> = {
  connecting: 'Connexion…',
  live: 'En direct',
  reconnecting: 'Reconnexion…',
  not_found: 'Activité introuvable',
  unavailable: 'Serveur indisponible',
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
            Simuler un GPX
          </a>
        </div>
        <output id="status" className={phase === 'live' ? 'connected' : ''}>
          {phaseText[phase]}
        </output>
      </header>
      <main>
        <section className="metrics" aria-label="Métriques de l'activité">
          <div>
            <span>Distance</span>
            <strong id="distance">
              {(metrics.distance / 1000).toLocaleString('fr-FR', {
                minimumFractionDigits: 2,
                maximumFractionDigits: 2,
              })}{' '}
              km
            </strong>
          </div>
          <div>
            <span>Durée</span>
            <strong id="duration">{durationText(metrics.duration)}</strong>
          </div>
          <div>
            <span>Vitesse</span>
            <strong id="speed">
              {metrics.speed === null
                ? '—'
                : `${metrics.speed.toLocaleString('fr-FR', {maximumFractionDigits: 1})} km/h`}
            </strong>
          </div>
          <div>
            <span>Allure</span>
            <strong id="pace">{paceText(metrics.pace)}</strong>
          </div>
        </section>
        <section id="map" ref={mapNode} aria-label="Carte du parcours" />
      </main>
      <footer>
        Activité <code id="activity-id">{id}</code>
      </footer>
    </>
  );
}
