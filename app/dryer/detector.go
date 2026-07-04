package dryer

import (
	"fmt"
	"math"
	"time"

	"github.com/mqtt-home/mqtt-washdata/config"
)

// EventType classifies what a reading did to the detector state machine.
type EventType int

const (
	EventNone EventType = iota
	// EventStarted: a new run has just begun.
	EventStarted
	// EventSample: a new sample was appended to the running profile.
	EventSample
	// EventFinished: the run just ended (Run is the completed run).
	EventFinished
)

// Event is returned from Feed describing the effect of the latest reading.
type Event struct {
	Type EventType
	Run  *Run
}

type detectorState int

const (
	stateIdle detectorState = iota
	stateRunning
)

// Detector turns a stream of power readings into discrete dryer runs using a
// debounced start/stop state machine. It is not safe for concurrent use; feed it
// from a single goroutine.
type Detector struct {
	cfg config.DetectionConfig

	state   detectorState
	current *Run
	phase   string // current phase of the in-progress run (monotonic, see phaseRank)

	aboveSince   time.Time // first reading of the current above-threshold streak
	belowSince   time.Time // first reading of the current below-threshold streak
	lastSampleAt time.Time
	lastTs       time.Time
	lastPower    float64

	// energy tracking
	energyStart  float64 // aenergy.total counter at run start
	hasCounter   bool    // whether the device reports an energy counter
	integratedWh float64 // trapezoidal fallback
	liveEnergy   float64 // best-estimate energy consumed so far in the current run
}

func NewDetector(cfg config.DetectionConfig) *Detector {
	return &Detector{cfg: cfg, state: stateIdle}
}

// Running reports whether a run is currently in progress.
func (d *Detector) Running() bool { return d.state == stateRunning }

// Current returns the in-progress run, or nil when idle.
func (d *Detector) Current() *Run { return d.current }

// LiveEnergyWh returns the best estimate of energy consumed so far in the
// current run (0 when idle).
func (d *Detector) LiveEnergyWh() float64 { return round1(d.liveEnergy) }

// Feed processes one reading. energyTotal/hasEnergy come from the Shelly energy
// counter (aenergy.total); when hasEnergy is false, energy is integrated from power.
func (d *Detector) Feed(ts time.Time, power, energyTotal float64, hasEnergy bool) Event {
	// Integrate energy across the interval while a run is active.
	if d.state == stateRunning && !d.lastTs.IsZero() {
		dt := ts.Sub(d.lastTs).Seconds()
		if dt > 0 && dt < 3600 {
			d.integratedWh += (d.lastPower + power) / 2 * dt / 3600.0
		}
	}
	d.lastTs = ts
	d.lastPower = power

	startDebounce := time.Duration(d.cfg.StartDebounceSec) * time.Second
	stopDebounce := time.Duration(d.cfg.StopDebounceSec) * time.Second
	sampleInterval := time.Duration(d.cfg.SampleIntervalSec) * time.Second

	switch d.state {
	case stateIdle:
		if power >= d.cfg.StartWatts {
			if d.aboveSince.IsZero() {
				d.aboveSince = ts
			}
			if ts.Sub(d.aboveSince) >= startDebounce {
				d.beginRun(ts, energyTotal, hasEnergy)
				return Event{Type: EventStarted, Run: d.current}
			}
		} else {
			d.aboveSince = time.Time{}
		}

	case stateRunning:
		if power > d.current.PeakPower {
			d.current.PeakPower = power
		}
		if d.hasCounter && energyTotal >= d.energyStart {
			d.liveEnergy = energyTotal - d.energyStart
		} else {
			d.liveEnergy = d.integratedWh
		}

		ev := Event{Type: EventNone}
		if ts.Sub(d.lastSampleAt) >= sampleInterval {
			d.recordSample(ts, power)
			ev = Event{Type: EventSample, Run: d.current}
		}
		if p := d.computePhase(); phaseRank(p) > phaseRank(d.phase) {
			d.phase = p
		}

		if power <= d.cfg.StopWatts {
			if d.belowSince.IsZero() {
				d.belowSince = ts
			}
			if ts.Sub(d.belowSince) >= stopDebounce {
				fin := d.finishRun(hasEnergy, energyTotal)
				return Event{Type: EventFinished, Run: fin}
			}
		} else {
			d.belowSince = time.Time{}
		}
		return ev
	}

	return Event{Type: EventNone}
}

func (d *Detector) beginRun(ts time.Time, energyTotal float64, hasEnergy bool) {
	d.state = stateRunning
	d.phase = PhaseDrying
	d.aboveSince = time.Time{}
	d.belowSince = time.Time{}
	d.energyStart = energyTotal
	d.hasCounter = hasEnergy
	d.integratedWh = 0
	d.liveEnergy = 0

	d.current = &Run{
		ID:        fmt.Sprintf("%d", ts.Unix()),
		Start:     ts,
		PeakPower: d.lastPower,
	}
	d.recordSample(ts, d.lastPower)
}

