package vnstat

type Report struct {
	Installed bool            `json:"installed"`
	Message   string          `json:"message,omitempty"`
	Version   string          `json:"version,omitempty"`
	Days      []TrafficPeriod `json:"days,omitempty"`
	Months    []TrafficPeriod `json:"months,omitempty"`
	Total     *TrafficPeriod  `json:"total,omitempty"`
}

type TrafficPeriod struct {
	Label   string `json:"label"`
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}
