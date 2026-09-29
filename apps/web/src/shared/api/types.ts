import type {z} from 'zod';
import type {
  activitySchema,
  liveMessageSchema,
  nextPointSchema,
  positionSchema,
  sampleSchema,
} from './schemas';

export type Position = z.infer<typeof positionSchema>;
export type NextPoint = z.infer<typeof nextPointSchema>;
export type Sample = z.infer<typeof sampleSchema>;
export type Activity = z.infer<typeof activitySchema>;
export type LiveMessage = z.infer<typeof liveMessageSchema>;