func (d *Detector) recordSample(ts time.Time, power float64) {
	offset := int(ts.Sub(d.current.Start).Seconds())
	if offset < 0 {
		offset = 0
	}
	d.current.Samples = append(d.current.Samples, PowerSample{Offset: offset, Power: round1(power)})
	d.lastSampleAt = ts
}

func (d *Detector) finishRun(hasEnergy bool, energyTotal float64) *Run {
	r := d.current
	end := d.belowSince // power actually dropped here, not stopDebounce later
	if end.IsZero() {
		end = d.lastTs
	}
	r.End = end
	r.Finished = true
	r.DurationSec = int(end.Sub(r.Start).Seconds())

	// Energy: prefer the device counter delta, fall back to integration.
	if hasEnergy && d.hasCounter && energyTotal >= d.energyStart {
		r.EnergyWh = round1(energyTotal - d.energyStart)
	} else {
		r.EnergyWh = round1(d.integratedWh)
	}

	TrimRunTail(r, d.cfg)
	r.PeakPower = round1(r.PeakPower)

	d.state = stateIdle
	d.current = nil
	d.belowSince = time.Time{}
	return r
}

// idleTailWindowSec is the trailing window used to decide whether a run is
// still actively drying at a given point (see lastActiveIndex).
const idleTailWindowSec = 300

// TrimRunTail cuts the idle end sequence off a finished run's profile and
// fixes End/DurationSec/MeanPower accordingly. Two tails are removed: trailing
// near-zero samples (cool-down), and the anti-crease ("Knitterschutz") /
// standby tail — a dryer keeps drawing a few watts with brief periodic drum
// tumbles after the program ended, which would otherwise inflate the learned
// program duration. EnergyWh is left untouched; the tail's contribution is a
// few watt-hours at most.
func TrimRunTail(r *Run, cfg config.DetectionConfig) {
	trimBelowStop := func() {
		for len(r.Samples) > 1 && r.Samples[len(r.Samples)-1].Power <= cfg.StopWatts {
			r.Samples = r.Samples[:len(r.Samples)-1]
		}
	}
	trimBelowStop()
	if cut := lastActiveIndex(r.Samples, cfg.StartWatts, idleTailWindowSec); cut >= 0 && cut < len(r.Samples)-1 {
		r.Samples = r.Samples[:cut+1]
		trimBelowStop()
	}
	if n := len(r.Samples); n > 0 {
		if off := r.Samples[n-1].Offset; off > 0 && off < r.DurationSec {
			r.DurationSec = off
			r.End = r.Start.Add(time.Duration(off) * time.Second)
		}
	}
	r.MeanPower = round1(meanPower(r.Samples))
}

// windowActivity measures, with sample-and-hold semantics (a reading holds
// until the next one), how much of the windowSec ending at sample i was spent
// at or above watts. Sample-and-hold matters because samples arrive on power
// *changes*: an anti-crease tumble emits a burst of samples but only covers
// the few seconds it actually lasted, while the idle baseline in between
// covers minutes.
func windowActivity(samples []PowerSample, i int, watts float64, windowSec int) (activeSec, totalSec int) {
	tStart := samples[i].Offset - windowSec
	for j := i - 1; j >= 0 && samples[j+1].Offset > tStart; j-- {
		lo := samples[j].Offset
		if lo < tStart {
			lo = tStart
		}
		seg := samples[j+1].Offset - lo
		if samples[j].Power >= watts {
			activeSec += seg
		}
		totalSec += seg
	}
	return activeSec, totalSec
}

// isActiveAt reports whether the run is actively working at sample i: whether
// the trailing windowSec ending there spent at least half its time at or
// above activeWatts. The continuously powered main cycle is above the
// threshold nearly 100% of the time.
func isActiveAt(samples []PowerSample, i int, activeWatts float64, windowSec int) bool {
	active, total := windowActivity(samples, i, activeWatts, windowSec)
	if total == 0 {
		return samples[i].Power >= activeWatts
	}
	return float64(active) >= 0.5*float64(total)
}

// hasSustainedActivity reports whether the profile contains at least one
// window with substantial coverage that was mostly above the start threshold.
// Distinguishes a real (even short) cycle from a chain of anti-crease tumble
// bursts: a burst is active at its own instant but never fills a window.
func hasSustainedActivity(samples []PowerSample, activeWatts float64, windowSec int) bool {
	for i := range samples {
		active, total := windowActivity(samples, i, activeWatts, windowSec)
		if total >= windowSec/2 && float64(active) >= 0.5*float64(total) {
			return true
		}
	}
	return false
}

