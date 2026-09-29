import type {Sample} from '../../shared/api/types';

export function parseGPX(source: string): Sample[] {
  const document = new DOMParser().parseFromString(source, 'application/xml');
  if (document.querySelector('parsererror')) throw new Error('Le fichier GPX est mal formé.');
  const nodes = document.getElementsByTagNameNS('*', 'trkpt');
  if (nodes.length === 0) throw new Error('Le GPX ne contient aucun point de trace.');
  if (nodes.length > 50000) throw new Error('Le GPX dépasse la limite de 50 000 points.');
  const samples: Sample[] = [];
  let previousTime: number | null = null;
  for (const node of nodes) {
    const latitude = Number(node.getAttribute('lat'));
    const longitude = Number(node.getAttribute('lon'));
    const timeNode = [...node.children].find(child => child.localName === 'time');
    const elevationNode = [...node.children].find(child => child.localName === 'ele');
    const timestamp = Date.parse(timeNode?.textContent?.trim() || '');
    const altitude = elevationNode ? Number(elevationNode.textContent.trim()) : 0;
    if (
      !node.hasAttribute('lat') ||
      !node.hasAttribute('lon') ||
      !Number.isFinite(latitude) ||
      !Number.isFinite(longitude) ||
      !Number.isFinite(altitude) ||
      Math.abs(latitude) > 90 ||
      Math.abs(longitude) > 180 ||
      !Number.isFinite(timestamp)
    ) {
      throw new Error(`Point GPX invalide à la ligne ${samples.length + 1}.`);
    }
    if (previousTime !== null && timestamp < previousTime) {
      throw new Error('Les horodatages GPX doivent être dans l’ordre.');
    }
    samples.push({
      timestamp: new Date(timestamp).toISOString(),
      position: {latitude, longitude, altitude},
    });
    previousTime = timestamp;
  }
  return samples;
}
