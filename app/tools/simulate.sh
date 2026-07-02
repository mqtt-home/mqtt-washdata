#!/usr/bin/env bash
#
# Simulate a Shelly Plug PM 3 dryer run by publishing switch:0 power status
# messages over MQTT. Lets you exercise mqtt-washdata (detection, learning,
# estimation, web UI) without a real device.
#
# Usage:
#   ./tools/simulate.sh [program]
#
#   program : cottons | synthetics | eco   (default: cottons)
#
# Environment overrides:
#   HOST=localhost PORT=1883 TOPIC=shelly/dryer   # broker + Shelly topic base
#   INTERVAL=5      # seconds between samples (real time)
#   DUR=...         # run length in seconds (per-program default otherwise)
#   TAIL=25         # trailing 0 W seconds to trigger the end of the run
#
# Requires mosquitto_pub (brew install mosquitto) and a running broker.
#
set -euo pipefail

HOST="${HOST:-localhost}"
PORT="${PORT:-1883}"
TOPIC="${TOPIC:-shelly/dryer}"
INTERVAL="${INTERVAL:-5}"
TAIL="${TAIL:-25}"
PROGRAM="${1:-cottons}"

case "$PROGRAM" in
  cottons)    DUR="${DUR:-150}"; PEAK=2100; BASE=250 ;;
  synthetics) DUR="${DUR:-100}"; PEAK=1500; BASE=220 ;;
  eco)        DUR="${DUR:-180}"; PEAK=1100; BASE=200 ;;
  *) echo "unknown program '$PROGRAM' (use: cottons | synthetics | eco)" >&2; exit 1 ;;
esac

if ! command -v mosquitto_pub >/dev/null 2>&1; then
  echo "mosquitto_pub not found. Install with: brew install mosquitto" >&2
  exit 1
fi

pub() { mosquitto_pub -h "$HOST" -p "$PORT" -t "$TOPIC/status/switch:0" -m "$1"; }

# random-ish starting energy counter so successive runs look realistic
E=$(awk 'BEGIN { srand(); print 1000 + int(rand() * 500) }')

echo "Simulating '$PROGRAM' run (~${DUR}s, samples every ${INTERVAL}s) -> ${HOST}:${PORT} ${TOPIC}/status/switch:0"

# pre-roll: idle
for _ in 1 2 3; do
  pub "{\"id\":0,\"apower\":0,\"voltage\":230,\"aenergy\":{\"total\":$E}}"
  sleep "$INTERVAL"
done

# running: rise -> plateau (with heater thermostat cycling) -> fall
t=0
while [ "$t" -lt "$DUR" ]; do
  read -r pw E < <(awk -v t="$t" -v dur="$DUR" -v peak="$PEAK" -v base="$BASE" -v e="$E" -v iv="$INTERVAL" 'BEGIN {
    p = t / dur
    if (p < 0.10)      env = p / 0.10          # ramp up
    else if (p > 0.85) env = (1 - p) / 0.15    # ramp down / cool
    else               env = 1                 # plateau
    if (env < 0) env = 0
    duty = (sin(t / 40 * 6.28318) > -0.3) ? 1.0 : 0.25   # heater cycling
    pw = base + (peak - base) * env * duty
    e = e + pw * iv / 3600
    printf "%.1f %.4f\n", pw, e
  }')
  pub "{\"id\":0,\"apower\":$pw,\"voltage\":230,\"aenergy\":{\"total\":$E}}"
  t=$((t + INTERVAL))
  sleep "$INTERVAL"
done

# tail: idle long enough to end the run
n=$(( TAIL / INTERVAL )); [ "$n" -lt 1 ] && n=1
for _ in $(seq "$n"); do
  pub "{\"id\":0,\"apower\":0,\"voltage\":230,\"aenergy\":{\"total\":$E}}"
  sleep "$INTERVAL"
done

echo "Done. Open the web UI to see the recorded run."
