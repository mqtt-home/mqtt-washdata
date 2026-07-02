package dryer

import (
	"fmt"
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

	// Trim trailing near-zero samples (the cool-down tail) so the profile ends
	// where the dryer actually stopped drawing power.
	for len(r.Samples) > 1 && r.Samples[len(r.Samples)-1].Power <= d.cfg.StopWatts {
		r.Samples = r.Samples[:len(r.Samples)-1]
	}

	// Energy: prefer the device counter delta, fall back to integration.
	if hasEnergy && d.hasCounter && energyTotal >= d.energyStart {
		r.EnergyWh = round1(energyTotal - d.energyStart)
	} else {
		r.EnergyWh = round1(d.integratedWh)
	}

	r.MeanPower = round1(meanPower(r.Samples))
	r.PeakPower = round1(r.PeakPower)

	d.state = stateIdle
	d.current = nil
	d.belowSince = time.Time{}
	return r
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
	return float64(int64(v*10+0.5)) / 10
}
