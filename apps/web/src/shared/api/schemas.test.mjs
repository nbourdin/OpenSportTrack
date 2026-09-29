import assert from 'node:assert/strict';
import {test} from 'node:test';
import {activitySchema, liveMessageSchema} from './schemas.ts';

const activity = {
  id: 'run-1',
  sport: 'running',
  started_at: '2026-09-29T08:00:00Z',
};
const position = {latitude: 47.2184, longitude: -1.5536};
const sample = {timestamp: '2026-09-29T08:00:01Z', position};

test('accepts the API activity and a live snapshot', () => {
  assert.equal(activitySchema.parse(activity).id, 'run-1');
  const result = liveMessageSchema.safeParse({
    version: 1,
    type: 'snapshot',
    activity_id: activity.id,
    activity,
    route: [position],
    samples: [sample],
  });
  assert.equal(result.success, true);
});

test('rejects malformed API and WebSocket data before rendering', () => {
  assert.equal(activitySchema.safeParse({...activity, id: 42}).success, false);
  assert.equal(
    liveMessageSchema.safeParse({
      version: 1,
      type: 'sample',
      activity_id: activity.id,
      sample: {...sample, position: {...position, latitude: 91}},
    }).success,
    false,
  );
  assert.equal(
    liveMessageSchema.safeParse({
      version: 1,
      type: 'sample',
      activity_id: activity.id,
      sample: {...sample, next: {...sample, after_ms: 0}},
    }).success,
    false,
  );
  assert.equal(
    liveMessageSchema.safeParse({version: 2, type: 'route', activity_id: activity.id, route: []})
      .success,
    false,
  );
});
