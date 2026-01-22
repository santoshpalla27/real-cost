// Package units provides canonical unit types and conversions.
package units

import "time"

// Unit represents a measurable quantity.
type Unit string

const (
	// Time units
	UnitHours  Unit = "hours"
	UnitDays   Unit = "days"
	UnitMonths Unit = "months"

	// Storage units
	UnitGB      Unit = "GB"
	UnitGBMonth Unit = "GB-month"
	UnitTB      Unit = "TB"
	UnitIOPS    Unit = "IOPS"

	// Network units
	UnitGBTransfer Unit = "GB-transfer"
	UnitRequests   Unit = "requests"

	// Compute units
	UnitVCPU       Unit = "vCPU"
	UnitVCPUHours  Unit = "vCPU-hours"
	UnitGBMemory   Unit = "GB-memory"
)

// HoursPerMonth is the standard billing assumption.
const HoursPerMonth = 730.0

// ToMonthlyHours converts various time periods to monthly hours.
func ToMonthlyHours(value float64, unit Unit) float64 {
	switch unit {
	case UnitHours:
		return value
	case UnitDays:
		return value * 24
	case UnitMonths:
		return value * HoursPerMonth
	default:
		return value
	}
}

// GBToTB converts gigabytes to terabytes.
func GBToTB(gb float64) float64 {
	return gb / 1024
}

// TBToGB converts terabytes to gigabytes.
func TBToGB(tb float64) float64 {
	return tb * 1024
}

// MonthlyToDaily calculates daily cost from monthly.
func MonthlyToDaily(monthly float64) float64 {
	return monthly / 30.0
}

// DailyToMonthly calculates monthly cost from daily.
func DailyToMonthly(daily float64) float64 {
	return daily * 30.0
}

// HourlyToMonthly calculates monthly cost from hourly.
func HourlyToMonthly(hourly float64) float64 {
	return hourly * HoursPerMonth
}

// NormalizeTimePeriod converts a duration to standard monthly period.
func NormalizeTimePeriod(value float64, period time.Duration) float64 {
	hours := period.Hours()
	if hours == 0 {
		return 0
	}
	monthlyFactor := HoursPerMonth / hours
	return value * monthlyFactor
}
