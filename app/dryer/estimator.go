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

// EstimatePartial recognizes an in-progress run by correlating its partial power
// shape against the leading portion of every learned program, then predicts the
// remaining runtime. The best-correlating alignment yields the progress fraction;
// the matched program's typical duration sets the time scale, cross-checked
// against energy consumed so far.
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
		fr, corr := bestAlignment(samples, elapsedSec, p.Profile)
		if corr <= bestCorr {
			continue
		}
		bestCorr = corr

		prog := fr
		if p.MedianEnergy > 0 && energyWh > 0 {
			frEnergy := clamp01(energyWh / p.MedianEnergy)
			prog = 0.6*fr + 0.4*frEnergy
		}
		prog = clamp01(prog)

		remaining := int((1 - prog) * float64(p.MedianDurSec))
		if remaining < 0 {
			remaining = 0
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

// bestAlignment scans how far into a program's timeline the current partial shape
// best fits. It returns the progress fraction (leading length / full length) that
// maximizes the correlation, and that correlation.
func bestAlignment(samples []PowerSample, elapsedSec int, profile []float64) (float64, float64) {
	n := len(profile)
	bestFr, bestCorr := float64(0), -2.0
	// k is the number of leading program points the partial run is compared to.
	kMin := 5
	for k := kMin; k <= n; k++ {
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
	prog := clamp01(float64(elapsedSec) / float64(overallDur))
	if overallEnergy > 0 && energyWh > 0 {
		prog = 0.5*prog + 0.5*clamp01(energyWh/overallEnergy)
	}
	remaining := int((1 - prog) * float64(overallDur))
	if remaining < 0 {
		remaining = 0
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
