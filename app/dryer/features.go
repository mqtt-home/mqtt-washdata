package dryer

import (
	"math"
	"sort"
)

// profilePoints is the fixed length every power profile is resampled to so that
// runs of different durations can be compared by shape.
const profilePoints = 100

// resampleSamples resamples an irregular power profile (samples spread over
// [0, durationSec]) onto n equally spaced points using linear interpolation.
func resampleSamples(samples []PowerSample, durationSec, n int) []float64 {
	out := make([]float64, n)
	if len(samples) == 0 || n <= 0 {
		return out
	}
	if durationSec <= 0 {
		for i := range out {
			out[i] = samples[len(samples)-1].Power
		}
		return out
	}
	// samples are already ordered by offset
	step := float64(durationSec) / float64(n-1)
	j := 0
	for i := 0; i < n; i++ {
		t := float64(i) * step
		// advance j so that samples[j].Offset <= t <= samples[j+1].Offset
		for j < len(samples)-1 && float64(samples[j+1].Offset) < t {
			j++
		}
		if j >= len(samples)-1 {
			out[i] = samples[len(samples)-1].Power
			continue
		}
		a, b := samples[j], samples[j+1]
		span := float64(b.Offset - a.Offset)
		if span <= 0 {
			out[i] = a.Power
			continue
		}
		frac := (t - float64(a.Offset)) / span
		out[i] = a.Power + frac*(b.Power-a.Power)
	}
	return out
}

// resampleVector resamples a numeric vector to length n (linear interpolation).
func resampleVector(v []float64, n int) []float64 {
	out := make([]float64, n)
	if len(v) == 0 || n <= 0 {
		return out
	}
	if len(v) == 1 {
		for i := range out {
			out[i] = v[0]
		}
		return out
	}
	step := float64(len(v)-1) / float64(n-1)
	for i := 0; i < n; i++ {
		x := float64(i) * step
		lo := int(math.Floor(x))
		hi := lo + 1
		if hi >= len(v) {
			out[i] = v[len(v)-1]
			continue
		}
		frac := x - float64(lo)
		out[i] = v[lo] + frac*(v[hi]-v[lo])
	}
	return out
}

// pearson returns the Pearson correlation coefficient of two equal-length
// vectors, in [-1, 1]. It measures shape similarity independent of absolute
// scale and offset. Flat vectors (no variance) yield 0.
func pearson(a, b []float64) float64 {
	n := len(a)
	if n == 0 || n != len(b) {
		return 0
	}
	var ma, mb float64
	for i := 0; i < n; i++ {
		ma += a[i]
		mb += b[i]
	}
	ma /= float64(n)
	mb /= float64(n)

	var cov, va, vb float64
	for i := 0; i < n; i++ {
		da := a[i] - ma
		db := b[i] - mb
		cov += da * db
		va += da * da
		vb += db * db
	}
	if va == 0 || vb == 0 {
		return 0
	}
	return cov / math.Sqrt(va*vb)
}

// medianProfile returns the pointwise median across a set of equal-length
// profiles (the representative shape of a program).
func medianProfile(profiles [][]float64, n int) []float64 {
	out := make([]float64, n)
	if len(profiles) == 0 {
		return out
	}
	col := make([]float64, 0, len(profiles))
	for i := 0; i < n; i++ {
		col = col[:0]
		for _, p := range profiles {
			if i < len(p) {
				col = append(col, p[i])
			}
		}
		out[i] = medianFloat(col)
	}
	return out
}

func medianFloat(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	mid := len(c) / 2
	if len(c)%2 == 1 {
		return c[mid]
	}
	return (c[mid-1] + c[mid]) / 2
}

func medianInt(v []int) int {
	if len(v) == 0 {
		return 0
	}
	c := append([]int(nil), v...)
	sort.Ints(c)
	mid := len(c) / 2
	if len(c)%2 == 1 {
		return c[mid]
	}
	return (c[mid-1] + c[mid]) / 2
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
