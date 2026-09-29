# Architecture du premier monorepo

La séparation suit les programmes qui se lancent indépendamment : `apps/api` pour le serveur Go, `apps/simulator` pour le client Go de replay et `apps/web` pour React. Les exemples, contrats et documents restent au niveau racine. L'API et le simulateur ont chacun leur Dockerfile mais partagent le module Go racine et les packages privés dans `internal`. Le web garde ses propres dépendances npm.

Le web appelle l'API avec des chemins relatifs `/api/v1/...` et ouvre le WebSocket sur le même hôte que la page. En développement, Vite relaie ces requêtes vers `127.0.0.1:8081`. Dans Compose, Nginx relaie vers le service `api:8081`. Cette règle conserve la même URL côté navigateur dans les deux environnements.

Le contrat réseau v1 est documenté dans `contracts/http-ws-v1.md`. Les types TypeScript correspondants sont dans `apps/web/src/shared/api/types.ts` ; les types Go restent dans `internal/tracking`. Le document est la référence commune jusqu'à ce qu'une génération de schéma devienne utile.

Un seul paquet JavaScript ne justifie pas npm workspaces. Un seul module Go ne justifie pas `go.work`. Ces outils pourront être ajoutés si plusieurs paquets ou modules apparaissent.
