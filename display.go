package main

import (
	"fmt"
	"strconv"
	"strings"
)

// ANSI colour codes
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	cyan   = "\033[36m"
	grey   = "\033[90m"
)

const boxWidth = 50 // visible characters between the two │ borders

// row wraps content in box borders, padding to boxWidth.
func row(content string) string {
	pad := boxWidth - visLen(content)
	if pad < 0 {
		pad = 0
	}
	return bold + "│" + reset + content + strings.Repeat(" ", pad) + bold + "│" + reset
}

func divider() string {
	return bold + "├" + strings.Repeat("─", boxWidth) + "┤" + reset
}

func printBattery(s *BatterySnapshot, fixedLoadW float64) {
	top := bold + "┌" + strings.Repeat("─", boxWidth) + "┐" + reset
	bot := bold + "└" + strings.Repeat("─", boxWidth) + "┘" + reset

	model := deref(s.DeviceModel, "Felicity Battery")
	alias := deref(s.Alias, s.DeviceSn)
	updated := s.DataTimeStr
	if updated == "" {
		updated = "–"
	}

	fmt.Println()
	fmt.Println(top)
	fmt.Println(row(fmt.Sprintf("  %-24s%24s", model, alias)))
	fmt.Println(row(fmt.Sprintf("  %-24s%24s", "Last update:", updated)))
	fmt.Println(divider())

	// Parse electrical values early — needed for the ETA estimate
	soc := parseFloat(s.BattSoc)
	cap := parseFloat(s.BattCapacity)
	volt := parseFloat(s.BattVolt)
	curr := parseFloat(s.BattCurr)
	pwr := parseFloat(s.BmsPower)
	dir, dirColour := currentDir(curr)

	bar := socBar(soc, 28)
	fmt.Println(row(fmt.Sprintf("  SOC %s%s%s %s%.0f%%%s",
		socColour(soc), bar, reset, bold, soc, reset)))

	soh := deref(s.BattSoh, "–")
	status := statusLabel(deref(s.Status, ""))
	fmt.Println(row(fmt.Sprintf("  Status: %s   SOH: %s%s%%%s",
		status, green, soh, reset)))

	if eta := timeEstimate(soc, cap, curr, volt, 0); eta != "" {
		fmt.Println(row(fmt.Sprintf("  %s", eta)))
	}
	if fixedLoadW > 0 && volt > 0 {
		fixedCurr := -(fixedLoadW / volt)
		if eta := timeEstimate(soc, cap, fixedCurr, volt, fixedLoadW); eta != "" {
			fmt.Println(row(fmt.Sprintf("  %s", eta)))
		}
	}

	fmt.Println(divider())

	// Electrical

	fmt.Println(row(fmt.Sprintf("  Voltage  %s%6.2f V%s", cyan, volt, reset)))
	fmt.Println(row(fmt.Sprintf("  Current  %s%6.2f A  %s%s",
		dirColour, curr, dir, reset)))
	fmt.Println(row(fmt.Sprintf("  Power    %s%6.2f W%s", dirColour, pwr, reset)))

	fmt.Println(divider())

	// Thermal
	tMax := parseFloat(s.TempMax)
	tMin := parseFloat(s.TempMin)
	fmt.Println(row(fmt.Sprintf("  Temperature  %s%.0f°C%s (max)   %.0f°C (min)",
		tempColour(tMax), tMax, reset, tMin)))

	// Cell voltages
	active := filterActive(s.cellVolts())
	if len(active) > 0 {
		fmt.Println(divider())
		fmt.Println(row("  Cells (V):"))
		for i := 0; i < len(active); i += 4 {
			end := i + 4
			if end > len(active) {
				end = len(active)
			}
			var sb strings.Builder
			sb.WriteString("  ")
			for j, mv := range active[i:end] {
				v, _ := strconv.Atoi(mv)
				fmt.Fprintf(&sb, "%2d:%s%.3f%s  ", i+j+1, cellVoltColour(v), float64(v)/1000.0, reset)
			}
			fmt.Println(row(sb.String()))
		}
	}

	// Cell temperatures
	activeTemps := filterActive(s.cellTemps())
	if len(activeTemps) > 0 {
		var sb strings.Builder
		sb.WriteString("  Temp: ")
		for i, t := range activeTemps {
			v, _ := strconv.ParseFloat(t, 64)
			fmt.Fprintf(&sb, "%s%d:%.0f°C%s  ", tempColour(v), i+1, v, reset)
		}
		fmt.Println(row(sb.String()))
	}

	fmt.Println(divider())

	wifi := deref(s.WifiSignal, "–")
	capStr := deref(s.BattCapacity, "–")
	fmt.Println(row(fmt.Sprintf("%s  WiFi: %s dBm  Capacity: %s Ah  Refresh: %ds%s",
		grey, wifi, capStr, s.ReportFreq, reset)))

	fmt.Println(bot)
	fmt.Println()
}

