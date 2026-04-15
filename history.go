package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// HistoryRecord holds one decoded battery reading ready for charting.
// All values are already converted to human units (V, A, W, %, °C, mV).
type HistoryRecord struct {
	Timestamp  string    `json:"ts"`
	DeviceSn   string    `json:"device_sn"`
	SOC        float64   `json:"soc"`
	SOH        float64   `json:"soh"`
	CapacityAh float64   `json:"capacity_ah"`
	VoltV      float64   `json:"volt_v"`
	CurrA      float64   `json:"curr_a"`
	PowerW     float64   `json:"power_w"`
	TempMaxC   float64   `json:"temp_max_c"`
	TempMinC   float64   `json:"temp_min_c"`
	CellVoltMv []int     `json:"cell_volts_mv"` // active cells only; 32767 = unpopulated, excluded
	CellTempC  []float64 `json:"cell_temps_c"`  // active sensors only
	Status     string    `json:"status"`

	// Estimations — null when current is too low (<1 A) or direction doesn't apply.
	HoursLeft   *float64 `json:"hours_left"`    // hours until empty at current discharge rate
	HoursToFull *float64 `json:"hours_to_full"` // hours until full at current charge rate
}

// snapshotToRecord converts a BatterySnapshot into a HistoryRecord using wall-clock time.
func snapshotToRecord(s *BatterySnapshot) HistoryRecord {
	r := HistoryRecord{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		DeviceSn:   s.DeviceSn,
		SOC:        parseFloat(s.BattSoc),
		SOH:        parseFloat(s.BattSoh),
		CapacityAh: parseFloat(s.BattCapacity),
		VoltV:      parseFloat(s.BattVolt),
		CurrA:      parseFloat(s.BattCurr),
		PowerW:     parseFloat(s.BmsPower),
		TempMaxC:   parseFloat(s.TempMax),
		TempMinC:   parseFloat(s.TempMin),
		Status:     deref(s.Status, ""),
	}

	for _, p := range filterActive(s.cellVolts()) {
		mv, err := strconv.Atoi(p)
		if err == nil {
			r.CellVoltMv = append(r.CellVoltMv, mv)
		}
	}

	for _, p := range filterActive(s.cellTemps()) {
		v, err := strconv.ParseFloat(p, 64)
		if err == nil {
			r.CellTempC = append(r.CellTempC, v)
		}
	}

	const minCurrentA = 1.0 // below this threshold, estimates are meaningless
	remainingAh := (r.SOC / 100) * r.CapacityAh
	toFillAh := (1 - r.SOC/100) * r.CapacityAh

	if r.CurrA < -minCurrentA {
		h := remainingAh / (-r.CurrA)
		r.HoursLeft = &h
	}
	if r.CurrA > minCurrentA {
		h := toFillAh / r.CurrA
		r.HoursToFull = &h
	}

	return r
}

// readHistory reads the JSONL file at path, filters by from/to time range,
// and returns up to limit records starting at offset, ordered newest-first.
// Zero values of from/to mean no lower/upper bound. Malformed lines are skipped.
func readHistory(path string, from, to time.Time, limit, offset int) ([]HistoryRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []HistoryRecord{}, nil
		}
		return nil, fmt.Errorf("open history: %w", err)
	}
	defer f.Close()

	var all []HistoryRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MB line buffer
	for scanner.Scan() {
		var rec HistoryRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue // skip malformed lines (e.g. partial write on crash)
		}
		if !from.IsZero() && rec.Timestamp < from.UTC().Format(time.RFC3339) {
			continue
		}
		if !to.IsZero() && rec.Timestamp > to.UTC().Format(time.RFC3339) {
			continue
		}
		all = append(all, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan history: %w", err)
	}

	// Reverse to newest-first
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	// Apply offset and limit
	if offset >= len(all) {
		return []HistoryRecord{}, nil
	}
	all = all[offset:]
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// AppendHistory serialises rec as a single JSON line and appends it to path.
// The parent directory is created if it does not exist.
func AppendHistory(path string, snap *BatterySnapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	rec := snapshotToRecord(snap)

	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s\n", line)
	return err
}
