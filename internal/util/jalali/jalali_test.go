package jalali

import "testing"

func TestToJalali(t *testing.T) {
	cases := []struct {
		gy, gm, gd int
		jy, jm, jd int
	}{
		{2024, 1, 1, 1402, 10, 11},
		{2000, 3, 20, 1379, 1, 1},
		{1979, 2, 11, 1357, 11, 22},
	}
	for _, c := range cases {
		jy, jm, jd := ToJalali(c.gy, c.gm, c.gd)
		if jy != c.jy || jm != c.jm || jd != c.jd {
			t.Errorf("ToJalali(%d,%d,%d) = (%d,%d,%d), want (%d,%d,%d)",
				c.gy, c.gm, c.gd, jy, jm, jd, c.jy, c.jm, c.jd)
		}
	}
}
