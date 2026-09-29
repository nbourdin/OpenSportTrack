export interface Position {
  latitude: number;
  longitude: number;
  altitude?: number;
}

export interface NextPoint {
  timestamp: string;
  position: Position;
  after_ms: number;
}

export interface Sample {
  timestamp: string;
  position: Position;
  next?: NextPoint;
}

export interface Activity {
  id: string;
  sport: string;
}

export type LiveMessage =
  | {version: 1; type: 'snapshot'; activity_id: string; route?: Position[]; samples?: Sample[]}
  | {version: 1; type: 'route'; activity_id: string; route: Position[]}
  | {version: 1; type: 'sample'; activity_id: string; sample: Sample};
