import {useEffect, useRef, useState} from 'react';
import * as L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import {liveURL} from '../../shared/api/client';
import type {LiveMessage, Position, Sample} from '../../shared/api/types';

type Metrics = {distance: number; duration: number; speed: number | null; pace: number | null};
const emptyMetrics: Metrics = {distance: 0, duration: 0, speed: null, pace: null};

function haversine(a: Position, b: Position): number {
  const rad = Math.PI / 180;
  const dLat = (b.latitude - a.latitude) * rad;
  const dLon = (b.longitude - a.longitude) * rad;
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(a.latitude * rad) *
    Math.cos(b.latitude * rad) * Math.sin(dLon / 2) ** 2;
  return 2 * 6371000 * Math.asin(Math.sqrt(h));
}

function durationText(seconds: number): string {
  const value = Math.floor(seconds);
  return `${String(Math.floor(value / 3600)).padStart(2, '0')}:${String(Math.floor(value / 60) % 60).padStart(2, '0')}:${String(value % 60).padStart(2, '0')}`;
}

function paceText(pace: number | null): string {
  if (pace === null) return '—';
  return `${Math.floor(pace / 60)}:${String(Math.floor(pace % 60)).padStart(2, '0')}/km`;
}

export function LivePage({id}: {id: string}) {
  const mapNode = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState('Connexion…');
  const [metrics, setMetrics] = useState<Metrics>(emptyMetrics);

  useEffect(() => {
    if (!mapNode.current) return;
    const map = L.map(mapNode.current).setView([47.2184, -1.5536], 13);
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors',
    }).addTo(map);
    const plannedOutline = L.polyline([], {color: '#526b61', weight: 8, opacity: 0.65});
    const plannedLine = L.polyline([], {color: '#e4f4eb', weight: 5, opacity: 1, dashArray: '9 8'});
    const plannedRoute = L.layerGroup([plannedOutline, plannedLine]).addTo(map);
    const line = L.polyline([], {color: '#0fa970', weight: 5, opacity: 1});
    const activeSegment = L.polyline([], {color: '#0fa970', weight: 5, opacity: 1});
    const travelledRoute = L.layerGroup([line, activeSegment]).addTo(map);
    L.control.layers(undefined, {
      '<span class="legend-swatch planned"></span>Parcours initial': plannedRoute,
      '<span class="legend-swatch travelled"></span>Tracé parcouru': travelledRoute,
    }, {collapsed: false, position: 'topright'}).addTo(map);
    const marker = L.marker([47.2184, -1.5536], {
      icon: L.divIcon({className: 'current-position', iconSize: [16, 16]}),
    });
    let firstTime: number | null = null;
    let lastTime: number | null = null;
    let lastPosition: Position | null = null;
    let distanceMeters = 0;
    let pointCount = 0;
    let routeAvailable = false;
    let lastArrival: number | null = null;
    let predicting = false;
    let predictionFrame = 0;
    let fallbackFrame = 0;
    const pending: {sample: Sample; duration: number}[] = [];
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    function setRoute(positions: Position[] = []) {
      const coordinates = positions.map(pos => L.latLng(pos.latitude, pos.longitude));
      plannedOutline.setLatLngs(coordinates);
      plannedLine.setLatLngs(coordinates);
      routeAvailable = coordinates.length > 0;
      if (coordinates.length > 1) map.fitBounds(plannedLine.getBounds(), {padding: [30, 30], animate: false});
      else if (routeAvailable) map.setView(coordinates[0], 15, {animate: false});
    }

    function moveMarker(location: L.LatLngExpression, first = false) {
      marker.setLatLng(location);
      if (!map.hasLayer(marker)) marker.addTo(map);
      if (first && !routeAvailable) map.setView(location, 15, {animate: false});
      else if (!map.getBounds().contains(L.latLng(location))) map.panTo(location, {animate: false});
    }

    function renderMetrics(time: number, distance: number) {
      const seconds = Math.max(0, (time - (firstTime ?? time)) / 1000);
      setMetrics({distance, duration: seconds,
        speed: seconds > 0 && distance > 0 ? distance / seconds * 3.6 : null,
        pace: seconds > 0 && distance > 0 ? seconds / (distance / 1000) : null});
    }

    function commit(sample: Sample) {
      if (!sample?.position) return;
      const first = pointCount === 0;
      const pos = sample.position;
      const time = Date.parse(sample.timestamp);
      if (lastPosition) distanceMeters += haversine(lastPosition, pos);
      if (firstTime === null) firstTime = time;
      lastTime = time;
      lastPosition = pos;
      pointCount++;
      const location: L.LatLngTuple = [pos.latitude, pos.longitude];
      line.addLatLng(location);
      moveMarker(location, first);
      renderMetrics(time, distanceMeters);
    }

    function stopPrediction() {
      if (predictionFrame) cancelAnimationFrame(predictionFrame);
      predictionFrame = 0;
      predicting = false;
      activeSegment.setLatLngs([]);
    }

    function predictNext(sample: Sample) {
      if (!sample.next || reducedMotion) return;
      const from = sample.position;
      const to = sample.next.position;
      const fromTime = Date.parse(sample.timestamp);
      const toTime = Date.parse(sample.next.timestamp);
      const segmentDistance = haversine(from, to);
      const duration = sample.next.after_ms;
      const started = performance.now();
      predicting = true;
      function frame(now: number) {
        if (!predicting) return;
        const fraction = Math.max(0, Math.min(1, (now - started) / duration));
        const location: L.LatLngTuple = [
          from.latitude + (to.latitude - from.latitude) * fraction,
          from.longitude + (to.longitude - from.longitude) * fraction,
        ];
        activeSegment.setLatLngs([[from.latitude, from.longitude], location]);
        moveMarker(location);
        renderMetrics(fromTime + (toTime - fromTime) * fraction,
          distanceMeters + segmentDistance * fraction);
        predictionFrame = fraction < 1 ? requestAnimationFrame(frame) : 0;
      }
      predictionFrame = requestAnimationFrame(frame);
    }

    function animatePending() {
      if (fallbackFrame) return;
      while (pending.length && (reducedMotion || pending.length > 20 || !lastPosition)) commit(pending.shift()!.sample);
      if (!pending.length) return;
      const {sample, duration} = pending.shift()!;
      const from = lastPosition!;
      const to = sample.position;
      const fromTime = lastTime!;
      const toTime = Date.parse(sample.timestamp);
      const segmentDistance = haversine(from, to);
      const started = performance.now();
      function frame(now: number) {
        const fraction = Math.max(0, Math.min(1, (now - started) / duration));
        const location: L.LatLngTuple = [
          from.latitude + (to.latitude - from.latitude) * fraction,
          from.longitude + (to.longitude - from.longitude) * fraction,
        ];
        activeSegment.setLatLngs([[from.latitude, from.longitude], location]);
        moveMarker(location);
        renderMetrics(fromTime + (toTime - fromTime) * fraction,
          distanceMeters + segmentDistance * fraction);
        if (fraction < 1) fallbackFrame = requestAnimationFrame(frame);
        else {
          fallbackFrame = 0;
          activeSegment.setLatLngs([]);
          commit(sample);
          animatePending();
        }
      }
      fallbackFrame = requestAnimationFrame(frame);
    }

    function receive(sample: Sample) {
      if (!sample?.position) return;
      if (predicting || sample.next) {
        stopPrediction();
        if (fallbackFrame) cancelAnimationFrame(fallbackFrame);
        fallbackFrame = 0;
        while (pending.length) commit(pending.shift()!.sample);
        commit(sample);
        predictNext(sample);
        return;
      }
      const arrived = performance.now();
      const duration = lastArrival === null ? 350 : Math.min(2000, Math.max(80, arrived - lastArrival));
      lastArrival = arrived;
      if (pointCount === 0 && !fallbackFrame) commit(sample);
      else { pending.push({sample, duration}); animatePending(); }
    }

    const socket = new WebSocket(liveURL(id));
    socket.onopen = () => setStatus('En direct');
    socket.onmessage = event => {
      const message = JSON.parse(event.data) as LiveMessage;
      if (message.version !== 1) return;
      if (message.type === 'snapshot') {
        setRoute(message.route);
        for (const sample of message.samples || []) commit(sample);
        lastArrival = performance.now();
        if (!routeAvailable && pointCount > 1) map.fitBounds(line.getBounds(), {padding: [30, 30], animate: false});
        const latest = message.samples?.at(-1);
        if (latest) predictNext(latest);
      } else if (message.type === 'route') setRoute(message.route);
      else if (message.type === 'sample') receive(message.sample);
    };
    socket.onclose = () => setStatus('Déconnecté');
    socket.onerror = () => setStatus('Erreur de connexion');
    return () => {
      socket.close();
      if (predictionFrame) cancelAnimationFrame(predictionFrame);
      if (fallbackFrame) cancelAnimationFrame(fallbackFrame);
      map.remove();
    };
  }, [id]);

  return <>
    <header>
      <div><strong>OpenSportTrack</strong><span className="separator">/</span><span>Live tracking</span><a className="header-link" href="/simulator">Simuler un GPX</a></div>
      <div id="status" className={status === 'En direct' ? 'connected' : ''} role="status">{status}</div>
    </header>
    <main>
      <section className="metrics" aria-label="Métriques de l'activité">
        <div><span>Distance</span><strong id="distance">{(metrics.distance / 1000).toLocaleString('fr-FR', {minimumFractionDigits: 2, maximumFractionDigits: 2})} km</strong></div>
        <div><span>Durée</span><strong id="duration">{durationText(metrics.duration)}</strong></div>
        <div><span>Vitesse</span><strong id="speed">{metrics.speed === null ? '—' : `${metrics.speed.toLocaleString('fr-FR', {maximumFractionDigits: 1})} km/h`}</strong></div>
        <div><span>Allure</span><strong id="pace">{paceText(metrics.pace)}</strong></div>
      </section>
      <section id="map" ref={mapNode} aria-label="Carte du parcours" />
    </main>
    <footer>Activité <code id="activity-id">{id}</code></footer>
  </>;
}
