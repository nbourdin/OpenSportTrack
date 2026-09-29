# OpenSportTrack

> Open-source real-time sports telemetry server and protocol.

**Status:** Draft\
**Version:** 0.1\
**Primary language:** Go

------------------------------------------------------------------------

## 1. Vision

OpenSportTrack est un projet open source permettant à n'importe quel
appareil ou application d'envoyer des données sportives en temps réel
vers un serveur indépendant.

Le serveur reçoit les données de télémétrie, maintient l'état d'une
activité, calcule des métriques en temps réel et diffuse les données aux
spectateurs connectés.

L'objectif est notamment de permettre :

-   le live tracking d'une activité ;
-   l'expérimentation autour des données sportives ;
-   la création de devices compatibles ;
-   le self-hosting ;
-   la simulation d'appareils sans matériel spécifique.

Le projet doit également constituer un cas d'utilisation naturel des
forces de Go :

-   concurrence ;
-   networking ;
-   streaming ;
-   gestion efficace des I/O ;
-   faible empreinte mémoire ;
-   binaires autonomes ;
-   traitement temps réel.

------------------------------------------------------------------------

## 2. MVP

Le premier MVP doit permettre le scénario suivant :

``` text
GPX file
   │
   ▼
Go Simulator
   │
   │ HTTP
   ▼
Go Tracking Server
   │
   ├── Activity state
   ├── Live metrics
   └── Broadcast
           │
           │ WebSocket
           ▼
       Web Viewer
```

Commande cible :

``` bash
docker compose up
```

Puis :

``` bash
ost simulator replay examples/run.gpx --speed 10
```

L'utilisateur ouvre ensuite :

``` text
http://localhost:8080/live/{activity_id}
```

et voit l'activité se dérouler sur une carte.

------------------------------------------------------------------------

## 3. MVP Scope

### Included

#### Server

-   création d'une activité ;
-   ingestion de points de télémétrie ;
-   gestion de plusieurs activités simultanées ;
-   calcul de métriques en temps réel ;
-   diffusion WebSocket ;
-   gestion de plusieurs spectateurs ;
-   état en mémoire ;
-   graceful shutdown.

#### Simulator

-   lecture GPX ;
-   replay en temps réel ;
-   accélération du temps ;
-   création automatique d'une activité ;
-   envoi des points au serveur.

#### Viewer

-   connexion WebSocket ;
-   carte ;
-   position actuelle ;
-   tracé parcouru ;
-   distance ;
-   durée ;
-   vitesse ;
-   allure.

### Not included initially

Le MVP ne contient pas :

-   authentification ;
-   comptes utilisateurs ;
-   clubs ;
-   PostgreSQL ;
-   Redis ;
-   Kubernetes ;
-   microservices ;
-   application mobile ;
-   Garmin/Coros/Wahoo ;
-   FIT ;
-   historique d'activités ;
-   analytics avancées.

Ces éléments pourront être ajoutés lorsque le besoin apparaîtra.

------------------------------------------------------------------------

## 4. Domain model

### Activity

Une `Activity` représente une session sportive en cours.

``` go
type Activity struct {
    ID        string
    Sport     Sport
    StartedAt time.Time
}
```

Exemples de sports :

``` text
running
cycling
trail
walking
skiing
rowing
```

Pour le MVP, `running` suffit.

------------------------------------------------------------------------

## 5. Telemetry model

Le cœur du protocole est un `Sample`.

``` go
type Sample struct {
    Timestamp time.Time `json:"timestamp"`

    Position *Position `json:"position,omitempty"`

    HeartRate *uint16  `json:"heart_rate,omitempty"`
    Cadence   *uint16  `json:"cadence,omitempty"`
    Power     *uint16  `json:"power,omitempty"`
    Speed     *float64 `json:"speed,omitempty"`
}
```

Position :

``` go
type Position struct {
    Latitude  float64 `json:"latitude"`
    Longitude float64 `json:"longitude"`
    Altitude  float64 `json:"altitude,omitempty"`
}
```

Les métriques sont optionnelles afin que différents devices puissent
envoyer différentes capacités.

``` text
Phone
GPS

Running watch
GPS + HR + cadence

Cycling computer
GPS + HR + cadence + power
```

------------------------------------------------------------------------

## 6. Protocol

Le protocole doit rester simple dans un premier temps.

### Create activity

``` http
POST /api/v1/activities
```

Request :

``` json
{
  "sport": "running"
}
```

Response :

``` json
{
  "id": "01JXYZ...",
  "sport": "running",
  "started_at": "2026-09-28T18:00:00Z"
}
```

### Send telemetry

``` http
POST /api/v1/activities/{activity_id}/samples
```

