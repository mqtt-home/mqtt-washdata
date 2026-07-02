package dryer

import (
	"encoding/json"
	"fmt"
)

// ShellyStatus mirrors the JSON published by a Shelly Plug PM 3 (Gen3) on
// "<topic>/status/switch:<id>". Only the fields relevant to power profiling are
// declared; unknown fields are ignored by encoding/json.
type ShellyStatus struct {
	ID      int     `json:"id"`
	Output  bool    `json:"output"`
	Apower  float64 `json:"apower"`  // active power in watts
	Voltage float64 `json:"voltage"` // volts
	Current float64 `json:"current"` // amps
	Pf      float64 `json:"pf"`      // power factor
	Freq    float64 `json:"freq"`    // Hz
	Aenergy struct {
		Total    float64 `json:"total"` // accumulated energy counter in Wh
		MinuteTs int64   `json:"minute_ts"`
	} `json:"aenergy"`
	Temperature struct {
		TC float64 `json:"tC"`
	} `json:"temperature"`
}

// ParseShellyStatus decodes a Shelly switch status payload.
func ParseShellyStatus(payload []byte) (*ShellyStatus, error) {
	var s ShellyStatus
	if err := json.Unmarshal(payload, &s); err != nil {
		return nil, fmt.Errorf("parse shelly status: %w", err)
	}
	return &s, nil
}

// StatusTopic returns the MQTT topic a Shelly publishes switch status on.
func StatusTopic(shellyTopic string, switchID int) string {
	return fmt.Sprintf("%s/status/switch:%d", shellyTopic, switchID)
}

// CommandTopic returns the MQTT topic used to poke the Shelly for a status update.
func CommandTopic(shellyTopic string, switchID int) string {
	return fmt.Sprintf("%s/command/switch:%d", shellyTopic, switchID)
}
