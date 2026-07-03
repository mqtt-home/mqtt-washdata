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

// rpcFrame is the envelope of the RPC notifications a Gen2+/Gen3 Shelly
// publishes on "<topic>/events/rpc" (NotifyStatus / NotifyFullStatus). Params
// holds one entry per component that changed, keyed like "pm1:0" or "switch:0".
type rpcFrame struct {
	Method string                     `json:"method"`
	Params map[string]json.RawMessage `json:"params"`
}

// ParseShellyMessage decodes a power reading from either message style a Shelly
// publishes: a plain component status (on "<topic>/status/switch:<id>") or an
// RPC NotifyStatus/NotifyFullStatus frame (on "<topic>/events/rpc"). The power
// meter appears as "switch:<id>" on relay plugs and as "pm1:<id>" on
// meter-profile devices. RPC notifications are incremental — a frame that
// carries no power component (e.g. a sys-only update) yields (nil, nil).
func ParseShellyMessage(payload []byte, switchID int) (*ShellyStatus, error) {
	var frame rpcFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return nil, fmt.Errorf("parse shelly message: %w", err)
	}
	if frame.Method == "" {
		return ParseShellyStatus(payload)
	}
	for _, key := range []string{
		fmt.Sprintf("switch:%d", switchID),
		fmt.Sprintf("pm1:%d", switchID),
	} {
		raw, ok := frame.Params[key]
		if !ok {
			continue
		}
		var s ShellyStatus
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse shelly %s status: %w", key, err)
		}
		return &s, nil
	}
	return nil, nil
}

// StatusTopic returns the MQTT topic a Shelly publishes plain switch status on.
func StatusTopic(shellyTopic string, switchID int) string {
	return fmt.Sprintf("%s/status/switch:%d", shellyTopic, switchID)
}

// RpcTopic returns the MQTT topic a Shelly publishes RPC notifications on.
func RpcTopic(shellyTopic string) string {
	return shellyTopic + "/events/rpc"
}

// CommandTopic returns the MQTT topic used to poke the Shelly for a status
// update (payload "status_update" triggers a NotifyFullStatus on the RPC topic).
func CommandTopic(shellyTopic string) string {
	return shellyTopic + "/command"
}
