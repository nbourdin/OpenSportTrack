import type {Sample} from '../../shared/api/types';

export function parseGPX(source: string): Sample[] {
  const document = new DOMParser().parseFromString(source, 'application/xml');
  if (document.querySelector('parsererror')) throw new Error('The GPX file is malformed.');
  // GPX files can use different namespace prefixes for the same track-point element.
  const nodes = document.getElementsByTagNameNS('*', 'trkpt');
  if (nodes.length === 0) throw new Error('The GPX file contains no track points.');
  if (nodes.length > 50000) throw new Error('The GPX file exceeds the 50,000-point limit.');
  const samples: Sample[] = [];
  let previousTime: number | null = null;
  for (const node of nodes) {
    const latitude = Number(node.getAttribute('lat'));
    const longitude = Number(node.getAttribute('lon'));
    const timeNode = [...node.children].find(child => child.localName === 'time');
    const elevationNode = [...node.children].find(child => child.localName === 'ele');
    const timestamp = Date.parse(timeNode?.textContent?.trim() || '');
    const altitude = elevationNode ? Number(elevationNode.textContent.trim()) : 0;
    // Check attribute presence because Number(null) would silently become zero.
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
      throw new Error(`Invalid GPX point #${samples.length + 1}.`);
    }
    if (previousTime !== null && timestamp < previousTime) {
      throw new Error('GPX timestamps must be in order.');
    }
    samples.push({
      timestamp: new Date(timestamp).toISOString(),
      position: {latitude, longitude, altitude},
    });
    previousTime = timestamp;
  }
  return samples;
}
