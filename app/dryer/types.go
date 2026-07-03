package dryer

import "time"

// PowerSample is one point on a recorded power profile.
type PowerSample struct {
	// Offset is the number of seconds since the run started.
	Offset int `json:"t"`
	// Power is the active power in watts at that offset.
	Power float64 `json:"power"`
}

// Run is a single dryer cycle, from power-on to power-off, together with its
// recorded power profile and (once finished) derived features and program label.
type Run struct {
	ID          string    `json:"id"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end,omitempty"`
	Finished    bool      `json:"finished"`
	DurationSec int       `json:"durationSec"`
	EnergyWh    float64   `json:"energyWh"`
	PeakPower   float64   `json:"peakPower"`
	MeanPower   float64   `json:"meanPower"`

	// Program is the effective program name shown in the UI: the user label when
	// the run has been labeled, otherwise the auto-detected program.
	Program string `json:"program"`
	// ProgramAuto is the program the classifier inferred (kept even after a user
	// relabels, for reference / accuracy tracking).
	ProgramAuto string `json:"programAuto"`
	// Confidence is the classifier confidence for ProgramAuto (0..1).
	Confidence float64 `json:"confidence"`
	// Labeled is true when a user assigned Program (authoritative for training).
	Labeled bool `json:"labeled"`

	Samples []PowerSample `json:"samples,omitempty"`
}

// State of the live dryer.
const (
	StateIdle    = "idle"
	StateRunning = "running"
)

// Phase of a live run (only meaningful while running).
const (
	// PhaseDrying: the program is actively working.
	PhaseDrying = "drying"
	// PhaseCooling: the heat source is off; drum and fan finish the cycle
	// ("Abkühlen"). The cycle result is already decided at this point.
	PhaseCooling = "cooling"
	// PhaseAntiCrease: the program is complete; the dryer only tumbles
	// periodically to prevent creases ("Knitterschutz").
	PhaseAntiCrease = "anti-crease"
)

// LiveStatus is the payload published to MQTT and broadcast to the web UI on
// every reading. It describes the current live run (or the idle state).
type LiveStatus struct {
	State string `json:"state"`
	// Phase is "drying" or "anti-crease" while running, empty when idle.
	Phase      string  `json:"phase,omitempty"`
	DryerName  string  `json:"dryerName"`
	Power      float64 `json:"power"`
	Program    string  `json:"program,omitempty"`
	Confidence float64 `json:"confidence"`
	EnergyWh   float64 `json:"energyWh"`
	ElapsedSec int     `json:"elapsedSec"`
	// RemainingSec is the estimated remaining runtime, or -1 when unknown.
	RemainingSec int `json:"remainingSec"`
	// Progress is 0..1, or -1 when unknown.
	Progress  float64    `json:"progress"`
	ETA       *time.Time `json:"eta,omitempty"`
	RunID     string     `json:"runId,omitempty"`
	UpdatedAt time.Time  `json:"updatedAt"`
}
