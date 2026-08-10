// Package jalali converts Gregorian dates to the Jalali (Persian/Solar
// Hijri) calendar, shared by the subscription page's expiry labels and the
// reseller Telegram bot's invoice/subscription date fields.
package jalali

// ToJalali converts a Gregorian date to Jalali (Solar Hijri) date. Uses a
// reference-date approach: counts days from a known reference point
// (2024-01-01 = 1402-10-11 JAL) and walks the Jalali calendar forward/backward.
func ToJalali(gy, gm, gd int) (jy, jm, jd int) {
	// Compute Julian Day Number for the input Gregorian date
	a := (14 - gm) / 12
	y := gy + 4800 - a
	m := gm + 12*a - 3
	jdn := gd + (153*m+2)/5 + 365*y + y/4 - y/100 + y/400 - 32045

	// Reference: 2024-01-01 = JDN 2460311 = 1402-10-11 JAL
	refJDN := 2460311
	days := int64(jdn - refJDN)
	jy, jm, jd = 1402, 10, 11

	// Walk forward
	for days > 0 {
		remaining := int64(MonthDays(jy, jm) - jd + 1)
		if days < remaining {
			jd += int(days)
			return
		}
		days -= remaining
		jm++
		if jm > 12 {
			jm = 1
			jy++
		}
		jd = 1
	}
	// Walk backward
	for days < 0 {
		jd += int(days)
		for jd < 1 {
			jm--
			if jm < 1 {
				jm = 12
				jy--
			}
			jd += MonthDays(jy, jm)
		}
		days = 0
	}
	return
}

// MonthDays returns the number of days in the given Jalali month/year.
func MonthDays(y, m int) int {
	if m <= 6 {
		return 31
	}
	if m <= 11 {
		return 30
	}
	if IsLeap(y) {
		return 30
	}
	return 29
}

// IsLeap reports whether the given Jalali year is a leap year. The leap
// pattern repeats every 33 years with 8 leap years.
func IsLeap(y int) bool {
	switch y % 33 {
	case 1, 5, 9, 13, 17, 22, 26, 30:
		return true
	}
	return false
}
