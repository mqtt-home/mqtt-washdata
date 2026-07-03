package dryer

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/mqtt-home/mqtt-washdata/config"
	"github.com/philipparndt/go-logger"
	"github.com/philipparndt/mqtt-gateway/mqtt"
)

// ErrRunNotFound is returned when a run id is unknown.
var ErrRunNotFound = fmt.Errorf("run not found")

// Manager wires the Shelly power stream through detection, persistence, learning
// and estimation, and fans the resulting live status out to MQTT and the web UI.
type Manager struct {
	cfg        config.DryerConfig
	pubTopic   string // base topic to publish status on
	retain     bool
	detector   *Detector
	store      *Store
	classifier *Classifier

	procMu sync.Mutex // serializes readings (MQTT callbacks may be concurrent)

	mu       sync.RWMutex
	live     LiveStatus
	onStatus func(LiveStatus)
}

func NewManager(cfg config.DryerConfig, pubTopic string, retain bool, store *Store) *Manager {
	return &Manager{
		cfg:        cfg,
		pubTopic:   pubTopic,
		retain:     retain,
		detector:   NewDetector(cfg.Detection),
		store:      store,
		classifier: NewClassifier(),
		live:       LiveStatus{State: StateIdle, DryerName: cfg.Name, RemainingSec: -1, Progress: -1},
	}
}

// SetStatusCallback registers a callback invoked on every live-status update
// (used by the web server to broadcast over SSE).
func (m *Manager) SetStatusCallback(fn func(LiveStatus)) {
	m.mu.Lock()
	m.onStatus = fn
	m.mu.Unlock()
}

// Start learns from history and subscribes to the Shelly status topic.
func (m *Manager) Start() {
	m.classifier.Build(m.store.All())

	// Subscribe to both message styles: plain component status and Gen2+/Gen3
	// RPC notifications — which one a device uses depends on its MQTT settings.
	for _, topic := range []string{
		StatusTopic(m.cfg.ShellyTopic, m.cfg.SwitchID),
		RpcTopic(m.cfg.ShellyTopic),
	} {
		logger.Info("Subscribing to Shelly status", "topic", topic)
		mqtt.Subscribe(topic, m.onShellyMessage)
	}

	// Poke the device so we get a fresh reading right away.
	mqtt.PublishAbsolute(CommandTopic(m.cfg.ShellyTopic), "status_update", false)

	m.updateLive(time.Now(), 0)
}

func (m *Manager) onShellyMessage(topic string, payload []byte) {
	st, err := ParseShellyMessage(payload, m.cfg.SwitchID)
	if err != nil {
		logger.Warn("Failed to parse Shelly status", "topic", topic, "error", err)
		return
	}
	if st == nil {
		// RPC notification without power data (e.g. a sys-only update).
		return
	}
	m.process(time.Now(), st.Apower, st.Aenergy.Total, st.Aenergy.Total > 0)
}

// process handles a single reading (serialized).
func (m *Manager) process(ts time.Time, power, energyTotal float64, hasEnergy bool) {
	m.procMu.Lock()
	defer m.procMu.Unlock()

	ev := m.detector.Feed(ts, power, energyTotal, hasEnergy)
	switch ev.Type {
	case EventStarted:
		logger.Info("Dryer run started", "id", ev.Run.ID, "power", round1(power))
	case EventFinished:
		m.finish(ev.Run)
	}
	m.updateLive(ts, power)
}

func (m *Manager) finish(run *Run) {
	if run.DurationSec < m.cfg.Detection.MinRunSec {
		logger.Info("Discarding short run", "id", run.ID, "durationSec", run.DurationSec)
		return
	}

	name, conf := m.classifier.ClassifyFull(run)
	run.ProgramAuto = name
	run.Confidence = round2(conf)
	if run.Program == "" {
		run.Program = name
	}

	if err := m.store.Save(run); err != nil {
		logger.Error("Failed to save run", "id", run.ID, "error", err)
		return
	}
	// Learn: rebuild programs including this run.
	m.classifier.Build(m.store.All())

	logger.Info("Dryer run finished",
		"id", run.ID,
		"durationSec", run.DurationSec,
		"energyWh", run.EnergyWh,
		"peakW", run.PeakPower,
		"program", run.Program,
		"confidence", run.Confidence,
	)
}

