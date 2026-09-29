import assert from 'node:assert/strict';
import {test} from 'node:test';
import {sendScheduledSamples} from './replaySchedule.ts';

test('HTTP send time does not accumulate in replay cadence', async () => {
  const point = (timestamp, latitude) => ({timestamp, position: {latitude, longitude: -1}});
  const samples = [
    point('2026-09-29T08:00:00Z', 47),
    point('2026-09-29T08:00:01Z', 48),
    point('2026-09-29T08:00:02Z', 49),
  ];
  const sentAt = [];
  const hints = [];
  let now = 0;
  const clock = {
    now: () => now,
    waitUntil: async deadline => {
      now = Math.max(now, deadline);
    },
  };
  await sendScheduledSamples(
    samples,
    () => 400,
    new AbortController().signal,
    async sample => {
      sentAt.push(now);
      hints.push(sample.next?.after_ms);
      now += 200;
    },
    () => {},
    clock,
  );
  assert.deepEqual(sentAt, [0, 400, 800]);
  assert.deepEqual(hints, [400, 400, undefined]);
});
