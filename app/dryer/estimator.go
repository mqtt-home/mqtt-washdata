package dryer

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

// Moisture-sensing dryers adapt the cycle to the load: a wetter load extends
// the drying phases, a drier one shortens them. The estimator therefore treats
// a program's duration as dynamic — it infers the pace of the current run from
// the shape alignment (elapsed time vs. matched profile fraction) and blends it
// with the program median, shifting trust toward the observed pace as the run
// progresses. The predicted duration is bounded relative to the durations the
// program has actually shown, stretched to allow loads outside the seen range.
const (
	durBandLo = 0.8
	durBandHi = 1.3
)

// predictTotalSec predicts this run's total duration for program p, given the
// matched shape fraction fr and the elapsed time.
func predictTotalSec(fr float64, elapsedSec int, p *Program) float64 {
	med := float64(p.MedianDurSec)
	if fr <= 0 {
		return med
	}
	pace := float64(elapsedSec) / fr
	lo, hi := durBandLo*float64(p.MinDurSec), durBandHi*float64(p.MaxDurSec)
	if lo <= 0 || hi <= 0 {
		lo, hi = durBandLo*med, durBandHi*med
	}
	if pace < lo {
		pace = lo
	}
	if pace > hi {
		pace = hi
	}
	w := clamp01(fr)
	return w*pace + (1-w)*med
}

// EstimatePartial recognizes an in-progress run by correlating its partial power
// shape against the leading portion of every learned program, then predicts the
// remaining runtime. The best-correlating alignment yields the progress fraction;
// the time scale is the dynamic duration predicted for this run (see
// predictTotalSec), cross-checked against energy consumed so far.
func (c *Classifier) EstimatePartial(samples []PowerSample, elapsedSec int, energyWh float64) Estimate {
	if elapsedSec <= 0 || len(samples) < 2 {
		return unknownEstimate()
	}

	c.mu.RLock()
	programs := c.programs
	overallDur := c.overallDur
	overallEnergy := c.overallEnrgy
	c.mu.RUnlock()

	best := unknownEstimate()
	bestCorr := -2.0

	for _, p := range programs {
		if len(p.Profile) < 5 || p.MedianDurSec <= 0 {
			continue
		}
		// Constrain the alignment scan to fractions whose implied total
		// duration is plausible for this program: without this, a partial can
		// correlate near-perfectly with a tiny leading slice of the profile
		// (Pearson is scale-invariant), implying an absurd multi-hour run.
		frLo, frHi := 0.0, 1.0
		if lo := durBandLo * float64(p.MinDurSec); lo > 0 {
			frHi = float64(elapsedSec) / lo
		}
		if hi := durBandHi * float64(p.MaxDurSec); hi > 0 {
			frLo = float64(elapsedSec) / hi
		}
		fr, corr := bestAlignment(samples, elapsedSec, p.Profile, frLo, frHi)
		if corr <= bestCorr {
			continue
		}
		bestCorr = corr

		// Heat-pump dryers ramp their draw up as the load dries (compressor
		// pressure rises with temperature). When the recent draw has reached
		// the profile's crest level, the load is at target dryness no matter
		// what the shape alignment believes — a lighter load gets there well
		// before the median timeline. The floor never pulls a further-along
		// shape estimate back.
		if frLevel, ok := levelFloor(samples, p.Profile); ok && frLevel > fr {
			fr = frLevel
		}

		total := predictTotalSec(fr, elapsedSec, p)

		prog := fr
		if p.MedianEnergy > 0 && energyWh > 0 && p.MedianDurSec > 0 {
			// A stretched (wetter) run consumes proportionally more energy, so the
			// expected total energy scales with the predicted duration.
			expEnergy := p.MedianEnergy * total / float64(p.MedianDurSec)
			prog = 0.6*fr + 0.4*clamp01(energyWh/expEnergy)
		}
		prog = clamp01(prog)

		remaining := int((1 - prog) * total)
		// While the dryer is still running the cycle is not done, even if it has
		// outlasted the prediction — keep a small sliding remainder instead of 0.
		if minRemaining := int(total / 50); remaining < minRemaining {
			remaining = minRemaining
		}
		predicted := elapsedSec + remaining
		progress := clamp01(float64(elapsedSec) / float64(maxInt(predicted, 1)))

		best = Estimate{
			Program:      p.Name,
			Confidence:   clamp01(corr),
			RemainingSec: remaining,
			Progress:     progress,
		}
	}

	if bestCorr < minMatchCorr {
		return fallbackEstimate(elapsedSec, energyWh, overallDur, overallEnergy)
	}
	return best
}

// crestLevelFrac defines "at crest level" for the level floor. Deliberately
// loose: the same program runs at visibly different absolute levels depending
// on the load (~10% observed), and a lighter load tops out below the learned
// median crest.
const crestLevelFrac = 0.90

