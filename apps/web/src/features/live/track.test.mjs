import assert from 'node:assert/strict';
import {test} from 'node:test';
import {haversine, metricsAt} from './track.ts';

test('distance and pace use all travelled points', () => {
  const first = {latitude: 47.0, longitude: -1};
  const second = {latitude: 47.009, longitude: -1};
  const distance = haversine(first, second);
  assert.ok(distance > 990 && distance < 1010);
  const metrics = metricsAt(0, 300000, distance);
  assert.equal(metrics.duration, 300);
  assert.ok(metrics.speed > 11.8 && metrics.speed < 12.2);
  assert.ok(metrics.pace > 295 && metrics.pace < 305);
});
