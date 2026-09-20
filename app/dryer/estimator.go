package dryer

import (
	"math"
	"sort"
)

// Estimate is the live prediction for an in-progress run.
type Estimate struct {
	Program    string
	Confidence float64
	// RemainingSec is the estimated remaining runtime, or -1 when unknown.
	RemainingSec int
	// Progress is 0..1, or -1 when unknown.
	Progress float64
}

// unknownEstimate is returned when there is not enough learned data to predict.
func unknownEstimate() Estimate {
	return Estimate{RemainingSec: -1, Progress: -1}
}

// A moisture-sensing dryer ends the cycle when the load is dry, so the same
// program legitimately runs anywhere between half an hour and two hours. The
// power curve is a featureless ramp whose shape says next to nothing about
// how far along the run is — backtests on real runs showed shape alignment
// losing to a plain median. What the curve does carry is state: how high the
// draw has climbed (bigger loads climb higher), and whether it has crested
// and started to fall (the load is nearly dry). The estimator therefore asks:
// of the past runs that were still running at this elapsed time, how much
// longer did the ones that looked most like this run take?
const (
	// trackStepSec is the resolution of a run's state track.
	trackStepSec = 60
	// trackWarmupSec: no level is derived before this much of the run is seen.
	trackWarmupSec = 300
	// levelWindowSec is the trailing window the level is the median of —
	// robust against a single reversal-pause dip or tumble spike.
	levelWindowSec = 360
	// levelMinSamples: windows with fewer working samples yield no level.
	levelMinSamples = 3
	// workingLevelFrac: samples below this fraction of the peak seen so far
	// are drum-reversal pauses, not the working draw.
	workingLevelFrac = 0.5
	// slopeSpanSteps is how far back the level slope looks (in track steps).
	slopeSpanSteps = 10

	// neighbours is how many of the most similar past runs are consulted.
	neighbours = 5
	// Similarity scales: a difference of one scale unit in any feature counts
	// the same. Level in W, drop from the crest in W, slope in W/min.
	levelScale = 20.0
	dropScale  = 8.0
	slopeScale = 1.5
	// missingStateDist is the distance given to a past run whose state at
	// this elapsed time is unknown.
	missingStateDist = 9.0

	// minRemainingSec: while the dryer is still running the cycle is not done.
	minRemainingSec = 60
)

// runTrack is the per-step state of a run: the working power level and its
// running maximum, indexed by elapsed time / trackStepSec. NaN when unknown.
type runTrack struct {
	durSec  int
	program string
	level   []float64
	crest   []float64
}

// buildTrack derives the state track of a (partial) run up to uptoSec.
func buildTrack(samples []PowerSample, uptoSec int) runTrack {
	n := uptoSec/trackStepSec + 1
	tr := runTrack{level: make([]float64, n), crest: make([]float64, n)}
	lo, hi := 0, 0
	peak := 0.0
	crest := math.NaN()
	var window []float64
	for i := 0; i < n; i++ {
		t := i * trackStepSec
		for hi < len(samples) && samples[hi].Offset <= t {
			if samples[hi].Power > peak {
				peak = samples[hi].Power
			}
			hi++
		}
		for lo < hi && samples[lo].Offset <= t-levelWindowSec {
			lo++
		}
		level := math.NaN()
		if t >= trackWarmupSec {
			window = window[:0]
			for _, s := range samples[lo:hi] {
				if s.Power >= workingLevelFrac*peak {
					window = append(window, s.Power)
				}
			}
			if len(window) >= levelMinSamples {
				level = medianFloat(window)
			}
		}
		if !math.IsNaN(level) && !(level <= crest) {
			crest = level
		}
		tr.level[i] = level
		tr.crest[i] = crest
	}
	return tr
}

// stateAt returns the similarity features at step i: level, drop from the
// crest, and level slope per minute (NaN when too early). ok is false when
// the level is unknown.
func (tr *runTrack) stateAt(i int) (level, drop, slope float64, ok bool) {
	if i < 0 || i >= len(tr.level) || math.IsNaN(tr.level[i]) {
		return 0, 0, 0, false
	}
	level = tr.level[i]
	drop = tr.crest[i] - level
	slope = math.NaN()
	if j := i - slopeSpanSteps; j >= 0 && !math.IsNaN(tr.level[j]) {
		slope = (level - tr.level[j]) / slopeSpanSteps
	}
	return level, drop, slope, true
}

// EstimatePartial predicts the remaining runtime of an in-progress run from
// the past runs that were still running at the same elapsed time, weighting
// toward those whose power state looked most like this run's (see above).
// The reported program is the one most of those runs belong to.
func (c *Classifier) EstimatePartial(samples []PowerSample, elapsedSec int) Estimate {
	if elapsedSec <= 0 || len(samples) < 2 {
		return unknownEstimate()
	}

	c.mu.RLock()
	tracks := c.tracks
	c.mu.RUnlock()
	if len(tracks) == 0 {
		return unknownEstimate()
	}

	step := elapsedSec / trackStepSec
	cur := buildTrack(samples, elapsedSec)
	level, drop, slope, known := cur.stateAt(step)

	type candidate struct {
		dist      float64
		remaining int
		program   string
	}
	var cands []candidate
	longest := 0
	for i := range tracks {
		tr := &tracks[i]
		if tr.durSec > longest {
			longest = tr.durSec
		}
		if tr.durSec <= elapsedSec {
			continue
		}
		dist := 0.0
		if known {
			dist = missingStateDist
			if l, d, s, ok := tr.stateAt(step); ok {
				dist = sq((level-l)/levelScale) + sq((drop-d)/dropScale)
				if !math.IsNaN(slope) && !math.IsNaN(s) {
					dist += sq((slope - s) / slopeScale)
				}
			}
		}
		cands = append(cands, candidate{dist, tr.durSec - elapsedSec, tr.program})
	}

	if len(cands) == 0 {
		// Outlasted every run seen so far: keep a small sliding remainder
		// instead of reporting the cycle as done.
		remaining := maxInt(longest/50, minRemainingSec)
		return Estimate{
			RemainingSec: remaining,
			Progress:     clamp01(float64(elapsedSec) / float64(elapsedSec+remaining)),
		}
	}

	// Without a known state every survivor counts equally (plain median).
	if known && len(cands) > neighbours {
		sort.SliceStable(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })
		cands = cands[:neighbours]
	}

	remainings := make([]int, len(cands))
	votes := map[string]int{}
	for i, cd := range cands {
		remainings[i] = cd.remaining
		if cd.program != "" {
			votes[cd.program]++
		}
	}
	remaining := maxInt(medianInt(remainings), minRemainingSec)

	program, best := "", 0
	for name, n := range votes {
		if n > best || (n == best && name < program) {
			program, best = name, n
		}
	}

	return Estimate{
		Program:      program,
		Confidence:   float64(best) / float64(len(cands)),
		RemainingSec: remaining,
		Progress:     clamp01(float64(elapsedSec) / float64(elapsedSec+remaining)),
	}
}

func sq(v float64) float64 { return v * v }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
