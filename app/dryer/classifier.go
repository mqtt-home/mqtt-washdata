package dryer

import (
	"fmt"
	"sort"
	"sync"
)

// Tunables for shape-correlation clustering / matching.
const (
	// clusterCorr: minimum profile correlation to fold an unlabeled run into an
	// existing auto cluster.
	clusterCorr = 0.85
	// minMatchCorr: below this correlation a live run is not confidently matched
	// to any program (estimator falls back to overall history).
	minMatchCorr = 0.30
	// minProfileSamples: runs with fewer samples are ignored for learning.
	minProfileSamples = 3
)

// Program is a learned dryer program: a representative power-curve shape plus
// duration/energy statistics, built from its member runs.
type Program struct {
	Name         string `json:"name"`
	Auto         bool   `json:"auto"` // provisional auto-cluster (vs. user label)
	Runs         int    `json:"runs"`
	MedianDurSec int    `json:"medianDurationSec"`
	// MinDurSec/MaxDurSec capture the duration spread of the member runs.
	// Moisture-sensing dryers stretch or shorten a program per load, so the same
	// program legitimately spans a range of durations.
	MinDurSec    int       `json:"minDurationSec"`
	MaxDurSec    int       `json:"maxDurationSec"`
	MedianEnergy float64   `json:"medianEnergyWh"`
	PeakPower    float64   `json:"peakPower"`
	Profile      []float64 `json:"profile"` // representative resampled watts (len profilePoints)
}

// Classifier holds the learned programs and recognizes runs by shape correlation.
// It is safe for concurrent use.
type Classifier struct {
	mu           sync.RWMutex
	programs     []*Program
	overallDur   int
	overallEnrgy float64
}

func NewClassifier() *Classifier {
	return &Classifier{}
}

// Build (re)learns all programs from the given finished runs. User-labeled runs
// form authoritative named programs; the rest are clustered by shape.
func (c *Classifier) Build(runs []*Run) {
	labeled := map[string][]*Run{}
	var order []string
	var unlabeled []*Run
	var allDur []int
	var allEnergy []float64

	// stable input order (oldest first) for deterministic clustering
	sorted := append([]*Run(nil), runs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start.Before(sorted[j].Start) })

	for _, r := range sorted {
		if !r.Finished || len(r.Samples) < minProfileSamples || r.DurationSec <= 0 {
			continue
		}
		allDur = append(allDur, r.DurationSec)
		allEnergy = append(allEnergy, r.EnergyWh)
		if r.Labeled && r.Program != "" {
			if _, ok := labeled[r.Program]; !ok {
				order = append(order, r.Program)
			}
			labeled[r.Program] = append(labeled[r.Program], r)
		} else {
			unlabeled = append(unlabeled, r)
		}
	}

	var programs []*Program
	used := map[string]bool{}

	// Members of each labeled program, seeded from user-labeled runs. Matching
	// centroids are frozen to the labeled runs only so fold-in is order-stable.
	type labeledProgram struct {
		name     string
		members  []*Run
		centroid []float64
	}
	var labeledPrograms []*labeledProgram
	for _, name := range order {
		m := labeled[name]
		labeledPrograms = append(labeledPrograms, &labeledProgram{
			name:     name,
			members:  append([]*Run(nil), m...),
			centroid: medianProfile(profilesOf(m), profilePoints),
		})
		used[name] = true
	}

	// Fold each unlabeled run into the best-matching labeled program (by shape);
	// only runs that match nothing go on to form auto clusters.
	var stillUnlabeled []*Run
	for _, r := range unlabeled {
		prof := resampleSamples(r.Samples, r.DurationSec, profilePoints)
		best, bestCorr := -1, clusterCorr
		for i, lp := range labeledPrograms {
			if len(lp.centroid) == 0 {
				continue
			}
			if corr := pearson(prof, lp.centroid); corr >= bestCorr {
				bestCorr = corr
				best = i
			}
		}
		if best >= 0 {
			labeledPrograms[best].members = append(labeledPrograms[best].members, r)
		} else {
			stillUnlabeled = append(stillUnlabeled, r)
		}
	}
	for _, lp := range labeledPrograms {
		programs = append(programs, buildProgram(lp.name, false, lp.members))
	}

	// Greedy shape clustering of the runs that matched no labeled program.
	var clusters [][]*Run
	var centroids [][]float64
	for _, r := range stillUnlabeled {
		prof := resampleSamples(r.Samples, r.DurationSec, profilePoints)
		best, bestCorr := -1, clusterCorr
		for i, cen := range centroids {
			if corr := pearson(prof, cen); corr >= bestCorr {
				bestCorr = corr
				best = i
			}
		}
		if best >= 0 {
			clusters[best] = append(clusters[best], r)
			centroids[best] = medianProfile(profilesOf(clusters[best]), profilePoints)
		} else {
			clusters = append(clusters, []*Run{r})
			centroids = append(centroids, prof)
		}
	}
	for i, cl := range clusters {
		name := autoName(i, used)
		programs = append(programs, buildProgram(name, true, cl))
	}

	sort.Slice(programs, func(i, j int) bool {
		if programs[i].Auto != programs[j].Auto {
			return !programs[i].Auto // labeled programs first
		}
		if programs[i].Runs != programs[j].Runs {
			return programs[i].Runs > programs[j].Runs
		}
		return programs[i].Name < programs[j].Name
	})

	c.mu.Lock()
	c.programs = programs
	c.overallDur = medianInt(allDur)
	c.overallEnrgy = medianFloat(allEnergy)
	c.mu.Unlock()
}

