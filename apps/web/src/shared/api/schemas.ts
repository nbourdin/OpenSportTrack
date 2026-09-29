import {z} from 'zod';

export const positionSchema = z.object({
  latitude: z.number().finite().min(-90).max(90),
  longitude: z.number().finite().min(-180).max(180),
  altitude: z.number().finite().optional(),
});

const timestampSchema = z.iso.datetime({offset: true});

export const nextPointSchema = z.object({
  timestamp: timestampSchema,
  position: positionSchema,
  after_ms: z
    .number()
    .int()
    .min(1)
    .max(24 * 60 * 60 * 1000),
});

export const sampleSchema = z.object({
  timestamp: timestampSchema,
  position: positionSchema,
  next: nextPointSchema.optional(),
});

export const activitySchema = z.object({
  id: z.string().min(1),
  sport: z.string().min(1),
  started_at: timestampSchema,
});

const messageBase = {
  version: z.literal(1),
  activity_id: z.string().min(1),
};

export const liveMessageSchema = z.discriminatedUnion('type', [
  z.object({
    ...messageBase,
    type: z.literal('snapshot'),
    activity: activitySchema,
    route: z.array(positionSchema).optional(),
    samples: z.array(sampleSchema).optional(),
  }),
  z.object({...messageBase, type: z.literal('route'), route: z.array(positionSchema)}),
  z.object({...messageBase, type: z.literal('sample'), sample: sampleSchema}),
]);
