package materialyou

import "math"

func lerp(a, b, t float64) float64 {
	return (1.0-t)*a + t*b
}

func between(lo, hi, x float64) bool {
	return x >= lo && x < hi
}

func ratioOfYs(y1, y2 float64) float64 {
	li := max(y1, y2)
	dk := min(y1, y2)
	return (li + 5.0) / (dk + 5.0)
}

func ratioOfTones(a, b float64) float64 {
	return ratioOfYs(yFromLstar(clamp(0.0, 100.0, a)), yFromLstar(clamp(0.0, 100.0, b)))
}

func clamp(lo, hi, x float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func lighter(tone, ratio float64) float64 {
	if !between(0.0, 100.01, tone) {
		return -1.0
	}
	darkY := yFromLstar(tone)
	lightY := ratio*(darkY+5.0) - 5.0
	rc := ratioOfYs(lightY, darkY)
	d := math.Abs(rc - ratio)
	if rc < ratio && d > 0.04 {
		return -1.0
	}
	rv := lstarFromY(lightY) + 0.4
	if rv < 0.0 || rv > 100.0 {
		return -1.0
	}
	return rv
}

func darker(tone, ratio float64) float64 {
	if !between(0.0, 100.01, tone) {
		return -1.0
	}
	lightY := yFromLstar(tone)
	darkY := (lightY+5.0)/ratio - 5.0
	rc := ratioOfYs(lightY, darkY)
	d := math.Abs(rc - ratio)
	if rc < ratio && d > 0.04 {
		return -1.0
	}
	rv := lstarFromY(darkY) - 0.4
	if rv < 0.0 || rv > 100.0 {
		return -1.0
	}
	return rv
}

func lighterUnsafe(tone, ratio float64) float64 {
	x := lighter(tone, ratio)
	if x < 0.0 {
		return 100.0
	}
	return x
}

func darkerUnsafe(tone, ratio float64) float64 {
	x := darker(tone, ratio)
	if x < 0.0 {
		return 0.0
	}
	return x
}

func tonePrefersLight(tone float64) bool {
	return round(tone) < 60.0
}

func foregroundTone(bgTone, ratio float64) float64 {
	lt := lighterUnsafe(bgTone, ratio)
	dt := darkerUnsafe(bgTone, ratio)
	lr := ratioOfTones(lt, bgTone)
	dr := ratioOfTones(dt, bgTone)

	if tonePrefersLight(bgTone) {
		if lr >= ratio || lr >= dr || (math.Abs(lr-dr) < 0.1 && lr < ratio && dr < ratio) {
			return lt
		}
		return dt
	} else if dr >= ratio || dr >= lr {
		return dt
	}
	return lt
}

type ContrastCurve struct {
	Low    float64
	Normal float64
	Medium float64
	High   float64
}

func cc(low, normal, medium, high float64) *ContrastCurve {
	return &ContrastCurve{Low: low, Normal: normal, Medium: medium, High: high}
}

func ccGet(cc *ContrastCurve, cl float64) float64 {
	if cc == nil {
		return 0.0
	}
	if cl <= -1.0 {
		return cc.Low
	} else if cl < 0.0 {
		return lerp(cc.Low, cc.Normal, cl+1.0)
	} else if cl < 0.5 {
		return lerp(cc.Normal, cc.Medium, cl/0.5)
	} else if cl < 1.0 {
		return lerp(cc.Medium, cc.High, (cl-0.5)/0.5)
	}
	return cc.High
}

type Scheme struct {
	IsDark         bool
	ContrastLevel  float64
	SourceColorHct HCT
	Palettes       PaletteSet
}

type ToneDeltaPair struct {
	Subject      *Role
	Basis        *Role
	Delta        float64
	Polarity     string
	StayTogether bool
}

type Role struct {
	Name             string
	Palette          string
	IsBackground     bool
	Tone             func(s *Scheme) float64
	Background       func(s *Scheme) *Role
	SecondBackground func(s *Scheme) *Role
	ContrastCurve    *ContrastCurve
	ToneDeltaPair    *ToneDeltaPair
}

func highestSurface(s *Scheme) *Role {
	if s.IsDark {
		return roles["surface_bright"]
	}
	return roles["surface_dim"]
}

var roles = map[string]*Role{}

func tdp(subject, basis *Role, delta float64, polarity string, stayTogether bool) *ToneDeltaPair {
	return &ToneDeltaPair{Subject: subject, Basis: basis, Delta: delta, Polarity: polarity, StayTogether: stayTogether}
}

func initRoles() {
	var r = func(name, palette string, tone func(s *Scheme) float64) *Role {
		return &Role{Name: name, Palette: palette, Tone: tone}
	}

	roles["background"] = r("background", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return 6.0
		}
		return 98.0
	})
	roles["background"].IsBackground = true

	roles["surface_dim"] = r("surface_dim", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return 6.0
		}
		return ccGet(cc(87.0, 87.0, 80.0, 75.0), s.ContrastLevel)
	})
	roles["surface_dim"].IsBackground = true

	roles["surface_bright"] = r("surface_bright", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return ccGet(cc(24.0, 24.0, 29.0, 34.0), s.ContrastLevel)
		}
		return 98.0
	})
	roles["surface_bright"].IsBackground = true

	roles["surface"] = r("surface", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return 6.0
		}
		return 98.0
	})
	roles["surface"].IsBackground = true

	roles["on_background"] = r("on_background", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return 90.0
		}
		return 10.0
	})
	roles["on_background"].Background = func(s *Scheme) *Role { return roles["background"] }
	roles["on_background"].ContrastCurve = cc(3.0, 3.0, 4.5, 7.0)

	roles["surface_container_lowest"] = r("surface_container_lowest", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return ccGet(cc(4.0, 4.0, 2.0, 0.0), s.ContrastLevel)
		}
		return 100.0
	})
	roles["surface_container_lowest"].IsBackground = true

	roles["surface_container_low"] = r("surface_container_low", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return ccGet(cc(10.0, 10.0, 11.0, 12.0), s.ContrastLevel)
		}
		return ccGet(cc(96.0, 96.0, 96.0, 95.0), s.ContrastLevel)
	})
	roles["surface_container_low"].IsBackground = true

	roles["surface_container"] = r("surface_container", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return ccGet(cc(12.0, 12.0, 16.0, 20.0), s.ContrastLevel)
		}
		return ccGet(cc(94.0, 94.0, 92.0, 90.0), s.ContrastLevel)
	})
	roles["surface_container"].IsBackground = true

	roles["surface_container_high"] = r("surface_container_high", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return ccGet(cc(17.0, 17.0, 21.0, 25.0), s.ContrastLevel)
		}
		return ccGet(cc(92.0, 92.0, 88.0, 85.0), s.ContrastLevel)
	})
	roles["surface_container_high"].IsBackground = true

	roles["surface_container_highest"] = r("surface_container_highest", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return ccGet(cc(22.0, 22.0, 26.0, 30.0), s.ContrastLevel)
		}
		return ccGet(cc(90.0, 90.0, 84.0, 80.0), s.ContrastLevel)
	})
	roles["surface_container_highest"].IsBackground = true

	roles["on_surface"] = r("on_surface", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return 90.0
		}
		return 10.0
	})
	roles["on_surface"].Background = highestSurface
	roles["on_surface"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	roles["surface_variant"] = r("surface_variant", "NeutralVariant", func(s *Scheme) float64 {
		if s.IsDark {
			return 30.0
		}
		return 90.0
	})
	roles["surface_variant"].IsBackground = true

	roles["on_surface_variant"] = r("on_surface_variant", "NeutralVariant", func(s *Scheme) float64 {
		if s.IsDark {
			return 80.0
		}
		return 30.0
	})
	roles["on_surface_variant"].Background = highestSurface
	roles["on_surface_variant"].ContrastCurve = cc(3.0, 4.5, 7.0, 11.0)

	roles["inverse_surface"] = r("inverse_surface", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return 90.0
		}
		return 20.0
	})

	roles["inverse_on_surface"] = r("inverse_on_surface", "Neutral", func(s *Scheme) float64 {
		if s.IsDark {
			return 20.0
		}
		return 95.0
	})
	roles["inverse_on_surface"].Background = func(s *Scheme) *Role { return roles["inverse_surface"] }
	roles["inverse_on_surface"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	roles["outline"] = r("outline", "NeutralVariant", func(s *Scheme) float64 {
		if s.IsDark {
			return 60.0
		}
		return 50.0
	})
	roles["outline"].Background = highestSurface
	roles["outline"].ContrastCurve = cc(1.5, 3.0, 4.5, 7.0)

	roles["outline_variant"] = r("outline_variant", "NeutralVariant", func(s *Scheme) float64 {
		if s.IsDark {
			return 30.0
		}
		return 80.0
	})
	roles["outline_variant"].Background = highestSurface
	roles["outline_variant"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["shadow"] = r("shadow", "Neutral", func(s *Scheme) float64 { return 0.0 })
	roles["scrim"] = r("scrim", "Neutral", func(s *Scheme) float64 { return 0.0 })
	roles["surface_tint"] = r("surface_tint", "Primary", func(s *Scheme) float64 {
		if s.IsDark {
			return 80.0
		}
		return 40.0
	})
	roles["surface_tint"].IsBackground = true

	// Primary
	roles["primary"] = r("primary", "Primary", func(s *Scheme) float64 {
		if s.IsDark {
			return 80.0
		}
		return 40.0
	})
	roles["primary"].IsBackground = true
	roles["primary"].Background = highestSurface
	roles["primary"].ContrastCurve = cc(3.0, 4.5, 7.0, 7.0)

	roles["on_primary"] = r("on_primary", "Primary", func(s *Scheme) float64 {
		if s.IsDark {
			return 20.0
		}
		return 100.0
	})
	roles["on_primary"].Background = func(s *Scheme) *Role { return roles["primary"] }
	roles["on_primary"].ContrastCurve = cc(3.0, 7.0, 11.0, 21.0)

	roles["primary_container"] = r("primary_container", "Primary", func(s *Scheme) float64 {
		if s.IsDark {
			return 30.0
		}
		return 90.0
	})
	roles["primary_container"].IsBackground = true
	roles["primary_container"].Background = highestSurface
	roles["primary_container"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["primary"].ToneDeltaPair = tdp(roles["primary_container"], roles["primary"], 10.0, "Nearer", false)
	roles["primary_container"].ToneDeltaPair = roles["primary"].ToneDeltaPair

	roles["on_primary_container"] = r("on_primary_container", "Primary", func(s *Scheme) float64 {
		if s.IsDark {
			return 90.0
		}
		return 10.0
	})
	roles["on_primary_container"].Background = func(s *Scheme) *Role { return roles["primary_container"] }
	roles["on_primary_container"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	roles["inverse_primary"] = r("inverse_primary", "Primary", func(s *Scheme) float64 {
		if s.IsDark {
			return 40.0
		}
		return 80.0
	})
	roles["inverse_primary"].Background = func(s *Scheme) *Role { return roles["inverse_surface"] }
	roles["inverse_primary"].ContrastCurve = cc(3.0, 4.5, 7.0, 7.0)

	// Secondary
	roles["secondary"] = r("secondary", "Secondary", func(s *Scheme) float64 {
		if s.IsDark {
			return 80.0
		}
		return 40.0
	})
	roles["secondary"].IsBackground = true
	roles["secondary"].Background = highestSurface
	roles["secondary"].ContrastCurve = cc(3.0, 4.5, 7.0, 7.0)

	roles["on_secondary"] = r("on_secondary", "Secondary", func(s *Scheme) float64 {
		if s.IsDark {
			return 20.0
		}
		return 100.0
	})
	roles["on_secondary"].Background = func(s *Scheme) *Role { return roles["secondary"] }
	roles["on_secondary"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	roles["secondary_container"] = r("secondary_container", "Secondary", func(s *Scheme) float64 {
		if s.IsDark {
			return 30.0
		}
		return 90.0
	})
	roles["secondary_container"].IsBackground = true
	roles["secondary_container"].Background = highestSurface
	roles["secondary_container"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["secondary"].ToneDeltaPair = tdp(roles["secondary_container"], roles["secondary"], 10.0, "Nearer", false)
	roles["secondary_container"].ToneDeltaPair = roles["secondary"].ToneDeltaPair

	roles["on_secondary_container"] = r("on_secondary_container", "Secondary", func(s *Scheme) float64 {
		if s.IsDark {
			return 90.0
		}
		return 10.0
	})
	roles["on_secondary_container"].Background = func(s *Scheme) *Role { return roles["secondary_container"] }
	roles["on_secondary_container"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	// Tertiary
	roles["tertiary"] = r("tertiary", "Tertiary", func(s *Scheme) float64 {
		if s.IsDark {
			return 80.0
		}
		return 40.0
	})
	roles["tertiary"].IsBackground = true
	roles["tertiary"].Background = highestSurface
	roles["tertiary"].ContrastCurve = cc(3.0, 4.5, 7.0, 7.0)

	roles["on_tertiary"] = r("on_tertiary", "Tertiary", func(s *Scheme) float64 {
		if s.IsDark {
			return 20.0
		}
		return 100.0
	})
	roles["on_tertiary"].Background = func(s *Scheme) *Role { return roles["tertiary"] }
	roles["on_tertiary"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	roles["tertiary_container"] = r("tertiary_container", "Tertiary", func(s *Scheme) float64 {
		if s.IsDark {
			return 30.0
		}
		return 90.0
	})
	roles["tertiary_container"].IsBackground = true
	roles["tertiary_container"].Background = highestSurface
	roles["tertiary_container"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["tertiary"].ToneDeltaPair = tdp(roles["tertiary_container"], roles["tertiary"], 10.0, "Nearer", false)
	roles["tertiary_container"].ToneDeltaPair = roles["tertiary"].ToneDeltaPair

	roles["on_tertiary_container"] = r("on_tertiary_container", "Tertiary", func(s *Scheme) float64 {
		if s.IsDark {
			return 90.0
		}
		return 10.0
	})
	roles["on_tertiary_container"].Background = func(s *Scheme) *Role { return roles["tertiary_container"] }
	roles["on_tertiary_container"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	// Error
	roles["error"] = r("error", "Error", func(s *Scheme) float64 {
		if s.IsDark {
			return 80.0
		}
		return 40.0
	})
	roles["error"].IsBackground = true
	roles["error"].Background = highestSurface
	roles["error"].ContrastCurve = cc(3.0, 4.5, 7.0, 7.0)

	roles["on_error"] = r("on_error", "Error", func(s *Scheme) float64 {
		if s.IsDark {
			return 20.0
		}
		return 100.0
	})
	roles["on_error"].Background = func(s *Scheme) *Role { return roles["error"] }
	roles["on_error"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	roles["error_container"] = r("error_container", "Error", func(s *Scheme) float64 {
		if s.IsDark {
			return 30.0
		}
		return 90.0
	})
	roles["error_container"].IsBackground = true
	roles["error_container"].Background = highestSurface
	roles["error_container"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["error"].ToneDeltaPair = tdp(roles["error_container"], roles["error"], 10.0, "Nearer", false)
	roles["error_container"].ToneDeltaPair = roles["error"].ToneDeltaPair

	roles["on_error_container"] = r("on_error_container", "Error", func(s *Scheme) float64 {
		if s.IsDark {
			return 90.0
		}
		return 10.0
	})
	roles["on_error_container"].Background = func(s *Scheme) *Role { return roles["error_container"] }
	roles["on_error_container"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	// Fixed roles
	roles["primary_fixed"] = r("primary_fixed", "Primary", func(s *Scheme) float64 { return 90.0 })
	roles["primary_fixed"].IsBackground = true
	roles["primary_fixed"].Background = highestSurface
	roles["primary_fixed"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["primary_fixed_dim"] = r("primary_fixed_dim", "Primary", func(s *Scheme) float64 { return 80.0 })
	roles["primary_fixed_dim"].IsBackground = true
	roles["primary_fixed_dim"].Background = highestSurface
	roles["primary_fixed_dim"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["primary_fixed"].ToneDeltaPair = tdp(roles["primary_fixed"], roles["primary_fixed_dim"], 10.0, "Lighter", true)
	roles["primary_fixed_dim"].ToneDeltaPair = roles["primary_fixed"].ToneDeltaPair

	roles["on_primary_fixed"] = r("on_primary_fixed", "Primary", func(s *Scheme) float64 { return 10.0 })
	roles["on_primary_fixed"].Background = func(s *Scheme) *Role { return roles["primary_fixed_dim"] }
	roles["on_primary_fixed"].SecondBackground = func(s *Scheme) *Role { return roles["primary_fixed"] }
	roles["on_primary_fixed"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	roles["on_primary_fixed_variant"] = r("on_primary_fixed_variant", "Primary", func(s *Scheme) float64 { return 30.0 })
	roles["on_primary_fixed_variant"].Background = func(s *Scheme) *Role { return roles["primary_fixed_dim"] }
	roles["on_primary_fixed_variant"].SecondBackground = func(s *Scheme) *Role { return roles["primary_fixed"] }
	roles["on_primary_fixed_variant"].ContrastCurve = cc(3.0, 4.5, 7.0, 11.0)

	roles["secondary_fixed"] = r("secondary_fixed", "Secondary", func(s *Scheme) float64 { return 90.0 })
	roles["secondary_fixed"].IsBackground = true
	roles["secondary_fixed"].Background = highestSurface
	roles["secondary_fixed"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["secondary_fixed_dim"] = r("secondary_fixed_dim", "Secondary", func(s *Scheme) float64 { return 80.0 })
	roles["secondary_fixed_dim"].IsBackground = true
	roles["secondary_fixed_dim"].Background = highestSurface
	roles["secondary_fixed_dim"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["secondary_fixed"].ToneDeltaPair = tdp(roles["secondary_fixed"], roles["secondary_fixed_dim"], 10.0, "Lighter", true)
	roles["secondary_fixed_dim"].ToneDeltaPair = roles["secondary_fixed"].ToneDeltaPair

	roles["on_secondary_fixed"] = r("on_secondary_fixed", "Secondary", func(s *Scheme) float64 { return 10.0 })
	roles["on_secondary_fixed"].Background = func(s *Scheme) *Role { return roles["secondary_fixed_dim"] }
	roles["on_secondary_fixed"].SecondBackground = func(s *Scheme) *Role { return roles["secondary_fixed"] }
	roles["on_secondary_fixed"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	roles["on_secondary_fixed_variant"] = r("on_secondary_fixed_variant", "Secondary", func(s *Scheme) float64 { return 30.0 })
	roles["on_secondary_fixed_variant"].Background = func(s *Scheme) *Role { return roles["secondary_fixed_dim"] }
	roles["on_secondary_fixed_variant"].SecondBackground = func(s *Scheme) *Role { return roles["secondary_fixed"] }
	roles["on_secondary_fixed_variant"].ContrastCurve = cc(3.0, 4.5, 7.0, 11.0)

	roles["tertiary_fixed"] = r("tertiary_fixed", "Tertiary", func(s *Scheme) float64 { return 90.0 })
	roles["tertiary_fixed"].IsBackground = true
	roles["tertiary_fixed"].Background = highestSurface
	roles["tertiary_fixed"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["tertiary_fixed_dim"] = r("tertiary_fixed_dim", "Tertiary", func(s *Scheme) float64 { return 80.0 })
	roles["tertiary_fixed_dim"].IsBackground = true
	roles["tertiary_fixed_dim"].Background = highestSurface
	roles["tertiary_fixed_dim"].ContrastCurve = cc(1.0, 1.0, 3.0, 4.5)

	roles["tertiary_fixed"].ToneDeltaPair = tdp(roles["tertiary_fixed"], roles["tertiary_fixed_dim"], 10.0, "Lighter", true)
	roles["tertiary_fixed_dim"].ToneDeltaPair = roles["tertiary_fixed"].ToneDeltaPair

	roles["on_tertiary_fixed"] = r("on_tertiary_fixed", "Tertiary", func(s *Scheme) float64 { return 10.0 })
	roles["on_tertiary_fixed"].Background = func(s *Scheme) *Role { return roles["tertiary_fixed_dim"] }
	roles["on_tertiary_fixed"].SecondBackground = func(s *Scheme) *Role { return roles["tertiary_fixed"] }
	roles["on_tertiary_fixed"].ContrastCurve = cc(4.5, 7.0, 11.0, 21.0)

	roles["on_tertiary_fixed_variant"] = r("on_tertiary_fixed_variant", "Tertiary", func(s *Scheme) float64 { return 30.0 })
	roles["on_tertiary_fixed_variant"].Background = func(s *Scheme) *Role { return roles["tertiary_fixed_dim"] }
	roles["on_tertiary_fixed_variant"].SecondBackground = func(s *Scheme) *Role { return roles["tertiary_fixed"] }
	roles["on_tertiary_fixed_variant"].ContrastCurve = cc(3.0, 4.5, 7.0, 11.0)
}

func init() {
	initRoles()
}

func getTone(role *Role, s *Scheme) float64 {
	cl := s.ContrastLevel
	decreasing := cl < 0.0

	if role.ToneDeltaPair != nil {
		pair := role.ToneDeltaPair
		roleA := pair.Subject
		roleB := pair.Basis
		delta := pair.Delta
		polarity := pair.Polarity
		stayTogether := pair.StayTogether
		bgTone := getTone(role.Background(s), s)
		aIsNearer := polarity == "Nearer" || (polarity == "Lighter" && !s.IsDark) || (polarity == "Darker" && s.IsDark)

		var nearer, farther *Role
		if aIsNearer {
			nearer = roleA
			farther = roleB
		} else {
			nearer = roleB
			farther = roleA
		}

		amNearer := role.Name == nearer.Name
		dir := -1.0
		if s.IsDark {
			dir = 1.0
		}
		nContrast := ccGet(nearer.ContrastCurve, cl)
		fContrast := ccGet(farther.ContrastCurve, cl)
		nInitial := nearer.Tone(s)

		var nTone0 float64
		if decreasing {
			nTone0 = foregroundTone(bgTone, nContrast)
		} else if ratioOfTones(bgTone, nInitial) >= nContrast {
			nTone0 = nInitial
		} else {
			nTone0 = foregroundTone(bgTone, nContrast)
		}

		fInitial := farther.Tone(s)
		var fTone0 float64
		if decreasing {
			fTone0 = foregroundTone(bgTone, fContrast)
		} else if ratioOfTones(bgTone, fInitial) >= fContrast {
			fTone0 = fInitial
		} else {
			fTone0 = foregroundTone(bgTone, fContrast)
		}

		var expN, expF float64
		if (fTone0-nTone0)*dir >= delta {
			expN = nTone0
			expF = fTone0
		} else {
			f1 := clamp(0.0, 100.0, delta*dir+nTone0)
			if (f1-nTone0)*dir >= delta {
				expN = nTone0
				expF = f1
			} else {
				expN = clamp(0.0, 100.0, 0.0-delta*dir+f1)
				expF = f1
			}
		}

		var adjN, adjF float64
		if between(50.0, 60.0, expN) {
			if dir > 0.0 {
				adjN = 60.0
				adjF = max(expF, delta*dir+60.0)
			} else {
				adjN = 49.0
				adjF = min(expF, delta*dir+49.0)
			}
		} else if between(50.0, 60.0, expF) {
			if stayTogether {
				if dir > 0.0 {
					adjN = 60.0
					adjF = max(expF, delta*dir+60.0)
				} else {
					adjN = 49.0
					adjF = min(expF, delta*dir+49.0)
				}
			} else {
				adjN = expN
				if dir > 0.0 {
					adjF = 60.0
				} else {
					adjF = 49.0
				}
			}
		} else {
			adjN = expN
			adjF = expF
		}

		if amNearer {
			return adjN
		}
		return adjF
	}

	answer0 := role.Tone(s)
	if role.Background == nil {
		return answer0
	}

	bgTone := getTone(role.Background(s), s)
	desired := ccGet(role.ContrastCurve, cl)

	var answer1 float64
	if ratioOfTones(bgTone, answer0) >= desired && !decreasing {
		answer1 = answer0
	} else {
		answer1 = foregroundTone(bgTone, desired)
	}

	var answer2 float64
	if role.IsBackground && between(50.0, 60.0, answer1) {
		if ratioOfTones(49.0, bgTone) >= desired {
			answer2 = 49.0
		} else {
			answer2 = 60.0
		}
	} else {
		answer2 = answer1
	}

	if role.SecondBackground == nil {
		return answer2
	}

	bgt1 := getTone(role.Background(s), s)
	bgt2 := getTone(role.SecondBackground(s), s)
	upper := max(bgt1, bgt2)
	lower := min(bgt1, bgt2)
	lightOption := lighter(upper, desired)
	darkOption := darker(lower, desired)
	prefersLight := tonePrefersLight(bgt1) || tonePrefersLight(bgt2)

	availables := []float64{}
	if math.Abs(lightOption-(-1.0)) > 1.0e-12 {
		availables = append(availables, lightOption)
	}
	if math.Abs(darkOption-(-1.0)) > 1.0e-12 {
		availables = append(availables, darkOption)
	}

	if ratioOfTones(upper, answer2) >= desired && ratioOfTones(lower, answer2) >= desired {
		return answer2
	}
	if prefersLight {
		if lightOption < 0.0 {
			return 100.0
		}
		return lightOption
	}
	if len(availables) == 1 {
		return availables[0]
	}
	if darkOption < 0.0 {
		return 0.0
	}
	return darkOption
}

func colorOf(roleName string, s *Scheme) string {
	role := roles[roleName]
	tone := getTone(role, s)
	switch role.Palette {
	case "Primary":
		return s.Palettes.Primary.Tone(tone)
	case "Secondary":
		return s.Palettes.Secondary.Tone(tone)
	case "Tertiary":
		return s.Palettes.Tertiary.Tone(tone)
	case "Neutral":
		return s.Palettes.Neutral.Tone(tone)
	case "NeutralVariant":
		return s.Palettes.NeutralVariant.Tone(tone)
	case "Error":
		return s.Palettes.Error.Tone(tone)
	default:
		return s.Palettes.Neutral.Tone(tone)
	}
}

// colorsForBoth computes dark and light colors sharing the same PaletteSet,
// avoiding redundant HCT solves since the tonal palettes are memoized.
func colorsForBoth(sourceHex string) (dark, light map[string]string) {
	src := hctFromHex(sourceHex)
	palettes := rainbowPalettes(src.Hue, src.Chroma)

	mkSchemeWith := func(isDark bool) *Scheme {
		return &Scheme{
			IsDark:         isDark,
			ContrastLevel:  0.0,
			SourceColorHct: src,
			Palettes:       palettes,
		}
	}

	darkScheme := mkSchemeWith(true)
	lightScheme := mkSchemeWith(false)

	dark = make(map[string]string, len(roles))
	light = make(map[string]string, len(roles))
	for name := range roles {
		dark[name] = colorOf(name, darkScheme)
		light[name] = colorOf(name, lightScheme)
	}
	return dark, light
}