func (m *Manager) updateLive(ts time.Time, power float64) {
	ls := LiveStatus{
		DryerName:    m.cfg.Name,
		Power:        round1(power),
		UpdatedAt:    ts,
		RemainingSec: -1,
		Progress:     -1,
	}

	if m.detector.Running() {
		cur := m.detector.Current()
		elapsed := int(ts.Sub(cur.Start).Seconds())
		energy := m.detector.LiveEnergyWh()
		est := m.classifier.EstimatePartial(cur.Samples, elapsed, energy)

		ls.State = StateRunning
		ls.RunID = cur.ID
		ls.ElapsedSec = elapsed
		ls.EnergyWh = energy
		ls.Program = est.Program
		ls.Confidence = round2(est.Confidence)
		ls.RemainingSec = est.RemainingSec
		ls.Progress = round2(est.Progress)
		if est.RemainingSec >= 0 {
			eta := ts.Add(time.Duration(est.RemainingSec) * time.Second)
			ls.ETA = &eta
		}
	} else {
		ls.State = StateIdle
	}

	m.mu.Lock()
	m.live = ls
	cb := m.onStatus
	m.mu.Unlock()

	m.publish(ls)
	if cb != nil {
		cb(ls)
	}
}

func (m *Manager) publish(ls LiveStatus) {
	data, err := json.Marshal(ls)
	if err != nil {
		logger.Error("Failed to marshal live status", "error", err)
		return
	}
	mqtt.PublishAbsolute(m.pubTopic+"/status", string(data), m.retain)
}

// Live returns the current live status snapshot.
func (m *Manager) Live() LiveStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.live
}

// Runs returns all stored runs, newest first.
func (m *Manager) Runs() []*Run { return m.store.All() }

// Run returns a single run by id.
func (m *Manager) Run(id string) (*Run, bool) { return m.store.Get(id) }

// Programs returns the learned programs.
func (m *Manager) Programs() []*Program { return m.classifier.Programs() }

// LabelRun assigns (or clears, when program is empty) the program label of a run
// and re-learns. Clearing reverts the run to its auto-detected program.
func (m *Manager) LabelRun(id, program string) (*Run, error) {
	r, ok := m.store.Get(id)
	if !ok {
		return nil, ErrRunNotFound
	}
	if program == "" {
		r.Labeled = false
		r.Program = r.ProgramAuto
	} else {
		r.Labeled = true
		r.Program = program
	}
	if err := m.store.Save(r); err != nil {
		return nil, err
	}
	m.classifier.Build(m.store.All())
	logger.Info("Run relabeled", "id", id, "program", r.Program, "labeled", r.Labeled)
	return r, nil
}

// ImportRuns upserts finished runs (e.g. exported from another instance) and
// re-learns. Invalid entries are skipped. Imported runs get their idle tail
// trimmed, so recordings made with too-low stop thresholds (runs that dragged
// on through anti-crease / standby) are sanitized on the way in. Returns
// (imported, skipped).
func (m *Manager) ImportRuns(runs []*Run) (int, int, error) {
	imported, skipped := 0, 0
	for _, r := range runs {
		if r == nil || r.ID == "" || !r.Finished || r.DurationSec <= 0 || len(r.Samples) == 0 {
			skipped++
			continue
		}
		TrimRunTail(r, m.cfg.Detection)
		if err := m.store.Save(r); err != nil {
			return imported, skipped, err
		}
		imported++
	}
	if imported > 0 {
		m.classifier.Build(m.store.All())
	}
	logger.Info("Runs imported", "imported", imported, "skipped", skipped)
	return imported, skipped, nil
}

// DeleteRun removes a run and re-learns.
func (m *Manager) DeleteRun(id string) error {
	if _, ok := m.store.Get(id); !ok {
		return ErrRunNotFound
	}
	if err := m.store.Delete(id); err != nil {
		return err
	}
	m.classifier.Build(m.store.All())
	logger.Info("Run deleted", "id", id)
	return nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
