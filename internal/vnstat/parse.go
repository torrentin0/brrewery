package vnstat

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type jsonReport struct {
	VnstatVersion string          `json:"vnstatversion"`
	Interfaces    []jsonInterface `json:"interfaces"`
}

type jsonInterface struct {
	Name    string      `json:"name"`
	Traffic jsonTraffic `json:"traffic"`
}

type jsonTraffic struct {
	Total *jsonTotal   `json:"total"`
	Day   []jsonSample `json:"day"`
	Month []jsonSample `json:"month"`
}

type jsonTotal struct {
	Rx uint64 `json:"rx"`
	Tx uint64 `json:"tx"`
}

type jsonSample struct {
	Date jsonDate `json:"date"`
	Rx   uint64   `json:"rx"`
	Tx   uint64   `json:"tx"`
}

type jsonDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

func parseReport(dayJSON, monthJSON []byte) (Report, error) {
	days, err := parsePeriods(dayJSON, periodDay)
	if err != nil {
		return Report{}, err
	}

	months, err := parsePeriods(monthJSON, periodMonth)
	if err != nil {
		return Report{}, err
	}

	total := parseTotal(dayJSON)
	if total == nil {
		total = parseTotal(monthJSON)
	}

	version := ""
	if len(dayJSON) > 0 {
		var meta jsonReport
		if err := json.Unmarshal(dayJSON, &meta); err == nil {
			version = meta.VnstatVersion
		}
	}

	return Report{
		Installed: true,
		Version:   version,
		Days:      days,
		Months:    months,
		Total:     total,
	}, nil
}

func parseTotal(data []byte) *TrafficPeriod {
	if len(data) == 0 {
		return nil
	}
	var raw jsonReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	var totalRx, totalTx uint64
	hasData := false
	for _, iface := range raw.Interfaces {
		if skipInterface(iface.Name) || iface.Traffic.Total == nil {
			continue
		}
		totalRx += iface.Traffic.Total.Rx
		totalTx += iface.Traffic.Total.Tx
		hasData = true
	}

	if !hasData {
		return nil
	}

	return &TrafficPeriod{
		Label:   "All Time",
		RxBytes: totalRx,
		TxBytes: totalTx,
	}
}

type periodKind int

const (
	periodDay periodKind = iota
	periodMonth
)

func parsePeriods(data []byte, kind periodKind) ([]TrafficPeriod, error) {
	var raw jsonReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse vnstat json: %w", err)
	}

	totals := map[string]TrafficPeriod{}

	for _, iface := range raw.Interfaces {
		if skipInterface(iface.Name) {
			continue
		}

		samples := iface.Traffic.Day
		if kind == periodMonth {
			samples = iface.Traffic.Month
		}

		for _, sample := range samples {
			label, err := periodLabel(sample.Date, kind)
			if err != nil {
				continue
			}

			existing := totals[label]
			existing.Label = label
			existing.RxBytes += sample.Rx
			existing.TxBytes += sample.Tx
			totals[label] = existing
		}
	}

	periods := make([]TrafficPeriod, 0, len(totals))
	for _, period := range totals {
		periods = append(periods, period)
	}

	sort.Slice(periods, func(i, j int) bool {
		return periods[i].Label < periods[j].Label
	})

	return periods, nil
}

func periodLabel(date jsonDate, kind periodKind) (string, error) {
	if date.Year == 0 || date.Month == 0 {
		return "", fmt.Errorf("invalid date")
	}
	if kind == periodMonth {
		return fmt.Sprintf("%04d-%02d", date.Year, date.Month), nil
	}
	if date.Day == 0 {
		return "", fmt.Errorf("invalid day")
	}
	return fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day), nil
}

func skipInterface(name string) bool {
	switch name {
	case "lo":
		return true
	default:
		return strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "virbr")
	}
}
