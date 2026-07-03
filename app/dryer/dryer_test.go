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

// TestParseShellyMessage covers the two message styles a Shelly publishes:
// plain component status and Gen3 RPC NotifyStatus frames (captured from a
// real Shelly Plug PM Gen3 with a pm1 power meter component).
func TestParseShellyMessage(t *testing.T) {
	t.Run("plain switch status", func(t *testing.T) {
		payload := `{"id":0,"output":true,"apower":1850.4,"voltage":230.1,"aenergy":{"total":1234.5}}`
		st, err := ParseShellyMessage([]byte(payload), 0)
		if err != nil || st == nil {
			t.Fatalf("err=%v st=%v", err, st)
		}
		if st.Apower != 1850.4 || st.Aenergy.Total != 1234.5 {
			t.Errorf("unexpected values: %+v", st)
		}
	})

	t.Run("rpc NotifyStatus with pm1 component", func(t *testing.T) {
		payload := `{
			"src":"shellyplugpmg3-907069531520",
			"dst":"shelly/ug/heizraum/trockner/events",
			"method":"NotifyStatus",
			"params":{"ts":1783004880.01,"pm1:0":{
				"aenergy":{"by_minute":[0,0,0],"minute_ts":1783004880,"total":1},
				"apower":27,"current":0.123,"freq":50.01,
				"ret_aenergy":{"by_minute":[0,0,0],"minute_ts":1783004880,"total":0},
				"voltage":233.3}}}`
		st, err := ParseShellyMessage([]byte(payload), 0)
		if err != nil || st == nil {
			t.Fatalf("err=%v st=%v", err, st)
		}
		if st.Apower != 27 || st.Aenergy.Total != 1 || st.Voltage != 233.3 {
			t.Errorf("unexpected values: %+v", st)
		}
	})

	t.Run("rpc NotifyStatus with switch component", func(t *testing.T) {
		payload := `{"method":"NotifyStatus","params":{"ts":1.0,"switch:0":{"apower":42,"aenergy":{"total":7}}}}`
		st, err := ParseShellyMessage([]byte(payload), 0)
		if err != nil || st == nil {
			t.Fatalf("err=%v st=%v", err, st)
		}
		if st.Apower != 42 || st.Aenergy.Total != 7 {
			t.Errorf("unexpected values: %+v", st)
		}
	})

	t.Run("rpc notification without power data", func(t *testing.T) {
		payload := `{"src":"shellyplugpmg3-907069531520","method":"NotifyStatus","params":{"ts":1783004873.03,"sys":{"available_updates":{"beta":{"version":"2.0.0-beta3"}}}}}`
		st, err := ParseShellyMessage([]byte(payload), 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st != nil {
			t.Errorf("expected nil status for sys-only frame, got %+v", st)
		}
	})

	t.Run("garbage payload", func(t *testing.T) {
		if _, err := ParseShellyMessage([]byte("true"), 0); err == nil {
			t.Error("expected error for non-object payload")
		}
	})
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

// partialRun produces samples of a run following shape, stretched to trueDur
// seconds, observed up to elapsed seconds. Returns samples and consumed energy.
func partialRun(shape func(float64) float64, trueDur, elapsed int) ([]PowerSample, float64) {
	var samples []PowerSample
	var energy float64
	prev := -1.0
	for s := 0; s <= elapsed; s += 10 {
		power := shape(float64(s) / float64(trueDur))
		samples = append(samples, PowerSample{Offset: s, Power: power})
		if prev >= 0 {
			energy += (prev + power) / 2 * 10 / 3600.0
		}
		prev = power
	}
	return samples, energy
}

// TestEstimatorDynamicDuration verifies that the estimator follows the pace of
// the current run: moisture-sensing dryers stretch the cycle for wetter loads
// and shorten it for drier ones, so the same program has a dynamic duration.
func TestEstimatorDynamicDuration(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	c := NewClassifier()
	c.Build([]*Run{
		makeRun("1", base, 3600, "Cottons", cottonsShape),
		makeRun("2", base.Add(time.Hour), 3500, "Cottons", cottonsShape),
	})

	for _, tc := range []struct {
		name    string
		trueDur int
	}{
		{"wetter load stretches the cycle", 4500},
		{"drier load shortens the cycle", 2900},
	} {
		t.Run(tc.name, func(t *testing.T) {
			elapsed := int(0.6 * float64(tc.trueDur))
			samples, energy := partialRun(cottonsShape, tc.trueDur, elapsed)

			est := c.EstimatePartial(samples, elapsed, energy)
			if est.Program != "Cottons" {
				t.Fatalf("program = %q, want Cottons", est.Program)
			}
			predicted := elapsed + est.RemainingSec
			relErr := math.Abs(float64(predicted-tc.trueDur)) / float64(tc.trueDur)
			t.Logf("trueDur=%d elapsed=%d predictedTotal=%d relErr=%.2f", tc.trueDur, elapsed, predicted, relErr)
			if relErr > 0.15 {
				t.Errorf("predicted total %d, want within 15%% of %d", predicted, tc.trueDur)
			}
		})
	}
}

// TestEstimatorNeverDoneWhileRunning: a run that outlasts everything the
// program has seen must still report a small positive remainder, not 0.
func TestEstimatorNeverDoneWhileRunning(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	c := NewClassifier()
	c.Build([]*Run{
		makeRun("1", base, 3600, "Cottons", cottonsShape),
		makeRun("2", base.Add(time.Hour), 3500, "Cottons", cottonsShape),
	})

	// Full shape already played out, but the dryer keeps running.
	elapsed := 5200
	samples, energy := partialRun(cottonsShape, 5000, elapsed)
	est := c.EstimatePartial(samples, elapsed, energy)
	if est.RemainingSec <= 0 {
		t.Errorf("remaining = %d, want > 0 while still running", est.RemainingSec)
	}
	if est.Progress >= 1 {
		t.Errorf("progress = %f, want < 1 while still running", est.Progress)
	}
}

// TestTrimRunTail: a run recorded with a too-low stop threshold drags on
// through anti-crease ("Knitterschutz") — ~7 W standby with brief drum tumbles
// every few minutes. The trim must cut the profile back to the actual cycle.
func TestTrimRunTail(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	r := makeRun("x", base, 3600, "", cottonsShape)
	// 1h anti-crease tail: 7 W baseline sampled per minute, 150 W tumble
	// spike every 5 minutes.
	for s := 3660; s <= 7200; s += 60 {
		p := 7.0
		if s%300 == 0 {
			p = 150
		}
		r.Samples = append(r.Samples, PowerSample{Offset: s, Power: p})
	}
	r.DurationSec = 7200
	r.End = base.Add(7200 * time.Second)

	TrimRunTail(r, testDetectionConfig())

	if r.DurationSec < 3400 || r.DurationSec > 3900 {
		t.Errorf("trimmed duration = %d, want ~3600", r.DurationSec)
	}
	if last := r.Samples[len(r.Samples)-1].Offset; last != r.DurationSec {
		t.Errorf("last sample offset %d != duration %d", last, r.DurationSec)
	}
	if !r.End.Equal(base.Add(time.Duration(r.DurationSec) * time.Second)) {
		t.Errorf("End not adjusted to trimmed duration")
	}
	if r.MeanPower < 200 {
		t.Errorf("mean power = %f, tail should not drag it down", r.MeanPower)
	}
}

// TestTrimRunTailKeepsCleanRun: a profile without an idle tail is unchanged.
func TestTrimRunTailKeepsCleanRun(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	r := makeRun("x", base, 3600, "", cottonsShape)
	samples := len(r.Samples)
	TrimRunTail(r, testDetectionConfig())
	if r.DurationSec != 3600 || len(r.Samples) != samples {
		t.Errorf("clean run modified: duration=%d samples=%d", r.DurationSec, len(r.Samples))
	}
}

func TestImportRuns(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(config.DryerConfig{Name: "Test", Detection: testDetectionConfig()}, "t", false, store)

	base := time.Unix(1_700_000_000, 0)
	runs := []*Run{
		makeRun("1", base, 3600, "Cottons", cottonsShape),
		makeRun("2", base.Add(time.Hour), 3500, "Cottons", cottonsShape),
		nil,                       // skipped
		{ID: "3", Finished: true}, // skipped: no duration/samples
	}
	imported, skipped, err := m.ImportRuns(runs)
	if err != nil {
		t.Fatal(err)
	}
	if imported != 2 || skipped != 2 {
		t.Errorf("imported=%d skipped=%d, want 2/2", imported, skipped)
	}
	if got := len(m.Runs()); got != 2 {
		t.Errorf("stored runs = %d, want 2", got)
	}
	// Programs must be relearned from the imported runs.
	if len(m.Programs()) != 1 || m.Programs()[0].Name != "Cottons" {
		t.Errorf("expected learned Cottons program, got %+v", m.Programs())
	}
	// Import is idempotent (upsert by id).
	imported, _, err = m.ImportRuns(runs[:2])
	if err != nil || imported != 2 {
		t.Errorf("re-import: imported=%d err=%v", imported, err)
	}
	if got := len(m.Runs()); got != 2 {
		t.Errorf("after re-import stored runs = %d, want 2", got)
	}
}

func TestEstimatorUnknownWithoutHistory(t *testing.T) {
	c := NewClassifier()
	c.Build(nil)
	est := c.EstimatePartial([]PowerSample{{Offset: 0, Power: 100}, {Offset: 10, Power: 200}}, 100, 5)
	if est.RemainingSec != -1 || est.Progress != -1 {
		t.Errorf("expected unknown estimate, got remaining=%d progress=%f", est.RemainingSec, est.Progress)
	}
}