func buildProgram(name string, auto bool, runs []*Run) *Program {
	profs := profilesOf(runs)
	durs := make([]int, len(runs))
	energies := make([]float64, len(runs))
	var peak float64
	minDur, maxDur := 0, 0
	for i, r := range runs {
		durs[i] = r.DurationSec
		energies[i] = r.EnergyWh
		if r.PeakPower > peak {
			peak = r.PeakPower
		}
		if minDur == 0 || r.DurationSec < minDur {
			minDur = r.DurationSec
		}
		if r.DurationSec > maxDur {
			maxDur = r.DurationSec
		}
	}
	return &Program{
		Name:         name,
		Auto:         auto,
		Runs:         len(runs),
		MedianDurSec: medianInt(durs),
		MinDurSec:    minDur,
		MaxDurSec:    maxDur,
		MedianEnergy: medianFloat(energies),
		PeakPower:    peak,
		Profile:      medianProfile(profs, profilePoints),
	}
}

func profilesOf(runs []*Run) [][]float64 {
	out := make([][]float64, len(runs))
	for i, r := range runs {
		out[i] = resampleSamples(r.Samples, r.DurationSec, profilePoints)
	}
	return out
}

// autoName returns "Program A", "Program B", ... skipping any name already used
// by a labeled program.
func autoName(index int, used map[string]bool) string {
	i := 0
	for n := 0; ; n++ {
		name := fmt.Sprintf("Program %c", 'A'+rune(n%26))
		if n >= 26 {
			name = fmt.Sprintf("Program %c%d", 'A'+rune(n%26), n/26)
		}
		if used[name] {
			continue
		}
		if i == index {
			used[name] = true
			return name
		}
		i++
	}
}

// Programs returns a snapshot of the learned programs. The slice is never nil
// so an empty snapshot serializes to [] rather than null.
func (c *Classifier) Programs() []*Program {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append(make([]*Program, 0, len(c.programs)), c.programs...)
}

// ClassifyFull matches a finished run's full profile against the learned
// programs and returns the best-matching program name and confidence (0..1).
// An empty name means no confident match.
func (c *Classifier) ClassifyFull(r *Run) (string, float64) {
	if r.DurationSec <= 0 || len(r.Samples) < minProfileSamples {
		return "", 0
	}
	prof := resampleSamples(r.Samples, r.DurationSec, profilePoints)

	c.mu.RLock()
	defer c.mu.RUnlock()

	best, bestCorr := "", minMatchCorr
	for _, p := range c.programs {
		corr := pearson(prof, p.Profile)
		if corr >= bestCorr {
			bestCorr = corr
			best = p.Name
		}
	}
	if best == "" {
		return "", 0
	}
	return best, clamp01(bestCorr)
}
