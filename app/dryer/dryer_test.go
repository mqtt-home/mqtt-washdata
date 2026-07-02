package dryer

import (
	"math"
	"testing"
	"time"

	"github.com/mqtt-home/mqtt-washdata/config"
)

func testDetectionConfig() config.DetectionConfig {
	return config.DetectionConfig{
		StartWatts:        20,
		StopWatts:         5,
		StartDebounceSec:  20,
		StopDebounceSec:   60,
		SampleIntervalSec: 10,
		HeaterWatts:       800,
		MinRunSec:         120,
	}
}

// cottonsShape is a distinctive single-hump power curve over progress p in [0,1].
func cottonsShape(p float64) float64 {
	return 200 + 1500*math.Sin(math.Pi*p)
}

func TestDetectorDetectsRun(t *testing.T) {
	cfg := testDetectionConfig()
	d := NewDetector(cfg)

	base := time.Unix(1_700_000_000, 0)
	interval := 5 * time.Second
	runSeconds := 600

	var started, finished *Run
	counter := 1000.0 // Wh, cumulative energy counter
	var prevPower float64
	var prevT time.Time

	step := 0
	feed := func(power float64) {
		ts := base.Add(time.Duration(step) * interval)
		if !prevT.IsZero() {
			dt := ts.Sub(prevT).Seconds()
			counter += (prevPower + power) / 2 * dt / 3600.0
		}
		ev := d.Feed(ts, power, counter, true)
		switch ev.Type {
		case EventStarted:
			started = ev.Run
		case EventFinished:
			finished = ev.Run
		}
		prevPower = power
		prevT = ts
		step++
	}

	// 30s idle
	for s := 0; s < 30; s += 5 {
		feed(0)
	}
	// running
	for s := 0; s < runSeconds; s += 5 {
		p := float64(s) / float64(runSeconds)
		feed(cottonsShape(p))
	}
	// 120s off tail
	for s := 0; s < 120; s += 5 {
		feed(0)
	}

	if started == nil {
		t.Fatal("expected a run to start")
	}
	if finished == nil {
		t.Fatal("expected a run to finish")
	}
	if finished.DurationSec < 500 || finished.DurationSec > 620 {
		t.Errorf("unexpected duration: %d", finished.DurationSec)
	}
	if finished.EnergyWh <= 0 {
		t.Errorf("expected positive energy, got %f", finished.EnergyWh)
	}
	if finished.PeakPower < 1500 {
		t.Errorf("expected peak near 1700, got %f", finished.PeakPower)
	}
	if len(finished.Samples) < 10 {
		t.Errorf("expected many samples, got %d", len(finished.Samples))
	}
	// trailing tail should be trimmed
	last := finished.Samples[len(finished.Samples)-1]
	if last.Power <= cfg.StopWatts {
		t.Errorf("trailing near-zero sample not trimmed: %f", last.Power)
	}
}

// makeRun builds a finished, labeled run following a shape function.
func makeRun(id string, start time.Time, durationSec int, program string, shape func(float64) float64) *Run {
	var samples []PowerSample
	var energy float64
	prev := -1.0
	for s := 0; s <= durationSec; s += 10 {
		p := float64(s) / float64(durationSec)
		power := shape(p)
		samples = append(samples, PowerSample{Offset: s, Power: power})
		if prev >= 0 {
			energy += (prev + power) / 2 * 10 / 3600.0
		}
		prev = power
	}
	return &Run{
		ID:          id,
		Start:       start,
		End:         start.Add(time.Duration(durationSec) * time.Second),
		Finished:    true,
		DurationSec: durationSec,
		EnergyWh:    energy,
		Program:     program,
		Labeled:     program != "",
		Samples:     samples,
	}
}

func TestClassifierAndEstimator(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	runs := []*Run{
		makeRun("1", base, 3600, "Cottons", cottonsShape),
		makeRun("2", base.Add(time.Hour), 3500, "Cottons", cottonsShape),
	}

	c := NewClassifier()
	c.Build(runs)

	if len(c.Programs()) != 1 {
		t.Fatalf("expected 1 program, got %d", len(c.Programs()))
	}
	if c.Programs()[0].Name != "Cottons" {
		t.Fatalf("expected Cottons program, got %q", c.Programs()[0].Name)
	}

	// Full classification of a fresh Cottons run.
	fresh := makeRun("3", base.Add(2*time.Hour), 3550, "", cottonsShape)
	name, conf := c.ClassifyFull(fresh)
	if name != "Cottons" {
		t.Errorf("ClassifyFull = %q, want Cottons", name)
	}
	if conf < 0.8 {
		t.Errorf("ClassifyFull confidence = %f, want >= 0.8", conf)
	}

	// Partial estimation at ~40% through a Cottons run.
	fullDur := 3550
	elapsed := int(0.4 * float64(fullDur))
	var partial []PowerSample
	var energy float64
	prev := -1.0
	for s := 0; s <= elapsed; s += 10 {
		p := float64(s) / float64(fullDur) // progress along the FULL run
		power := cottonsShape(p)
		partial = append(partial, PowerSample{Offset: s, Power: power})
		if prev >= 0 {
			energy += (prev + power) / 2 * 10 / 3600.0
		}
		prev = power
	}

	est := c.EstimatePartial(partial, elapsed, energy)
	if est.Program != "Cottons" {
		t.Errorf("EstimatePartial program = %q, want Cottons", est.Program)
	}
	if est.Confidence < 0.5 {
		t.Errorf("EstimatePartial confidence = %f, want >= 0.5", est.Confidence)
	}
	if est.RemainingSec <= 0 || est.RemainingSec >= fullDur {
		t.Errorf("EstimatePartial remaining = %d, want in (0, %d)", est.RemainingSec, fullDur)
	}
	if est.Progress <= 0 || est.Progress >= 1 {
		t.Errorf("EstimatePartial progress = %f, want in (0,1)", est.Progress)
	}
	t.Logf("elapsed=%ds remaining=%ds progress=%.2f conf=%.2f", elapsed, est.RemainingSec, est.Progress, est.Confidence)
}

func TestEstimatorUnknownWithoutHistory(t *testing.T) {
	c := NewClassifier()
	c.Build(nil)
	est := c.EstimatePartial([]PowerSample{{Offset: 0, Power: 100}, {Offset: 10, Power: 200}}, 100, 5)
	if est.RemainingSec != -1 || est.Progress != -1 {
		t.Errorf("expected unknown estimate, got remaining=%d progress=%f", est.RemainingSec, est.Progress)
	}
}
