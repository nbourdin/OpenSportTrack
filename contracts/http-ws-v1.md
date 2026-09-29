# Contrat HTTP et WebSocket v1

Toutes les coordonnées sont en degrés WGS84. Les horodatages sont des chaînes RFC 3339. Les distances affichées par le web sont calculées à partir des points reçus.

| Méthode | Chemin | Requête | Réponse |
| --- | --- | --- | --- |
| `GET` | `/healthz` | — | `200` |
| `POST` | `/api/v1/activities` | `{"sport":"running"}` | `201` avec activité |
| `PUT` | `/api/v1/activities/{id}/route` | `{"positions":[Position, ...]}` | `204` |
| `POST` | `/api/v1/activities/{id}/samples` | `Sample` | `204` |
| `GET` | `/api/v1/activities/{id}/live` | Upgrade WebSocket | Flux JSON |

`Position` : `{"latitude":47.21808,"longitude":-1.55199,"altitude":12.5}`. L'altitude est facultative. Le parcours contient de 1 à 50 000 positions.

`Sample` : `{"timestamp":"2026-09-29T08:00:00Z","position":Position}`. Les horodatages d'une activité ne peuvent pas reculer. Le champ optionnel `next` sert à l'animation d'un replay déterministe : `{"timestamp":"2026-09-29T08:00:01Z","position":Position,"after_ms":500}`. `after_ms` décrit le délai avant l'envoi prévu du point suivant, en millisecondes. Le client doit traiter `next` comme une indication visuelle, jamais comme une mesure reçue.

Chaque message WebSocket porte `"version":1`, `"activity_id"` et `"type"` :

- `snapshot` contient `activity`, `route` et `samples` (état à la connexion) ;
- `route` contient le nouveau `route` ;
- `sample` contient le `sample` qui vient d'être accepté.

L'API renvoie `400` si le JSON ou les données sont invalides, `404` pour une activité absente et `503` pendant l'arrêt. L'état est actuellement volatile.