// --- helpers ---

func deref(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

func parseFloat(s *string) float64 {
	if s == nil {
		return 0
	}
	v, _ := strconv.ParseFloat(*s, 64)
	return v
}

func socBar(soc float64, w int) string {
	filled := int(soc / 100.0 * float64(w))
	if filled > w {
		filled = w
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", w-filled) + "]"
}

func socColour(soc float64) string {
	switch {
	case soc > 50:
		return green
	case soc > 20:
		return yellow
	default:
		return red
	}
}

func currentDir(curr float64) (string, string) {
	switch {
	case curr < -0.05:
		return "↓ discharging", red
	case curr > 0.05:
		return "↑ charging", green
	default:
		return "  idle", grey
	}
}

func tempColour(t float64) string {
	switch {
	case t > 40:
		return red
	case t > 30:
		return yellow
	default:
		return cyan
	}
}

func cellVoltColour(mv int) string {
	switch {
	case mv > 3400:
		return green
	case mv > 3200:
		return cyan
	case mv > 3000:
		return yellow
	default:
		return red
	}
}

func statusLabel(s string) string {
	switch s {
	case "NM":
		return green + "Normal" + reset
	case "":
		return grey + "–" + reset
	default:
		return yellow + s + reset
	}
}

// filterActive removes nil entries and the sentinel "32767" (unpopulated slots).
func filterActive(ptrs []*string) []string {
	var out []string
	for _, p := range ptrs {
		if p != nil && *p != "32767" {
			out = append(out, *p)
		}
	}
	return out
}

// timeEstimate returns a human-readable time estimate based on current SOC,
// capacity (Ah) and current (A). Returns "" when idle or data is unavailable.
// timeEstimate returns a human-readable runtime estimate.
// fixedLoadW > 0 means this is a projection for a known load, not the live reading.
func timeEstimate(soc, capacityAh, currentA, voltV, fixedLoadW float64) string {
	if capacityAh <= 0 {
		return ""
	}
	remainingAh := (soc / 100.0) * capacityAh

	switch {
	case currentA < -0.05: // discharging
		hoursLeft := remainingAh / (-currentA)
		hoursFull := capacityAh / (-currentA)
		watts := (-currentA) * voltV

		if fixedLoadW > 0 {
			return fmt.Sprintf("%s@ %.0fW: ~%s remaining%s  (full→%s%s%s)",
				grey, fixedLoadW, red+formatHours(hoursLeft)+reset,
				grey, yellow, formatHours(hoursFull), reset)
		}
		return fmt.Sprintf("%s~%s remaining%s  (%.0fW · full→%s%s%s)",
			red, formatHours(hoursLeft)+reset,
			grey, watts, yellow, formatHours(hoursFull), reset)

	case currentA > 0.05: // charging
		toFullAh := (1.0 - soc/100.0) * capacityAh
		hoursToFull := toFullAh / currentA
		hoursLeft := remainingAh / currentA // discharge estimate at same rate
		return fmt.Sprintf("%s~%s to full%s  (~%s stored)",
			green, formatHours(hoursToFull)+reset,
			grey, yellow+formatHours(hoursLeft)+reset)

	default:
		return ""
	}
}

func formatHours(h float64) string {
	total := int(h * 60)
	hours := total / 60
	mins := total % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// visLen returns the number of visible terminal columns in s,
// ignoring ANSI escape sequences and counting multi-byte runes as 1 column.
func visLen(s string) int {
	inEsc := false
	n := 0
	for _, c := range s {
		switch {
		case c == '\033':
			inEsc = true
		case inEsc:
			if c == 'm' {
				inEsc = false
			}
		default:
			n++
		}
	}
	return n
}