// levelFloor returns a floor on the run's progress for ramp-shaped program
// profiles: when the recent draw has reached the profile's crest level, the
// run is at least as far along as where the profile first reaches that level.
// Positions below the crest are deliberately not derived from the level — a
// shallow ramp (a few W/min) against tens of watts of load-to-load offset
// makes mid-ramp level positions far noisier than shape alignment.
func levelFloor(samples []PowerSample, profile []float64) (float64, bool) {
	n := len(profile)
	if n < 20 || len(samples) < 2 {
		return 0, false
	}
	sm := smoothProfile(profile, 5)

	// The ramp must crest late (a mid-run hump is not a ramp) and rise
	// meaningfully above its early level.
	peakIdx := 0
	for i, v := range sm {
		if v > sm[peakIdx] {
			peakIdx = i
		}
	}
	if peakIdx < n*3/5 {
		return 0, false
	}
	early := 0.0
	for i := n / 20; i < n/5; i++ {
		early += sm[i]
	}
	early /= float64(n/5 - n/20)
	if early <= 0 || sm[peakIdx] < 1.15*early {
		return 0, false // flat profile: level carries no position information
	}

	if recentLevel(samples, 300) < crestLevelFrac*sm[peakIdx] {
		return 0, false
	}
	// Report where the profile first reaches the crest: the run is at least
	// that far along. The crest itself would claim too much — the ramp hovers
	// near peak for the final stretch.
	for i := 0; i <= peakIdx; i++ {
		if sm[i] >= crestLevelFrac*sm[peakIdx] {
			return float64(i) / float64(n-1), true
		}
	}
	return 0, false
}

// recentLevel is the median power over the trailing windowSec of samples —
// robust against a single reversal-pause dip or tumble spike.
func recentLevel(samples []PowerSample, windowSec int) float64 {
	tStart := samples[len(samples)-1].Offset - windowSec
	var recent []float64
	for i := len(samples) - 1; i >= 0 && samples[i].Offset >= tStart; i-- {
		recent = append(recent, samples[i].Power)
	}
	return medianFloat(recent)
}

// smoothProfile applies a centered moving average of the given width.
func smoothProfile(profile []float64, width int) []float64 {
	n := len(profile)
	out := make([]float64, n)
	half := width / 2
	for i := 0; i < n; i++ {
		lo, hi := i-half, i+half
		if lo < 0 {
			lo = 0
		}
		if hi > n-1 {
			hi = n - 1
		}
		var sum float64
		for j := lo; j <= hi; j++ {
			sum += profile[j]
		}
		out[i] = sum / float64(hi-lo+1)
	}
	return out
}

// bestAlignment scans how far into a program's timeline the current partial shape
// best fits, considering only progress fractions within [frLo, frHi]. It
// returns the progress fraction (leading length / full length) that maximizes
// the correlation, and that correlation.
func bestAlignment(samples []PowerSample, elapsedSec int, profile []float64, frLo, frHi float64) (float64, float64) {
	n := len(profile)
	bestFr, bestCorr := float64(0), -2.0
	// k is the number of leading program points the partial run is compared to.
	kMin := int(frLo * float64(n))
	if kMin < 5 {
		kMin = 5
	}
	kMax := int(frHi*float64(n)) + 1
	if kMax > n {
		kMax = n
	}
	if kMin > kMax {
		kMin = kMax // run has outlasted the plausible band: pin to the end
	}
	for k := kMin; k <= kMax; k++ {
		leading := profile[:k]
		partial := resampleSamples(samples, elapsedSec, k)
		corr := pearson(partial, leading)
		if corr > bestCorr {
			bestCorr = corr
			bestFr = float64(k) / float64(n)
		}
	}
	return bestFr, bestCorr
}

// fallbackEstimate is used before any program matches confidently: it predicts
// from the overall median duration / energy of past runs.
func fallbackEstimate(elapsedSec int, energyWh float64, overallDur int, overallEnergy float64) Estimate {
	if overallDur <= 0 {
		return unknownEstimate()
	}
	total := float64(overallDur)
	if e := float64(elapsedSec); e > total {
		// Running longer than the historical median: the load is stretching the
		// cycle, so extend the horizon instead of reporting it as done.
		total = e * 1.05
	}
	prog := clamp01(float64(elapsedSec) / total)
	if overallEnergy > 0 && energyWh > 0 {
		prog = 0.5*prog + 0.5*clamp01(energyWh/overallEnergy)
	}
	remaining := int((1 - prog) * total)
	if minRemaining := int(total / 50); remaining < minRemaining {
		remaining = minRemaining
	}
	predicted := elapsedSec + remaining
	return Estimate{
		Program:      "",
		Confidence:   0,
		RemainingSec: remaining,
		Progress:     clamp01(float64(elapsedSec) / float64(maxInt(predicted, 1))),
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