// lastActiveIndex returns the index of the last sample that still belongs to
// the active part of the run (see isActiveAt), or -1 when none qualifies.
func lastActiveIndex(samples []PowerSample, activeWatts float64, windowSec int) int {
	for i := len(samples) - 1; i >= 0; i-- {
		if isActiveAt(samples, i, activeWatts, windowSec) {
			return i
		}
	}
	return -1
}

// Cool-down detection tunables.
const (
	// coolingWindowSec is deliberately shorter than idleTailWindowSec — the
	// cool-down phase only lasts a few minutes, a 5-minute window would still
	// be dominated by the drying level when it starts.
	coolingWindowSec = 120
	// coolingLevelFrac: power below this fraction of the run's peak no longer
	// counts as drying (heat source off; drum + fan draw far less).
	coolingLevelFrac = 0.5
	// coolingMinElapsedSec guards against the warm-up ramp at the start of a
	// run qualifying as cool-down.
	coolingMinElapsedSec = 600
)

// Phase reports which phase the in-progress run is in: PhaseDrying while the
// dryer is actively working, PhaseCooling once the sustained draw has dropped
// well below the run's peak (heat source off, drum and fan finishing the
// cycle), and PhaseAntiCrease when only the brief periodic tumbles of the end
// sequence ("Knitterschutz") remain. Empty when idle.
func (d *Detector) Phase() string {
	if d.state != stateRunning {
		return ""
	}
	return d.phase
}

// phaseRank orders the phases a run passes through. The phase only ever moves
// forward within a run (a cycle does not resume heating after cool-down), so
// noisy windows during a transition cannot flip the display back.
func phaseRank(phase string) int {
	switch phase {
	case PhaseCooling:
		return 2
	case PhaseAntiCrease:
		return 3
	default:
		return 1
	}
}

// PhaseSpan marks where a phase begins on a run's timeline.
type PhaseSpan struct {
	Phase    string `json:"phase"`
	StartSec int    `json:"startSec"`
}

// PhaseSpans classifies a finished run's profile into phases and returns the
// boundaries (first span always starts at 0). Unlike the live detection it
// looks at the whole profile: the cool-down is the trailing stretch of the
// active part that stays below half the run's peak — with sparse change-driven
// samples a short cool-down shelf may only be a couple of readings, which a
// trailing live window cannot resolve but hindsight can.
func PhaseSpans(samples []PowerSample, cfg config.DetectionConfig) []PhaseSpan {
	if len(samples) == 0 {
		return nil
	}
	spans := []PhaseSpan{{Phase: PhaseDrying, StartSec: 0}}
	var peak float64
	for _, s := range samples {
		if s.Power > peak {
			peak = s.Power
		}
	}

	// The active part ends here; anything after is the anti-crease tail
	// (present only on profiles that were not trimmed).
	end := lastActiveIndex(samples, cfg.StartWatts, idleTailWindowSec)
	if end < 0 {
		end = len(samples) - 1
	}

	cool := end + 1
	for cool > 0 &&
		samples[cool-1].Power < coolingLevelFrac*peak &&
		samples[cool-1].Offset >= coolingMinElapsedSec {
		cool--
	}
	if cool <= end {
		spans = append(spans, PhaseSpan{Phase: PhaseCooling, StartSec: samples[cool].Offset})
	}
	if end < len(samples)-1 {
		spans = append(spans, PhaseSpan{Phase: PhaseAntiCrease, StartSec: samples[end+1].Offset})
	}
	return spans
}

// computePhase classifies the run's current phase from the recent profile.
func (d *Detector) computePhase() string {
	s := d.current.Samples
	if len(s) == 0 {
		return PhaseDrying
	}
	last := len(s) - 1
	if !isActiveAt(s, last, d.cfg.StartWatts, idleTailWindowSec) {
		return PhaseAntiCrease
	}
	// Cooling: the recent window is still mostly powered (drum + fan), but
	// spends almost no time near the drying level anymore. Brief reversal
	// pauses don't qualify (power is off, not low, during a pause).
	if s[last].Offset >= coolingMinElapsedSec && d.current.PeakPower > 0 {
		active, total := windowActivity(s, last, d.cfg.StartWatts, coolingWindowSec)
		high, _ := windowActivity(s, last, coolingLevelFrac*d.current.PeakPower, coolingWindowSec)
		if total > 0 &&
			float64(active) >= 0.5*float64(total) &&
			float64(high) < 0.35*float64(total) {
			return PhaseCooling
		}
	}
	return PhaseDrying
}

func meanPower(samples []PowerSample) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += s.Power
	}
	return sum / float64(len(samples))
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
