package dryer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/mqtt-home/mqtt-washdata/config"
)

// TestBacktest replays an export of real runs against the estimator. It is
// skipped unless WASHDATA_EXPORT points at a file from GET /api/export:
//
//	WASHDATA_EXPORT=export.json go test ./dryer -run TestBacktest -v
//
// Walk-forward: every run is predicted with a classifier built only from the
// runs that finished before it, exactly as production saw it.
func TestBacktest(t *testing.T) {
	path := os.Getenv("WASHDATA_EXPORT")
	if path == "" {
		t.Skip("WASHDATA_EXPORT not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var export struct {
		Runs []*Run `json:"runs"`
	}
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatal(err)
	}
	runs := export.Runs
	sort.Slice(runs, func(i, j int) bool { return runs[i].Start.Before(runs[j].Start) })

	cfg := config.DetectionConfig{StartWatts: 20, StopWatts: 5}
	const warmup = 5 // runs needed before predictions are scored
	const stepSec = 60

	type point struct {
		frac   float64 // true progress of the checkpoint
		errSec float64 // predicted remaining - actual remaining
		naive  float64 // same, for "overall median duration - elapsed"
	}
	var all []point
	type runStat struct {
		id, program string
		dur         int
		mae, worst  float64
		bias        float64
		first       float64 // error of the first confident prediction (10 min in)
	}
	var stats []runStat

	for i := warmup; i < len(runs); i++ {
		r := runs[i]
		if r.DurationSec < 1200 || len(r.Samples) < 10 {
			continue // short aborted / refresh cycles say nothing about the ETA
		}
		c := NewClassifier()
		c.Build(runs[:i])

		var durs []int
		for _, p := range runs[:i] {
			durs = append(durs, p.DurationSec)
		}
		overall := float64(medianInt(durs))

		coolStart := r.DurationSec
		for _, sp := range PhaseSpans(r.Samples, cfg) {
			if sp.Phase == PhaseCooling {
				coolStart = sp.StartSec
			}
		}

		st := runStat{id: r.ID, program: r.Program, dur: r.DurationSec, first: math.NaN()}
		var n float64
		k := 0
		for ts := 300; ts < r.DurationSec; ts += stepSec {
			for k+1 < len(r.Samples) && r.Samples[k+1].Offset <= ts {
				k++
			}
			est := c.EstimatePartial(r.Samples[:k+1], ts)
			if est.RemainingSec < 0 {
				continue
			}
			rem := float64(est.RemainingSec)
			if ts >= coolStart && rem > coolingRemainingCapSec {
				rem = coolingRemainingCapSec
			}
			actual := float64(r.DurationSec - ts)
			e := rem - actual
			naive := math.Max(overall-float64(ts), overall/50) - actual
			all = append(all, point{float64(ts) / float64(r.DurationSec), e, naive})
			st.mae += math.Abs(e)
			st.bias += e
			st.worst = math.Max(st.worst, math.Abs(e))
			if math.IsNaN(st.first) && ts >= 600 {
				st.first = e
			}
			n++
		}
		if n > 0 {
			st.mae /= n
			st.bias /= n
			stats = append(stats, st)
		}
	}

	pct := func(v []float64, p float64) float64 {
		if len(v) == 0 {
			return math.NaN()
		}
		s := append([]float64(nil), v...)
		sort.Float64s(s)
		return s[int(math.Min(float64(len(s)-1), p*float64(len(s))))]
	}
	summarize := func(label string, sel func(point) (float64, bool)) {
		var abs []float64
		var bias float64
		for _, p := range all {
			if e, ok := sel(p); ok {
				abs = append(abs, math.Abs(e)/60)
				bias += e / 60
			}
		}
		if len(abs) == 0 {
			return
		}
		var mean float64
		for _, a := range abs {
			mean += a
		}
		fmt.Printf("%-22s n=%5d  mean=%5.1f  p50=%5.1f  p75=%5.1f  p90=%5.1f  p95=%5.1f  worst=%6.1f  bias=%+6.1f\n",
			label, len(abs), mean/float64(len(abs)), pct(abs, .5), pct(abs, .75), pct(abs, .9), pct(abs, .95), pct(abs, 1), bias/float64(len(abs)))
	}

	fmt.Printf("\n== remaining-time error, minutes (|predicted - actual|), %d runs scored ==\n", len(stats))
	summarize("estimator  all", func(p point) (float64, bool) { return p.errSec, true })
	summarize("naive      all", func(p point) (float64, bool) { return p.naive, true })
	for _, b := range [][2]float64{{0, .25}, {.25, .5}, {.5, .75}, {.75, .9}, {.9, 1.01}} {
		b := b
		in := func(p point) bool { return p.frac >= b[0] && p.frac < b[1] }
		summarize(fmt.Sprintf("estimator  %3.0f-%3.0f%%", b[0]*100, math.Min(b[1], 1)*100), func(p point) (float64, bool) { return p.errSec, in(p) })
		summarize(fmt.Sprintf("naive      %3.0f-%3.0f%%", b[0]*100, math.Min(b[1], 1)*100), func(p point) (float64, bool) { return p.naive, in(p) })
	}

	fmt.Printf("\n== per run (minutes), worst first ==\n%-12s %-30s %6s %6s %6s %7s %8s\n", "id", "program", "dur", "mae", "worst", "bias", "@10min")
	sort.Slice(stats, func(i, j int) bool { return stats[i].mae > stats[j].mae })
	var maes []float64
	for _, s := range stats {
		maes = append(maes, s.mae/60)
		fmt.Printf("%-12s %-30s %6.0f %6.1f %6.1f %+7.1f %+8.1f\n", s.id, s.program, float64(s.dur)/60, s.mae/60, s.worst/60, s.bias/60, s.first/60)
	}
	fmt.Printf("\nper-run MAE: p50=%.1f p90=%.1f worst=%.1f min\n", pct(maes, .5), pct(maes, .9), pct(maes, 1))
}
