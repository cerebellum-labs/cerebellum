package dashboard

import (
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
)

type statsView struct {
	Total        int64               `json:"total"`
	BySource     map[string]int64    `json:"by_source"`
	ByStatus     map[string]int64    `json:"by_status"`
	ByClass      map[string]int64    `json:"by_class"`
	Corrections  int64               `json:"corrections"`
	CostUsd      float64             `json:"cost_usd"`
	AvgLatencyMs map[string]*float64 `json:"avg_latency_ms"`
}

func newStatsView(d db.DecisionStatsRow, m db.ModelCallStatsRow) statsView {
	v := statsView{
		Total:    d.Total,
		BySource: map[string]int64{"device": d.SourceDevice, "server": d.SourceServer},
		ByStatus: map[string]int64{
			"decided_on_device":  d.StatusDecidedOnDevice,
			"waiting_for_server": d.StatusWaitingForServer,
			"decided_on_server":  d.StatusDecidedOnServer,
			"executed":           d.StatusExecuted,
			"corrected":          d.StatusCorrected,
			"cancelled":          d.StatusCancelled,
		},
		ByClass: map[string]int64{
			"reflex":       d.ClassReflex,
			"reversible":   d.ClassReversible,
			"irreversible": d.ClassIrreversible,
		},
		Corrections: d.Corrections,
		CostUsd:     m.CostUsd,
		// Null until there is something to average: older agents don't
		// report how long the local model took.
		AvgLatencyMs: map[string]*float64{"device": nil, "server": nil},
	}
	if d.DeviceLatencies > 0 {
		avg := d.AvgDeviceLatencyMs
		v.AvgLatencyMs["device"] = &avg
	}
	if m.DecideCalls > 0 {
		avg := m.AvgDecideLatencyMs
		v.AvgLatencyMs["server"] = &avg
	}
	return v
}
