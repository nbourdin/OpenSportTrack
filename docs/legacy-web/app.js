(() => {
  const id = decodeURIComponent(location.pathname.split('/').pop());
  document.getElementById('activity-id').textContent = id;
  const status = document.getElementById('status');
  if (!window.L) {
    status.textContent = 'Carte indisponible : Leaflet n’a pas pu être chargé';
    return;
  }
  const map = L.map('map').setView([47.2184, -1.5536], 13);
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
  L.control.layers(null, {
    '<span class="legend-swatch planned"></span>Parcours initial': plannedRoute,
    '<span class="legend-swatch travelled"></span>Tracé parcouru': travelledRoute,
  }, {collapsed: false, position: 'topright'}).addTo(map);
  const marker = L.marker([47.2184, -1.5536], {
    icon: L.divIcon({className: 'current-position', iconSize: [16, 16]}),
  });
  let firstTime = null;
  let lastTime = null;
  let lastPosition = null;
  let distanceMeters = 0;
  let pointCount = 0;
  let routeAvailable = false;
  let lastArrival = null;
  let animating = false;
  let predicting = false;
  let predictionFrame = 0;
  const pending = [];
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  function setRoute(positions) {
    const coordinates = (positions || []).map(pos => [pos.latitude, pos.longitude]);
    plannedOutline.setLatLngs(coordinates);
    plannedLine.setLatLngs(coordinates);
    routeAvailable = plannedLine.getLatLngs().length > 0;
    if (plannedLine.getLatLngs().length > 1) {
      map.fitBounds(plannedLine.getBounds(), {padding: [30, 30], animate: false});
    } else if (routeAvailable) {
      map.setView(plannedLine.getLatLngs()[0], 15, {animate: false});
    }
  }

  function haversine(a, b) {
    const rad = Math.PI / 180;
    const dLat = (b.latitude - a.latitude) * rad;
    const dLon = (b.longitude - a.longitude) * rad;
    const h = Math.sin(dLat / 2) ** 2 + Math.cos(a.latitude * rad) *
      Math.cos(b.latitude * rad) * Math.sin(dLon / 2) ** 2;
    return 2 * 6371000 * Math.asin(Math.sqrt(h));
  }

  function moveMarker(location, first = false) {
    marker.setLatLng(location);
    if (!map.hasLayer(marker)) marker.addTo(map);
    if (first && !routeAvailable) map.setView(location, 15, {animate: false});
    else if (!map.getBounds().contains(location)) map.panTo(location, {animate: false});
  }

  function renderMetrics(time, distance) {
    const seconds = Math.max(0, (time - firstTime) / 1000);
    const minutes = Math.floor(seconds / 60);
    document.getElementById('distance').textContent = (distance / 1000).toLocaleString('fr-FR', {minimumFractionDigits: 2, maximumFractionDigits: 2}) + ' km';
    document.getElementById('duration').textContent = `${String(Math.floor(minutes / 60)).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`;
    if (seconds > 0 && distance > 0) {
      document.getElementById('speed').textContent = (distance / seconds * 3.6).toLocaleString('fr-FR', {maximumFractionDigits: 1}) + ' km/h';
      const pace = seconds / (distance / 1000);
      document.getElementById('pace').textContent = `${Math.floor(pace / 60)}:${String(Math.floor(pace % 60)).padStart(2, '0')}/km`;
    }
  }

  function commit(sample) {
    if (!sample?.position) return;
    const first = pointCount === 0;
    const pos = sample.position;
    const time = new Date(sample.timestamp).getTime();
    if (lastPosition) distanceMeters += haversine(lastPosition, pos);
    if (firstTime === null) firstTime = time;
    lastTime = time;
    lastPosition = pos;
    pointCount++;
    const location = [pos.latitude, pos.longitude];
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

  // Replays know the next point in advance. Move toward it during the actual
  // send interval, then confirm the point when it arrives over WebSocket.
  function predictNext(sample) {
    if (!sample.next || reducedMotion) return;
    const from = sample.position;
    const to = sample.next.position;
    const fromTime = new Date(sample.timestamp).getTime();
    const toTime = new Date(sample.next.timestamp).getTime();
    const segmentDistance = haversine(from, to);
    const duration = sample.next.after_ms;
    const started = performance.now();
    predicting = true;

    function frame(now) {
      if (!predicting) return;
      const fraction = Math.min(1, (now - started) / duration);
      const location = [
        from.latitude + (to.latitude - from.latitude) * fraction,
        from.longitude + (to.longitude - from.longitude) * fraction,
      ];
      activeSegment.setLatLngs([[from.latitude, from.longitude], location]);
      moveMarker(location);
      renderMetrics(fromTime + (toTime - fromTime) * fraction,
        distanceMeters + segmentDistance * fraction);
      if (fraction < 1) predictionFrame = requestAnimationFrame(frame);
      else predictionFrame = 0;
    }
    predictionFrame = requestAnimationFrame(frame);
  }

  function animateNext() {
    if (animating) return;
    while (pending.length && (reducedMotion || pending.length > 20 || !lastPosition)) {
      commit(pending.shift().sample);
    }
    if (pending.length === 0) return;
    const {sample, duration} = pending.shift();
    animating = true;
    const from = lastPosition;
    const to = sample.position;
    const fromTime = lastTime;
    const toTime = new Date(sample.timestamp).getTime();
    const segmentDistance = haversine(from, to);
    const started = performance.now();

    function frame(now) {
      const fraction = Math.min(1, (now - started) / duration);
      const latitude = from.latitude + (to.latitude - from.latitude) * fraction;
      const longitude = from.longitude + (to.longitude - from.longitude) * fraction;
      const location = [latitude, longitude];
      activeSegment.setLatLngs([[from.latitude, from.longitude], location]);
      moveMarker(location);
      renderMetrics(fromTime + (toTime - fromTime) * fraction,
        distanceMeters + segmentDistance * fraction);
      if (fraction < 1) {
        requestAnimationFrame(frame);
      } else {
        activeSegment.setLatLngs([]);
        commit(sample);
        animating = false;
        animateNext();
      }
    }
    requestAnimationFrame(frame);
  }

  function receive(sample) {
    if (!sample?.position) return;
    if (predicting || sample.next) {
      stopPrediction();
      while (pending.length) commit(pending.shift().sample);
      commit(sample);
      predictNext(sample);
      return;
    }
    const arrived = performance.now();
    const duration = lastArrival === null ? 350 : Math.min(2000, Math.max(80, arrived - lastArrival));
    lastArrival = arrived;
    if (pointCount === 0 && !animating && pending.length === 0) {
      commit(sample);
      return;
    }
    pending.push({sample, duration});
    animateNext();
  }

  const scheme = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const socket = new WebSocket(`${scheme}//${location.host}/api/v1/activities/${encodeURIComponent(id)}/live`);
  socket.onopen = () => { status.textContent = 'En direct'; status.classList.add('connected'); };
  socket.onmessage = event => {
    const message = JSON.parse(event.data);
    if (message.version !== 1) return;
    if (message.type === 'snapshot') {
      setRoute(message.route);
      for (const sample of message.samples || []) commit(sample);
      lastArrival = performance.now();
      if (!routeAvailable && pointCount > 1) map.fitBounds(line.getBounds(), {padding: [30, 30], animate: false});
      const latest = message.samples?.at(-1);
      if (latest) predictNext(latest);
    } else if (message.type === 'route') {
      setRoute(message.route);
    } else if (message.type === 'sample') receive(message.sample);
  };
  socket.onclose = () => { status.textContent = 'Déconnecté'; status.classList.remove('connected'); };
  socket.onerror = () => { status.textContent = 'Erreur de connexion'; status.classList.remove('connected'); };
})();
