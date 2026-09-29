(() => {
  const form = document.getElementById('simulator-form');
  const fileInput = document.getElementById('gpx-file');
  const intervalInput = document.getElementById('send-interval');
  const startButton = document.getElementById('start');
  const stopButton = document.getElementById('stop');
  const status = document.getElementById('simulator-status');
  const progress = document.getElementById('progress');
  const viewerLink = document.getElementById('viewer-link');
  const viewerFrame = document.getElementById('viewer-frame');
  let controller = null;

  function parseGPX(source) {
    const document = new DOMParser().parseFromString(source, 'application/xml');
    if (document.querySelector('parsererror')) throw new Error('Le fichier GPX est mal formé.');
    const nodes = document.getElementsByTagNameNS('*', 'trkpt');
    if (nodes.length === 0) throw new Error('Le GPX ne contient aucun point de trace.');
    if (nodes.length > 50000) throw new Error('Le GPX dépasse la limite de 50 000 points.');
    const samples = [];
    let previousTime = null;
    for (const node of nodes) {
      const latitude = Number(node.getAttribute('lat'));
      const longitude = Number(node.getAttribute('lon'));
      const timeNode = [...node.children].find(child => child.localName === 'time');
      const elevationNode = [...node.children].find(child => child.localName === 'ele');
      const timestamp = Date.parse(timeNode?.textContent?.trim() || '');
      const altitude = elevationNode ? Number(elevationNode.textContent.trim()) : 0;
      if (!node.hasAttribute('lat') || !node.hasAttribute('lon') ||
          !Number.isFinite(latitude) || !Number.isFinite(longitude) || !Number.isFinite(altitude) ||
          Math.abs(latitude) > 90 || Math.abs(longitude) > 180 || !Number.isFinite(timestamp)) {
        throw new Error(`Point GPX invalide à la ligne ${samples.length + 1}.`);
      }
      if (previousTime !== null && timestamp < previousTime) {
        throw new Error('Les horodatages GPX doivent être dans l’ordre.');
      }
      samples.push({timestamp: new Date(timestamp).toISOString(),
        position: {latitude, longitude, altitude}});
      previousTime = timestamp;
    }
    return samples;
  }

  async function api(method, path, body, signal) {
    const response = await fetch(path, {
      method,
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body),
      signal,
    });
    if (!response.ok) {
      const detail = (await response.text()).trim();
      throw new Error(`${response.status} : ${detail || response.statusText}`);
    }
    return response.status === 204 ? null : response.json();
  }

  function wait(ms, signal) {
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

  stopButton.addEventListener('click', () => controller?.abort());
  form.addEventListener('submit', async event => {
    event.preventDefault();
    if (controller || !fileInput.files?.length) return;
    controller = new AbortController();
    const signal = controller.signal;
    startButton.disabled = true;
    stopButton.disabled = false;
    fileInput.disabled = true;
    viewerLink.hidden = true;
    viewerFrame.hidden = true;
    progress.hidden = true;
    status.textContent = 'Lecture du GPX…';

    try {
      const samples = parseGPX(await fileInput.files[0].text());
      if (signal.aborted) throw new DOMException('Arrêté', 'AbortError');
      status.textContent = `Création de l’activité (${samples.length} points)…`;
      const activity = await api('POST', '/api/v1/activities', {sport: 'running'}, signal);
      await api('PUT', `/api/v1/activities/${activity.id}/route`,
        {positions: samples.map(sample => sample.position)}, signal);
      const url = `/live/${encodeURIComponent(activity.id)}`;
      viewerLink.href = url;
      viewerLink.hidden = false;
      viewerFrame.src = url;
      viewerFrame.hidden = false;
      progress.max = samples.length;
      progress.value = 0;
      progress.hidden = false;

      for (let index = 0; index < samples.length; index++) {
        const afterMS = Number(intervalInput.value);
        const next = index + 1 < samples.length ? {
          timestamp: samples[index + 1].timestamp,
          position: samples[index + 1].position,
          after_ms: afterMS,
        } : undefined;
        await api('POST', `/api/v1/activities/${activity.id}/samples`, {...samples[index], next}, signal);
        progress.value = index + 1;
        status.textContent = `Points envoyés : ${index + 1} / ${samples.length}`;
        if (next) await wait(afterMS, signal);
      }
      status.textContent = `Replay terminé : ${samples.length} points envoyés.`;
    } catch (error) {
      status.textContent = error.name === 'AbortError' ? 'Replay arrêté.' : `Erreur : ${error.message}`;
    } finally {
      controller = null;
      startButton.disabled = false;
      stopButton.disabled = true;
      fileInput.disabled = false;
    }
  });
})();
