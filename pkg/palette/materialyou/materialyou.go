package materialyou

import (
	"math"
	"strings"
)

func diffDeg(a, b float64) float64 {
	return 180.0 - math.Abs(math.Abs(a-b)-180.0)
}

func rotDir(a, b float64) float64 {
	if sanitizeDegrees(b-a) <= 180.0 {
		return 1.0
	}
	return -1.0
}

func harmonizeHue(custHex, srcHex string) float64 {
	f := hctFromHex(custHex)
	t := hctFromHex(srcHex)
	d := diffDeg(f.Hue, t.Hue)
	x := d * 0.5
	rot := 15.0
	if x < 15.0 {
		rot = x
	}
	rot *= rotDir(f.Hue, t.Hue)
	return sanitizeDegrees(f.Hue + rot)
}

type ModeColors map[string]string

type CustomGroup struct {
	Dark  ModeColors
	Light ModeColors
}

func customGroup(name, custHex, srcHex string) CustomGroup {
	pal := newTonalPalette(harmonizeHue(custHex, srcHex), 48.0)
	lowerCust := strings.ToLower(custHex)
	return CustomGroup{
		Dark: ModeColors{
			name:                        pal.Tone(80.0),
			"on_" + name:                pal.Tone(20.0),
			name + "_container":         pal.Tone(30.0),
			"on_" + name + "_container": pal.Tone(90.0),
			name + "_source":            lowerCust,
			name + "_value":             lowerCust,
		},
		Light: ModeColors{
			name:                        pal.Tone(40.0),
			"on_" + name:                pal.Tone(100.0),
			name + "_container":         pal.Tone(90.0),
			"on_" + name + "_container": pal.Tone(10.0),
			name + "_source":            lowerCust,
			name + "_value":             lowerCust,
		},
	}
}

type Colorscheme struct {
	Dark  ModeColors
	Light ModeColors
}

func GenerateColorscheme(source string, customColors map[string]string) Colorscheme {
	darkRoles, lightRoles := colorsForBoth(source)

	customFor := func(isDark bool) map[string]string {
		res := make(map[string]string)
		for name, hex := range customColors {
			grp := customGroup(name, hex, source)
			var mode ModeColors
			if isDark {
				mode = grp.Dark
			} else {
				mode = grp.Light
			}
			for k, v := range mode {
				res[k] = v
			}
		}
		return res
	}

	modeColors := func(isDark bool, roleColors map[string]string) ModeColors {
		res := ModeColors{}
		for k, v := range roleColors {
			res[k] = v
		}
		for k, v := range customFor(isDark) {
			res[k] = v
		}
		res["source_color"] = strings.ToLower(source)
		return res
	}

	return Colorscheme{
		Dark:  modeColors(true, darkRoles),
		Light: modeColors(false, lightRoles),
	}
}
