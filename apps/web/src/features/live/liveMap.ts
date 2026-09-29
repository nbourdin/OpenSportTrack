import * as L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import type {LiveMessage, Position, Sample} from '../../shared/api/types';
import {emptyMetrics, haversine, metricsAt, type Metrics} from './track';

export function createLiveMap(node: HTMLDivElement, onMetrics: (metrics: Metrics) => void) {
  const map = L.map(node).setView([47.2184, -1.5536], 13);
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
  L.control
    .layers(
      undefined,
      {
        '<span class="legend-swatch planned"></span>Parcours initial': plannedRoute,
        '<span class="legend-swatch travelled"></span>Tracé parcouru': travelledRoute,
      },
      {collapsed: false, position: 'topright'},
    )
    .addTo(map);
  const marker = L.marker([47.2184, -1.5536], {
    icon: L.divIcon({className: 'current-position', iconSize: [16, 16]}),
  });
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const pending: {sample: Sample; duration: number}[] = [];
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
  let lastMetricsUpdate = 0;

  function publishMetrics(time: number, distance: number, force = false) {
    const now = performance.now();
    // Let Leaflet animate each frame without rerendering React at the same rate.
    if (!force && now - lastMetricsUpdate < 100) return;
    lastMetricsUpdate = now;
    onMetrics(metricsAt(firstTime, time, distance));
  }

  function setRoute(positions: Position[] = []) {
    const coordinates = positions.map(pos => L.latLng(pos.latitude, pos.longitude));
    plannedOutline.setLatLngs(coordinates);
    plannedLine.setLatLngs(coordinates);
    routeAvailable = coordinates.length > 0;
    if (coordinates.length > 1)
      map.fitBounds(plannedLine.getBounds(), {padding: [30, 30], animate: false});
    else if (routeAvailable) map.setView(coordinates[0], 15, {animate: false});
  }

  function moveMarker(location: L.LatLngExpression, first = false) {
    marker.setLatLng(location);
    if (!map.hasLayer(marker)) marker.addTo(map);
    if (first && !routeAvailable) map.setView(location, 15, {animate: false});
    else if (!map.getBounds().contains(L.latLng(location))) map.panTo(location, {animate: false});
  }

  function addSample(sample: Sample): L.LatLngTuple {
    const position = sample.position;
    const time = Date.parse(sample.timestamp);
    if (lastPosition) distanceMeters += haversine(lastPosition, position);
    if (firstTime === null) firstTime = time;
    lastTime = time;
    lastPosition = position;
    pointCount++;
    return [position.latitude, position.longitude];
  }

  function commit(sample: Sample) {
    const first = pointCount === 0;
    const location = addSample(sample);
    line.addLatLng(location);
    moveMarker(location, first);
    publishMetrics(lastTime!, distanceMeters, true);
  }

  function stopAnimations() {
    if (predictionFrame) cancelAnimationFrame(predictionFrame);
    if (fallbackFrame) cancelAnimationFrame(fallbackFrame);
    predictionFrame = 0;
    fallbackFrame = 0;
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
      publishMetrics(
        fromTime + (toTime - fromTime) * fraction,
        distanceMeters + segmentDistance * fraction,
      );
      predictionFrame = fraction < 1 ? requestAnimationFrame(frame) : 0;
    }
    predictionFrame = requestAnimationFrame(frame);
  }

  function animatePending() {
    if (fallbackFrame) return;
    // Commit excess queued points to keep the marker near live time after a burst.
    while (pending.length) {
      if (!reducedMotion && pending.length <= 20 && lastPosition) break;
      commit(pending.shift()!.sample);
    }
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
      publishMetrics(
        fromTime + (toTime - fromTime) * fraction,
        distanceMeters + segmentDistance * fraction,
      );
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
      stopAnimations();
      while (pending.length) commit(pending.shift()!.sample);
      commit(sample);
      predictNext(sample);
      return;
    }
    const arrived = performance.now();
    const duration =
      lastArrival === null ? 350 : Math.min(2000, Math.max(80, arrived - lastArrival));
    lastArrival = arrived;
    if (pointCount === 0 && !fallbackFrame) commit(sample);
    else {
      pending.push({sample, duration});
      animatePending();
    }
  }

  function snapshot(samples: Sample[], route: Position[] = []) {
    stopAnimations();
    pending.length = 0;
    firstTime = lastTime = lastArrival = null;
    lastPosition = null;
    distanceMeters = pointCount = 0;
    lastMetricsUpdate = 0;
    line.setLatLngs([]);
    setRoute(route);
    // Apply historical points in one map update instead of redrawing per point.
    const coordinates = samples.filter(sample => !!sample?.position).map(addSample);
    line.setLatLngs(coordinates);
    if (coordinates.length) {
      moveMarker(coordinates.at(-1)!);
      publishMetrics(lastTime!, distanceMeters, true);
    } else {
      marker.remove();
      onMetrics(emptyMetrics);
    }
    if (!routeAvailable && coordinates.length > 1)
      map.fitBounds(line.getBounds(), {padding: [30, 30], animate: false});
    lastArrival = performance.now();
    const latest = samples.at(-1);
    if (latest) predictNext(latest);
  }

  function apply(message: LiveMessage) {
    if (message.type === 'snapshot') snapshot(message.samples || [], message.route);
    else if (message.type === 'route') setRoute(message.route);
    else if (message.type === 'sample') receive(message.sample);
  }

  return {
    apply,
    reset: () => snapshot([]),
    dispose: () => {
      stopAnimations();
      map.remove();
    },
  };
}
