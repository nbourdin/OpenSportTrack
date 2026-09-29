# HTTP and WebSocket v1 contract

All coordinates use WGS84 degrees. Timestamps are RFC 3339 strings. The web app calculates displayed distances from received points.

| Method | Path | Request | Response |
| --- | --- | --- | --- |
| `GET` | `/healthz` | — | `200` |
| `POST` | `/api/v1/activities` | `{"sport":"running"}` | `201` with activity |
| `GET` | `/api/v1/activities/{id}` | — | `200` with activity, or `404` |
| `PUT` | `/api/v1/activities/{id}/route` | `{"positions":[Position, ...]}` | `204` |
| `POST` | `/api/v1/activities/{id}/samples` | `Sample` | `204` |
| `GET` | `/api/v1/activities/{id}/live` | WebSocket upgrade | JSON stream |

`Position`: `{"latitude":47.21808,"longitude":-1.55199,"altitude":12.5}`. Altitude is optional. A route contains 1 to 50,000 positions.

`Sample`: `{"timestamp":"2026-09-29T08:00:00Z","position":Position}`. Timestamps within an activity must not move backwards. The optional `next` field supports deterministic replay animation: `{"timestamp":"2026-09-29T08:00:01Z","position":Position,"after_ms":500}`. `after_ms` is the expected delay before sending the next point, in milliseconds. Clients must treat `next` as a visual hint, never as a received measurement.

Every WebSocket message includes `"version":1`, `"activity_id"`, and `"type"`:

- `snapshot` contains `activity`, `route`, and `samples` (state at connection time);
- `route` contains the new `route`;
- `sample` contains the newly accepted `sample`.

The API returns `400` for invalid JSON or data, `404` for an unknown activity, and `503` during shutdown. State is currently volatile.
