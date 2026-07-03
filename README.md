# mqtt-washdata

A small Go microservice that watches a **Shelly Plug PM 3 (Gen3)** over MQTT,
detects when the connected **dryer** is running, records a **power profile** for
each run, recognizes the dryer **program by shape-correlation matching**, and
**estimates the remaining runtime** — getting more accurate the more it learns.

Completed runs and the live run are visible in a small web UI, and the live
status (state, program, remaining time, ETA, progress, power, energy) is
published to MQTT for home automation.

Built in the style of the sibling services (`roborock-mqtt`, `shelly-commands`):
[go-logger](https://github.com/philipparndt/go-logger),
[mqtt-gateway](https://github.com/philipparndt/mqtt-gateway),
[chi](https://github.com/go-chi/chi), a React + Vite + Tailwind frontend, and
GoReleaser → Docker Hub.

## How it works

```
Shelly Plug PM 3 ──(MQTT: status/switch:0 or events/rpc, apower)──▶ mqtt-washdata
                                                                       │
   detect run (debounced start/stop) ─▶ record power profile ─▶ persist run (JSON)
                                                                       │
   learn programs (shape correlation + clustering) ◀── label runs in the UI
                                                                       │
   live: match partial shape ─▶ estimate remaining time ─▶ MQTT status + web UI (SSE)
```

- **Run detection** — a debounced state machine: sustained power above
  `start_watts` opens a run; sustained power below `stop_watts` (long enough to
  survive cool-down / anti-crease pauses) closes it. The cool-down tail is
  trimmed from the recorded profile.
- **Program recognition** — each run's power curve is resampled to a fixed length
  and compared to learned programs using **normalized cross-correlation (Pearson)**,
  which matches *shape* independently of absolute wattage. User labels create
  authoritative named programs; unlabeled runs are clustered automatically
  (`Program A`, `Program B`, …) until you name them.
- **Remaining-time estimate** — for the live run, the partial curve is aligned to
  the best-correlating point in each program's timeline. Because moisture-sensing
  dryers stretch or shorten a cycle per load, the program duration is treated as
  **dynamic**: the run's own pace (elapsed time vs. matched fraction) is blended
  with the program's typical duration — trusting the observed pace more as the
  run progresses — bounded by the duration range the program has shown, and
  cross-checked against energy consumed so far. A run that outlasts its
  prediction keeps a small sliding remainder instead of showing done.
  Before anything is learned, it falls back to the median of past runs.

## Configuration

The service takes a single argument: the path to a JSON config file. Runs are
persisted in a `runs/` directory next to that file. `${ENV_VAR}` placeholders are
expanded from the environment.

```json
{
  "mqtt": { "url": "tcp://192.168.0.1:1883", "topic": "home/washdata/dryer", "qos": 2, "retain": true },
  "dryer": {
    "name": "Dryer",
    "shelly_topic": "shelly/dryer",
    "detection": {
      "start_watts": 20, "stop_watts": 5,
      "start_debounce_sec": 20, "stop_debounce_sec": 180,
      "sample_interval_sec": 10, "heater_watts": 800, "min_run_sec": 120
    }
  },
  "web": { "enabled": true, "port": 8080 },
  "loglevel": "info"
}
```

See [`app/production/config/config.example.json`](app/production/config/config.example.json).

## MQTT

- **Subscribes** to both message styles a Shelly can be configured for:
  - `<shelly_topic>/status/switch:0` — plain component status with `apower`.
  - `<shelly_topic>/events/rpc` — Gen2+/Gen3 RPC `NotifyStatus` frames; the power
    meter is read from the `switch:<id>` or `pm1:<id>` component (the Plug PM
    Gen3 reports `pm1:0`). Frames without power data (e.g. sys updates) are ignored.
- **Publishes** `status_update` to `<shelly_topic>/command` on start to get a fresh reading.
- **Publishes** (retained): `<mqtt.topic>/status` — the live status payload:

```json
{ "state": "running", "dryerName": "Dryer", "program": "Cottons", "confidence": 0.82,
  "power": 1850.4, "energyWh": 1234, "elapsedSec": 3600, "remainingSec": 1800,
  "progress": 0.66, "eta": "2026-07-01T18:30:00Z", "runId": "1719849600" }
```

## Web API

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/api/status` | current live status |
| GET | `/api/runs` | list of runs (without samples) |
| GET | `/api/runs/{id}` | one run including its power profile |
| POST | `/api/runs/{id}/label` | `{ "program": "Cottons" }` — label/train (empty clears) |
| DELETE | `/api/runs/{id}` | delete a run |
| GET | `/api/programs` | learned programs + stats |
| GET | `/api/events` | SSE live status stream |
| GET | `/api/health`, `/api/livez` | health / liveness |

## Configuration files

- `production/config/config.example.json` — committed template (points at your
  real broker; use `${ENV_VAR}` for secrets).
- `production/config/config.json` — your local config, **git-ignored**. This is
  the file `make dev` runs against. Create it by copying the example:

  ```bash
  cp production/config/config.example.json production/config/config.json
  ```

Everything in `production/config/` is git-ignored except the example and the
`.gitignore` itself, so your local config and recorded `runs/` never get committed.

## Try it without a Shelly

You don't need the physical plug to see the whole thing work — you can feed it
simulated power data over MQTT.

1. **Have an MQTT broker.** Point `config.json` at your existing broker, or run a
   local one: `mosquitto -p 1883` (`brew install mosquitto`). The provided test
   config uses `tcp://localhost:1883` with shortened detection timings so runs
   complete in a minute or two.
2. **Start the service:** `cd app && make dev`, then open http://localhost:8080.
   With no data yet you'll see the dashboard in its idle state.
3. **Simulate a dryer run** (in another terminal):

   ```bash
   cd app
   make simulate                     # a "cottons" run
   make simulate PROGRAM=synthetics  # a different shape
   make simulate PROGRAM=eco
   ```

   Watch the dashboard: the run is detected, the live power sparkline moves, and
   when it ends it appears in **Recent runs**. Open a run, give it a **label**
   (e.g. *Cottons*), then simulate the same program again — it should now be
   **auto-recognized** and show a live remaining-time estimate.

> Even with no broker at all, `make dev` still serves the web UI (it just shows
> the idle/offline state) — so you can review the interface immediately.

The simulator is `app/tools/simulate.sh`; override behavior with env vars, e.g.
`HOST=192.168.0.1 INTERVAL=3 DUR=60 ./tools/simulate.sh cottons`.

## Development

```bash
cd app
make help            # list targets
make build           # build frontend (pnpm) + backend
make dev             # build and run against ../production/config/config.json
make dev-frontend    # Vite dev server (talks to the backend API on :8080)
make simulate        # publish a simulated dryer run over MQTT
make test            # go test ./...
make docker          # build the Docker image
```

Requires Go 1.26+, Node 22+ and pnpm (plus `mosquitto` for the simulator).

## Deployment

Multi-arch (amd64/arm64) Docker images are published to
`pharndt/mqtt-washdata` by GoReleaser on tag `v*` (or via the *Build release*
workflow). The container expects its config at
`/var/lib/mqtt-washdata/config.json`.
