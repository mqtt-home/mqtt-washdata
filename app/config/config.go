package config

import (
	"encoding/json"
	"os"

	"github.com/philipparndt/go-logger"
	"github.com/philipparndt/mqtt-gateway/config"
)

var cfg Config

type Config struct {
	MQTT     config.MQTTConfig `json:"mqtt"`
	Dryer    DryerConfig       `json:"dryer"`
	Web      WebConfig         `json:"web"`
	LogLevel string            `json:"loglevel,omitempty"`
}

type WebConfig struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

// DryerConfig configures which Shelly device is watched and how runs are detected.
type DryerConfig struct {
	// Name is a human readable label for the dryer (shown in the UI).
	Name string `json:"name"`
	// ShellyTopic is the MQTT topic prefix of the Shelly Plug PM 3.
	// Status is expected on "<ShellyTopic>/status/switch:<SwitchID>".
	ShellyTopic string `json:"shelly_topic"`
	// SwitchID is the Shelly switch component id (0 for a single-channel plug).
	SwitchID  int             `json:"switch_id"`
	Detection DetectionConfig `json:"detection"`
}

// DetectionConfig holds the thresholds that drive the run-detection state machine.
type DetectionConfig struct {
	// StartWatts: sustained power above this opens a new run.
	StartWatts float64 `json:"start_watts"`
	// StopWatts: sustained power below this closes the current run.
	StopWatts float64 `json:"stop_watts"`
	// StartDebounceSec: how long power must stay above StartWatts before a run starts.
	StartDebounceSec int `json:"start_debounce_sec"`
	// StopDebounceSec: how long power must stay below StopWatts before a run ends
	// (kept high to survive the dryer cool-down / anti-crease pauses).
	StopDebounceSec int `json:"stop_debounce_sec"`
	// SampleIntervalSec: the resolution at which the power profile is recorded.
	SampleIntervalSec int `json:"sample_interval_sec"`
	// HeaterWatts: power above this is considered "heating" (used for duty-cycle features).
	HeaterWatts float64 `json:"heater_watts"`
	// MinRunSec: runs shorter than this are discarded as noise.
	MinRunSec int `json:"min_run_sec"`
}

func LoadConfig(file string) (Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		logger.Error("Error reading config file", "error", err)
		return Config{}, err
	}

	data = config.ReplaceEnvVariables(data)

	if err := json.Unmarshal(data, &cfg); err != nil {
		logger.Error("Unmarshaling JSON", "error", err)
		return Config{}, err
	}

	applyDefaults(&cfg)
	return cfg, nil
}

func applyDefaults(c *Config) {
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.Web.Port == 0 {
		c.Web.Port = 8080
	}
	if c.Dryer.Name == "" {
		c.Dryer.Name = "Dryer"
	}
	if c.Dryer.ShellyTopic == "" {
		c.Dryer.ShellyTopic = "shelly/dryer"
	}

	d := &c.Dryer.Detection
	if d.StartWatts == 0 {
		d.StartWatts = 20
	}
	if d.StopWatts == 0 {
		d.StopWatts = 5
	}
	if d.StartDebounceSec == 0 {
		d.StartDebounceSec = 20
	}
	if d.StopDebounceSec == 0 {
		d.StopDebounceSec = 180
	}
	if d.SampleIntervalSec == 0 {
		d.SampleIntervalSec = 10
	}
	if d.HeaterWatts == 0 {
		d.HeaterWatts = 800
	}
	if d.MinRunSec == 0 {
		d.MinRunSec = 120
	}
}

func Get() Config {
	return cfg
}
