# OpenSportTrack

**Rejouez un parcours GPX comme s'il venait d'une montre, puis suivez la course en direct sur une carte.**

Go 1.27 · React 19 · Vite 8 · WebSocket · Leaflet

<p align="center">
  <a href="docs/images/live-nantes.png">
    <img src="docs/images/live-nantes.png" alt="Vue live du marathon de Nantes avec métriques, parcours prévu et tracé parcouru" width="760">
  </a>
</p>
<p align="center"><sub>Marathon de Nantes, après environ 22 km de replay. Cliquez pour agrandir.</sub></p>

## En bref

- **Deux tracés sur la carte** : le parcours GPX complet en clair et la progression reçue en vert, chacun avec son interrupteur.
- **Un suivi fluide** : la position et les métriques avancent entre deux points envoyés. Cette interpolation reste visuelle ; elle ne crée pas de mesure GPS.
- **Deux façons de simuler** : un client Go qui respecte les horodatages du GPX, ou un simulateur web à intervalle réglable (250 ms à 2 s).

## Démarrer en local

Prérequis : **Go 1.27**, **Node.js 24** et npm. Docker n'est pas nécessaire pour développer.

**Terminal 1 — API**, depuis la racine du dépôt :

```sh
go run ./apps/api
```

**Terminal 2 — interface web**, depuis la racine du dépôt :

```sh
cd apps/web
npm ci
npm run dev
```

Ouvrez **[le simulateur web](http://localhost:5173/simulator)**, choisissez `examples/marathon-nantes-2016.gpx`, puis démarrez le replay. La carte live s'affiche dans la page et peut aussi être ouverte dans un onglet séparé.

### Rejouer Nantes avec le client Go

Dans un troisième terminal, depuis la racine du dépôt :

```sh
go run ./apps/simulator replay examples/marathon-nantes-2016.gpx --speed 300
```

La commande affiche une URL `/live/{activity_id}` à ouvrir dans le navigateur. `--speed 300` compresse le temps enregistré dans le GPX ; le simulateur web utilise, lui, une cadence fixe que vous pouvez modifier pendant la course. Le client Go accepte aussi `--server` et `--web-url` si vous changez les ports.

## Comment ça fonctionne

```text
GPX → simulateur Go ou web → API Go → WebSocket → carte React
```

| Emplacement | Responsabilité |
| --- | --- |
| [`apps/api`](apps/api) | Serveur HTTP, ingestion GPS et diffusion WebSocket |
| [`apps/simulator`](apps/simulator) | Client Go de replay GPX |
| [`apps/web`](apps/web) | Carte live et simulateur React + Vite |
| [`internal`](internal) | Packages Go partagés, privés au module |
| [`contracts`](contracts/http-ws-v1.md) | Contrat HTTP et WebSocket v1 |

Vite relaie `/api` et le WebSocket vers l'API locale sur `127.0.0.1:8081`. Les deux exécutables Go partagent un seul `go.mod` à la racine ; le web possède son propre `package.json`. Les tuiles OpenStreetMap demandent une connexion Internet dans le navigateur.

## Vérifier le projet

```sh
go test -race ./...
cd apps/web
npm ci
npm run lint
npm run format:check
npm test
npm run build
```

Pour formater Go et le front depuis la racine : `./scripts/format.sh`.

Après `npm ci` dans `apps/web`, activez le contrôle avant chaque push sur ce clone avec `./scripts/install-hooks.sh`. Le hook vérifie les fichiers du commit envoyé avec `gofmt` et Oxfmt ; si le format ne convient pas, appliquez `./scripts/format.sh`, commitez les corrections, puis relancez le push. Les clones suivants doivent activer le hook à leur tour.

## Docker, si besoin

```sh
docker compose up --build
```

Le web est alors disponible sur [localhost:8080/simulator](http://localhost:8080/simulator) et l'API sur `localhost:8081`. Pour rejouer le GPX dans Compose :

```sh
docker compose run --rm simulator replay /data/marathon-nantes-2016.gpx --speed 300 --server http://api:8081 --web-url http://localhost:8080
```

## Documentation et limites actuelles

- [Architecture du monorepo](docs/architecture.md)
- [Contrat HTTP et WebSocket v1](contracts/http-ws-v1.md)
- [Conception technique v0](docs/technical-design-v0.md)

Cette première version accepte uniquement la course à pied. L'état est en mémoire : les activités disparaissent au redémarrage de l'API. Il n'y a pas encore d'authentification, de persistance ni d'envoi GPS par lots.
