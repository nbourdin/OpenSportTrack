# OpenSportTrack

Premier jalon exécutable : création d'une activité de course, ingestion GPS, diffusion WebSocket, replay GPX et carte live. L'état de l'API est en mémoire et disparaît au redémarrage.

## Structure

| Dossier | Rôle |
| --- | --- |
| `apps/api` | Module Go unique : API HTTP, WebSocket et replay GPX en CLI |
| `apps/web` | Application React + TypeScript + Vite : carte live et simulateur GPX |
| `contracts` | Description du protocole HTTP/WebSocket v1 partagé |
| `examples` | GPX du marathon de Nantes pour les essais |
| `docs` | Conception et décisions d'architecture |

Le web possède son propre `package.json` et `package-lock.json`. Il n'y a pas de npm workspace tant qu'il n'y a qu'un seul paquet JavaScript. Le backend a un seul `go.mod` ; `go.work` n'est pas nécessaire. Voir [l'architecture](docs/architecture.md) et la [conception technique v0](docs/technical-design-v0.md).

## Développement local, sans Docker

Prérequis : Go 1.27, Node.js 24 et npm. Sur macOS, installer Go avec `brew install go` si besoin.

Terminal 1, l'API :

```sh
cd apps/api
go run ./cmd/ost server
```

Terminal 2, le web :

```sh
cd apps/web
npm ci
npm run dev
```

Ouvrir [http://localhost:5173/simulator](http://localhost:5173/simulator). Choisir un GPX, puis régler l'intervalle d'envoi à 250 ms, 500 ms, 1 s ou 2 s. On peut le modifier pendant le replay. La vue live s'ouvre aussi à `/live/{activity_id}`. Vite relaie `/api` et le WebSocket vers l'API locale sur `127.0.0.1:8081` ; aucun réglage CORS n'est requis. Les changements React sont rechargés automatiquement. Pour un changement Go, relancer `go run`.

Le parcours GPX complet apparaît en trait clair pointillé, le parcours reçu en vert. Les deux tracés peuvent être masqués ou affichés dans le contrôle en haut à droite de la carte. Les simulateurs annoncent le point suivant pour animer la progression pendant l'intervalle d'envoi ; les positions intermédiaires sont visuelles, pas des mesures GPS. Leaflet et les tuiles OpenStreetMap nécessitent une connexion Internet pour afficher le fond de carte.

Le GPX de Nantes est inclus dans `examples/marathon-nantes-2016.gpx`. Il fonctionne aussi dans le sélecteur de fichier du simulateur web. Pour le rejouer avec la CLI, lancer dans un troisième terminal depuis `apps/api` :

```sh
go run ./cmd/ost simulator replay ../../examples/marathon-nantes-2016.gpx --speed 300
```

La CLI affiche l'URL web de la vue live. `--speed` respecte les intervalles enregistrés dans le GPX, contrairement au simulateur web qui utilise une cadence fixe. Options utiles : `--server http://localhost:8081` et `--web-url http://localhost:5173`.

## Vérification

```sh
cd apps/api && go test -race ./...
cd ../web && npm run build
```

Docker reste facultatif. Pour construire et démarrer les deux applications :

```sh
docker compose up --build
```

Le web est alors sur [http://localhost:8080/simulator](http://localhost:8080/simulator) et l'API sur `localhost:8081`. Exemple de replay dans Compose :

```sh
docker compose run --rm simulator replay /data/marathon-nantes-2016.gpx --speed 300 --server http://api:8081 --web-url http://localhost:8080
```

## API v1

```sh
curl -X POST http://localhost:8081/api/v1/activities \
  -H 'Content-Type: application/json' -d '{"sport":"running"}'
```

Voir [le contrat v1](contracts/http-ws-v1.md) pour les requêtes et messages WebSocket. Cette version accepte seulement `running` et des points GPS horodatés, ordonnés dans le temps. Les métriques affichées sont calculées dans le navigateur. Il n'y a ni authentification, ni persistance, ni batch d'ingestion pour l'instant.