Request :

``` json
{
  "timestamp": "2026-09-28T18:00:01Z",
  "position": {
    "latitude": 47.2184,
    "longitude": -1.5536,
    "altitude": 42.1
  },
  "heart_rate": 156
}
```

Response :

``` http
204 No Content
```

------------------------------------------------------------------------

## 7. Batch ingestion

Les devices réels peuvent perdre temporairement leur connexion.

Le protocole devra donc rapidement supporter l'envoi de plusieurs
samples.

``` http
POST /api/v1/activities/{activity_id}/samples/batch
```

``` json
{
  "samples": [
    {},
    {},
    {}
  ]
}
```

Cela permettra :

``` text
device
   │
   X network unavailable
   │
local buffer
   │
network restored
   ▼
batch upload
```

Le batch pourra être introduit après le premier vertical slice.

------------------------------------------------------------------------

## 8. Live stream

Les spectateurs utilisent WebSocket.

``` text
GET /api/v1/activities/{activity_id}/live
```

Connexion :

``` text
viewer
   │
   ▼
WebSocket
   │
   ▼
activity stream
```

Un message pourrait être :

``` json
{
  "type": "sample",
  "activity_id": "01JXYZ",
  "sample": {
    "timestamp": "...",
    "position": {
      "latitude": 47.2184,
      "longitude": -1.5536
    }
  }
}
```

Les messages devront être versionnables.

------------------------------------------------------------------------

## 9. Server architecture

Architecture conceptuelle :

``` text
HTTP
 │
 ▼
Ingestion Handler
 │
 ▼
Activity Manager
 │
 ▼
Activity Runtime
 │
 ├──────────────► Metrics
 │
 ├──────────────► Storage
 │
 └──────────────► Broadcast
                         │
                         ▼
                    WebSockets
```

Le composant central sera `ActivityRuntime`.

------------------------------------------------------------------------

## 10. Activity Runtime

Une activité active possède son propre runtime.

Conceptuellement :

``` go
type ActivityRuntime struct {
    activity Activity

    samples chan Sample

    subscribers map[*Subscriber]struct{}
}
```

Le runtime possède une boucle principale.

``` go
func (r *ActivityRuntime) Run(ctx context.Context) {
    for {
        select {
        case sample := <-r.samples:
            r.process(sample)

        case <-ctx.Done():
            return
        }
    }
}
```

Ce modèle permet d'explorer naturellement :

-   goroutines ;
-   channels ;
-   cancellation ;
-   backpressure ;
-   synchronisation ;
-   lifecycle management.

------------------------------------------------------------------------

## 11. Concurrency model

Chaque activité active peut posséder une goroutine.

``` text
ActivityManager

 ├── Activity A
 │      └── goroutine
 │
 ├── Activity B
 │      └── goroutine
 │
 └── Activity C
        └── goroutine
```

Un sample entrant est envoyé au channel correspondant.

``` text
HTTP request
     │
     ▼
ActivityManager
     │
     ▼
activity.samples
     │
     ▼
Activity goroutine
```

Le handler HTTP ne doit pas effectuer les calculs métier lourds.

------------------------------------------------------------------------

## 12. Backpressure

Un client lent ne doit jamais bloquer une activité.

Chaque subscriber disposera donc d'un buffer.

``` go
type Subscriber struct {
    Messages chan Message
}
```

Par exemple :

``` go
make(chan Message, 64)
```

Si le buffer est plein, plusieurs stratégies pourront être étudiées :

``` text
drop newest
drop oldest
disconnect slow consumer
```

Pour le MVP, **disconnect slow consumer** est probablement le
comportement le plus simple.

Ce comportement devra être mesurable et testé.

------------------------------------------------------------------------

## 13. Metrics

Les métriques initiales seront calculées progressivement.

### Distance

Distance entre deux coordonnées GPS.

Pour commencer : **Haversine**.

Distance totale :

``` text
distance += distance(previousPoint, currentPoint)
```

### Duration

``` text
current timestamp - started_at
```

### Speed

Si elle n'est pas fournie par le device :

``` text
distance delta / time delta
```

### Pace

Pour la course :

``` text
pace = duration / distance
```

Exemple :

``` text
4:32 / km
```

------------------------------------------------------------------------

## 14. Simulator

Le simulateur fait partie intégrante du projet.

Commande :

``` bash
ost simulator replay activity.gpx
```

Options :

``` bash
--speed 1
--speed 5
--speed 10
--speed 100
```

Exemple :

``` bash
ost simulator replay examples/run.gpx --speed 10
```

Une activité réelle de 50 minutes sera rejouée en 5 minutes.

------------------------------------------------------------------------

