package ui

import (
	"strconv"
	"strings"
	"time"
)

// formatDay writes a date for a person in the viewer's language: "2 Jan 2006"
// in English, "2 ene 2006" in Spanish, "2006年1月2日" in Japanese. The order
// comes from the key date.short ({d}, {m}, {mon}, {y}) and the month names
// from date.mon.1 to date.mon.12; without a translator, or without those
// keys, it is the English form.
func formatDay(t func(key string, args ...any) string, tm time.Time) string {
	english := tm.Format("2 Jan 2006")
	if t == nil {
		return english
	}
	pattern, mon := t("date.short"), t("date.mon."+strconv.Itoa(int(tm.Month())))
	if pattern == "" || pattern == "date.short" || mon == "" || strings.HasPrefix(mon, "date.mon.") {
		return english
	}
	return strings.NewReplacer(
		"{d}", strconv.Itoa(tm.Day()),
		"{m}", strconv.Itoa(int(tm.Month())),
		"{mon}", mon,
		"{y}", strconv.Itoa(tm.Year()),
	).Replace(pattern)
}