## 15. GPX reader

Le parser GPX doit fonctionner avec `io.Reader`.

API cible :

``` go
reader := gpx.NewReader(file)

for reader.Next() {
    point := reader.Point()
}
```

ou éventuellement :

``` go
err := gpx.Read(file, func(point Point) error {
    return simulator.Send(point)
})
```

L'objectif est d'éviter de charger inutilement tout le fichier en
mémoire.

Cela permet de travailler avec les patterns Go autour de :

-   `io.Reader` ;
-   streaming ;
-   iterators ;
-   error handling.

------------------------------------------------------------------------

## 16. Replay engine

Le replay doit respecter les timestamps GPX.

Exemple :

``` text
GPX

10:00:00 point A
10:00:01 point B
10:00:03 point C
```

À vitesse `1x` :

``` text
A
wait 1s
B
wait 2s
C
```

À vitesse `10x` :

``` text
A
wait 100ms
B
wait 200ms
C
```

Le replay doit accepter un `context.Context` afin de pouvoir être
interrompu proprement.

------------------------------------------------------------------------

## 17. CLI

Nom temporaire :

``` text
ost
```

Exemples :

``` bash
ost server
ost simulator replay run.gpx
ost simulator replay run.gpx --speed 20
```

Plus tard :

``` bash
ost activity inspect run.fit
ost benchmark ingestion
```

------------------------------------------------------------------------

## 18. Viewer

Le viewer doit rester volontairement simple.

Il doit afficher :

``` text
┌──────────────────────────────────────┐
│                                      │
│               MAP                    │
│                                      │
│          ───────●                    │
│                                      │
├──────────────────────────────────────┤
│                                      │
│ Distance       Pace       Duration   │
│                                      │
│ 8.42 km        4:32/km    38:12      │
│                                      │
└──────────────────────────────────────┘
```

Technologie à décider.

Le viewer n'est pas le cœur du projet. Une simple SPA légère suffit.

------------------------------------------------------------------------

## 19. Repository structure

Structure initiale envisagée :

``` text
opensporttrack/

├── cmd/
│   └── ost/
│
├── internal/
│   ├── activity/
│   ├── ingestion/
│   ├── broadcast/
│   └── metrics/
│
├── pkg/
│   ├── protocol/
│   └── client/
│
├── gpx/
│
├── simulator/
│
├── web/
│
├── examples/
│   └── activities/
│
├── go.mod
├── README.md
└── docker-compose.yml
```

Cette structure est volontairement provisoire.

On évitera de créer des packages avant qu'une responsabilité réelle
apparaisse.

------------------------------------------------------------------------

## 20. Dependency policy

Le projet doit privilégier la standard library lorsque cela est
raisonnable.

Cela ne signifie pas **no dependencies**.

Chaque dépendance doit résoudre un problème réel.

Exemples où une dépendance peut être pertinente :

-   WebSocket ;
-   CLI ;
-   logging structuré ;
-   parsing FIT ;
-   observabilité.

Pour HTTP, `net/http` sera privilégié initialement.

------------------------------------------------------------------------

## 21. Error handling

Les erreurs doivent être explicites et wrappées avec contexte.

``` go
if err != nil {
    return fmt.Errorf("decode GPX point: %w", err)
}
```

Pas d'exceptions implicites ni de `panic` pour les erreurs
opérationnelles.

------------------------------------------------------------------------

## 22. Context

`context.Context` doit être utilisé pour les opérations ayant un
lifecycle.

Exemples :

-   HTTP request ;
-   activity runtime ;
-   simulator ;
-   WebSocket ;
-   server shutdown.

Il ne doit pas être stocké arbitrairement dans les structures métier.

------------------------------------------------------------------------

## 23. Graceful shutdown

Lorsque le serveur reçoit `SIGTERM` ou `SIGINT`, il doit :

``` text
stop accepting connections
        ↓
cancel root context
        ↓
stop activity runtimes
        ↓
close websocket connections
        ↓
wait workers
        ↓
exit
```

Cela fera partie des comportements testés.

------------------------------------------------------------------------

## 24. Observability

Dès le début : **structured logs**.

Plus tard :

-   Prometheus metrics ;
-   OpenTelemetry ;
-   pprof.

Métriques intéressantes :

``` text
active_activities
connected_viewers
samples_received_total
samples_dropped_total
sample_processing_duration
```

------------------------------------------------------------------------

## 25. Testing strategy

Le projet devra avoir beaucoup de tests unitaires, particulièrement sur
:

-   GPX parsing ;
-   distance calculations ;
-   metrics ;
-   replay timing ;
-   activity lifecycle ;
-   subscriber lifecycle ;
-   backpressure.

Les tests concurrents seront exécutés régulièrement avec :

``` bash
go test -race ./...
```

------------------------------------------------------------------------

## 26. Benchmarks

Le projet devra également contenir de vrais benchmarks Go.

``` go
func BenchmarkActivityIngestion(b *testing.B) {
    // ...
}
```

Objectifs futurs :

-   samples / second ;
-   memory / active activity ;
-   broadcast throughput ;
-   number of simultaneous viewers.

On ne fixe pas encore de chiffres arbitraires. On mesure d'abord.

------------------------------------------------------------------------

## 27. Persistence

### MVP

``` text
memory
```

Les activités disparaissent au redémarrage du serveur. C'est volontaire.

### Future

Une abstraction de stockage pourra apparaître lorsqu'elle sera
réellement nécessaire.

Possibilités :

-   PostgreSQL ;
-   TimescaleDB ;
-   ClickHouse ;
-   SQLite.

Le choix devra être basé sur les patterns d'utilisation réels.

------------------------------------------------------------------------

## 28. Future device architecture

Une future application mobile pourra agir comme device.

``` text
Phone GPS
   │
   ▼
Flutter application
   │
   ▼
OpenSportTrack protocol
   │
   ▼
Go server
```

Elle pourra :

-   start activity ;
-   record GPS ;
-   buffer samples offline ;
-   send samples ;
-   stop activity.

Le téléphone permettra donc de tester OpenSportTrack avec un véritable
signal GPS sans montre connectée.

------------------------------------------------------------------------

## 29. Future protocol capabilities

Le modèle devra pouvoir évoluer vers :

-   GPS ;
-   heart rate ;
-   cadence ;
-   power ;
-   temperature ;
-   altitude ;
-   speed ;
-   stroke rate ;
-   running dynamics.

Mais seules les données nécessaires seront implémentées initialement.

------------------------------------------------------------------------

## 30. Potential hardware integrations

À terme, des adaptateurs pourraient connecter :

-   Garmin ;
-   Wahoo ;
-   Coros ;
-   Apple Watch ;
-   Wear OS ;
-   ANT+ ;
-   Bluetooth LE sensors ;
-   cycling computers ;
-   GPS trackers.

Ces intégrations doivent rester découplées du serveur principal.

------------------------------------------------------------------------

## 31. Engineering principles

### Keep the core small

Le serveur doit rester compréhensible.

### Prefer explicit code

Éviter les abstractions prématurées.

### Concurrency must solve a real problem

Pas de goroutines décoratives.

Chaque goroutine doit avoir :

``` text
owner
lifecycle
cancellation strategy
```

### Measure before optimizing

Utiliser benchmarks, pprof et metrics avant d'introduire des
optimisations complexes.

### Protocol first

Les devices ne doivent pas dépendre de l'implémentation interne du
serveur.

``` text
Device
   │
   ▼
Protocol
   │
   ▼
Server
```

Cela permettra à d'autres implémentations de serveur ou de client
d'exister.

------------------------------------------------------------------------

## 32. First implementation milestone

Le premier vertical slice doit être extrêmement petit.

### Milestone 1 --- One runner

#### Server

``` text
POST activity
POST sample
WebSocket live
```

#### Simulator

``` text
GPX reader
replay
HTTP client
```

#### Viewer

``` text
WebSocket
map
moving marker
```

Pas de DB. Pas d'auth. Pas de heart rate. Pas de multi-device.

------------------------------------------------------------------------

## 33. Milestone 2 --- Real telemetry engine

Ajouter :

-   distance ;
-   speed ;
-   pace ;
-   duration ;
-   activity state ;
-   multiple viewers.

Puis tester :

-   slow consumers ;
-   disconnects ;
-   invalid samples ;
-   out-of-order samples.

------------------------------------------------------------------------

## 34. Milestone 3 --- Concurrency

Supporter plusieurs activités :

``` text
100 activities
1000 viewers
continuous telemetry
```

Ajouter :

-   benchmarks ;
-   race detector ;
-   pprof ;
-   load simulator.

À ce stade, on pourra réellement observer le comportement de Go sous
charge.

------------------------------------------------------------------------

## 35. Milestone 4 --- Persistence

Introduire la persistence seulement maintenant.

Premiers besoins :

-   recover activity ;
-   activity history ;
-   historical track.

Le stockage pourra être choisi sur la base de benchmarks et du modèle
réel.

------------------------------------------------------------------------

## 36. Milestone 5 --- Real device

Créer une application mobile minimale :

``` text
START
  ↓
GPS recording
  ↓
live upload
  ↓
STOP
```

Le téléphone devient alors le premier véritable device OpenSportTrack.
