package icons

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	imagedraw "image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"workspaced/internal/configcue"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/svgraster"
	"workspaced/pkg/logging"
	"workspaced/pkg/taskgroup"

	xdraw "golang.org/x/image/draw"
)

var (
	ErrNoValidSizes        = errors.New("no valid sizes provided")
	ErrBase16NotConfigured = errors.New("module base16 is not configured")
	ErrInvalidBase16Config = errors.New("invalid modules.base16 config")
	ErrBase16MissingColors = errors.New("modules.base16 must include at least base00/base05/base0D")
	ErrInvalidSize         = errors.New("invalid icon size")
	ErrInvalidReplaceRule  = errors.New("invalid color replace rule")
	ErrNoSVGFiles          = errors.New("no .svg or .svg.tmpl files found")
)

var (
	sizeDirPrefixRe = regexp.MustCompile(`^\d+x\d+$`)
	// Constant patterns used on every themed SVG; compile once, not per icon.
	hexColorRe   = regexp.MustCompile(`(?i)#([0-9a-f]{3}|[0-9a-f]{6})\b`)
	svgViewBoxRe = regexp.MustCompile(`(?i)viewBox\s*=\s*"[^"]*([0-9.]+)\s+([0-9.]+)"`)
	svgWidthRe   = regexp.MustCompile(`(?i)\bwidth\s*=\s*"([0-9.]+)(px)?"`)
	svgHeightRe  = regexp.MustCompile(`(?i)\bheight\s*=\s*"([0-9.]+)(px)?"`)
)
var fastPNGEncoder = png.Encoder{CompressionLevel: png.BestSpeed}

func runThemeGenerateEngine(ctx context.Context, opts ThemeGenerateOptions, inputDir, outputDir string) error {
	sizes, err := parseSizes(opts.Sizes)
	if err != nil {
		return err
	}

	if opts.Clean {
		if err := os.RemoveAll(outputDir); err != nil {
			return fmt.Errorf("failed to clean output dir: %w", err)
		}
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	colors, err := loadBase16Colors(ctx)
	if err != nil {
		return err
	}
	colorReplacements, err := parseColorReplacements(opts.Replace)
	if err != nil {
		return err
	}

	paths, err := collectIconInputs(inputDir)
	if err != nil {
		return err
	}
	originalCount := len(paths)
	paths, dedupedCount, err := dedupeIconInputs(inputDir, paths)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("%w: %s", ErrNoSVGFiles, inputDir)
	}

	maxSize := 0
	if !opts.NoRaster {
		maxSize = sizes[len(sizes)-1]
	}

	// Ensure the rasterizer (resvg) is ready before we even schedule the
	// icons processing control task. This ensures the tool resolution step
	// (which may install/download) completes and its progress task finishes
	// *before* the "icon-theme:..." processing starts.
	if !opts.NoRaster {
		if err := svgraster.Ensure(ctx); err != nil {
			return fmt.Errorf("failed to ensure resvg (needed for icon rasterization): %w", err)
		}
	}

	// Map each icon to the theme dirs it touched; reduce merges into index theme.
	perIconDirs, err := taskgroup.Map[string, []string]{
		Name:     "icon-theme:" + opts.ThemeName,
		Items:    paths,
		PoolKind: taskgroup.CPU,
		TaskName: func(_ int, iconPath string) string {
			// Use the logical output-relative name (including context dir)
			// so that icons with the same basename in different categories
			// (e.g. apps/brightnesssettings.svg vs devices/brightnesssettings.svg)
			// get unique task names. This matches the deduplication key logic.
			// On Rel failure (e.g. paths on different volumes), fall back to the
			// basename so progress labels stay unique-ish instead of collapsing.
			rel, err := filepath.Rel(inputDir, iconPath)
			if err != nil {
				return "icon:" + filepath.Base(iconPath)
			}
			relOut := strings.TrimSuffix(rel, ".tmpl")
			relOut = strings.TrimSuffix(relOut, ".svg") + ".svg"
			relOut = stripLeadingSizeDir(relOut)
			return "icon:" + filepath.ToSlash(relOut)
		},
		Fn: func(ctx context.Context, itemS *taskgroup.Status, iconPath string) ([]string, error) {
			var localDirs []string

			rel, err := filepath.Rel(inputDir, iconPath)
			if err != nil {
				return nil, err
			}
			relOut := strings.TrimSuffix(rel, ".tmpl")
			relOut = strings.TrimSuffix(relOut, ".svg") + ".svg"
			relOut = stripLeadingSizeDir(relOut)

			ctxDir := filepath.Dir(relOut)
			if ctxDir == "." {
				ctxDir = opts.DefaultContext
				relOut = filepath.Join(ctxDir, filepath.Base(relOut))
			}

			iconName := strings.TrimSuffix(filepath.Base(relOut), ".svg")
			itemS.Update(iconName)

			svgContent, err := renderSVG(iconPath, colors, colorReplacements, opts.MapScheme, opts.ThemeName, iconName)
			if err != nil {
				return nil, err
			}

			targetSVG := filepath.Join(outputDir, "scalable", relOut)
			if err := os.MkdirAll(filepath.Dir(targetSVG), 0755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(targetSVG, []byte(svgContent), 0644); err != nil {
				return nil, err
			}
			localDirs = append(localDirs, filepath.ToSlash(filepath.Join("scalable", ctxDir)))

			if !opts.NoRaster {
				ratio := extractSVGAspectRatio(svgContent)
				maxW, maxH := fitSizePreservingAspect(ratio, maxSize)
				renderedMax, err := svgraster.RasterizeSVG(ctx, svgContent, maxW, maxH)
				if err != nil {
					return nil, err
				}

				for _, sz := range sizes {
					w, h := fitSizePreservingAspect(ratio, sz)
					var rendered image.Image
					if sz == maxSize {
						rendered = renderedMax
					} else {
						rendered = resizeBilinear(renderedMax, w, h)
					}

					final := centerInSquare(rendered, sz)
					sizeDir := filepath.Join(fmt.Sprintf("%dx%d", sz, sz), ctxDir)
					targetPNG := filepath.Join(outputDir, sizeDir, iconName+".png")
					if err := os.MkdirAll(filepath.Dir(targetPNG), 0755); err != nil {
						return nil, err
					}
					f, err := os.Create(targetPNG)
					if err != nil {
						return nil, err
					}
					if err := fastPNGEncoder.Encode(f, final); err != nil {
						logging.Close(ctx, f)
						return nil, err
					}
					if err := f.Close(); err != nil {
						return nil, err
					}
					localDirs = append(localDirs, filepath.ToSlash(sizeDir))
				}
			}

			return localDirs, nil
		},
	}.Run(ctx)
	if err != nil {
		return err
	}

	dirsUsed := map[string]bool{}
	for _, dirs := range perIconDirs {
		for _, d := range dirs {
			dirsUsed[d] = true
		}
	}

	if err := writeIndexTheme(outputDir, opts.ThemeName, dirsUsed); err != nil {
		return err
	}

	if opts.UpdateCache && execdriver.IsBinaryAvailable(ctx, "gtk-update-icon-cache") {
		logger := logging.GetLogger(ctx)
		cacheCmd, err := execdriver.Run(ctx, "gtk-update-icon-cache", "-f", "-q", outputDir)
		if err != nil {
			logger.Warn("failed to prepare gtk-update-icon-cache", "dir", outputDir, "error", err)
		} else if err := cacheCmd.Run(); err != nil {
			logger.Warn("gtk-update-icon-cache failed", "dir", outputDir, "error", err)
		}
	}

	_, _ = fmt.Fprintf(opts.Stdout, "generated icon theme %q in %s (%d SVG files, deduped %d/%d)\n", opts.ThemeName, outputDir, len(perIconDirs), dedupedCount, originalCount)
	return nil
}

func collectIconInputs(inputDir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(inputDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".svg") || strings.HasSuffix(d.Name(), ".svg.tmpl") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func dedupeIconInputs(inputDir string, paths []string) ([]string, int, error) {
	byKey := map[string]string{}
	byKeyScore := map[string]int{}
	for _, path := range paths {
		rel, err := filepath.Rel(inputDir, path)
		if err != nil {
			return nil, 0, err
		}
		relOut := strings.TrimSuffix(rel, ".tmpl")
		relOut = strings.TrimSuffix(relOut, ".svg") + ".svg"
		key := filepath.ToSlash(stripLeadingSizeDir(relOut))
		score := 1
		if strings.HasSuffix(path, ".svg.tmpl") {
			score = 2
		}
		prevScore, ok := byKeyScore[key]
		if !ok || score > prevScore {
			byKey[key] = path
			byKeyScore[key] = score
		}
	}

	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, byKey[k])
	}
	return out, len(paths) - len(out), nil
}

func parseSizes(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("%w: %q", ErrInvalidSize, p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, ErrNoValidSizes
	}
	sort.Ints(out)
	return out, nil
}

func loadBase16Colors(ctx context.Context) (map[string]string, error) {
	cfg, err := configcue.LoadHome(ctx)
	if err != nil {
		return nil, err
	}
	entry, err := cfg.ModuleEntry("base16")
	if err != nil {
		return nil, ErrBase16NotConfigured
	}
	m := entry.Config
	if m == nil {
		return nil, ErrInvalidBase16Config
	}

	out := map[string]string{}
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			continue
		}
		s = strings.TrimPrefix(s, "#")
		out[k] = s
		out[strings.ToUpper(k)] = s
	}
	if out["base00"] == "" || out["base05"] == "" || out["base0D"] == "" {
		return nil, ErrBase16MissingColors
	}
	return out, nil
}

func parseColorReplacements(rules []string) (map[string]string, error) {
	out := map[string]string{}
	for _, r := range rules {
		pair := strings.SplitN(r, "=", 2)
		if len(pair) != 2 {
			return nil, fmt.Errorf("%w: %q (expected old=new)", ErrInvalidReplaceRule, r)
		}
		oldC := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(pair[0]), "#"))
		newC := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(pair[1]), "#"))
		if len(oldC) != 6 || len(newC) != 6 {
			return nil, fmt.Errorf("%w: %q (expected 6-digit hex)", ErrInvalidReplaceRule, r)
		}
		out[oldC] = newC
	}
	return out, nil
}

func renderSVG(path string, colors map[string]string, replacements map[string]string, mapScheme bool, themeName string, iconName string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(b)
	if strings.HasSuffix(path, ".tmpl") {
		tpl, err := template.New(filepath.Base(path)).Parse(content)
		if err != nil {
			return "", fmt.Errorf("template parse failed for %s: %w", path, err)
		}
		data := map[string]any{
			"theme_name": themeName,
			"icon_name":  iconName,
		}
		for k, v := range colors {
			data[k] = v
		}
		var out bytes.Buffer
		if err := tpl.Execute(&out, data); err != nil {
			return "", fmt.Errorf("template render failed for %s: %w", path, err)
		}
		content = out.String()
	}

	for k, v := range colors {
		content = strings.ReplaceAll(content, "%"+k+"%", v)
		content = strings.ReplaceAll(content, "%"+strings.ToUpper(k)+"%", v)
	}
	if mapScheme {
		content = mapHexColorsToScheme(content, colors)
	}
	for oldC, newC := range replacements {
		content = strings.ReplaceAll(content, "#"+oldC, "#"+newC)
		content = strings.ReplaceAll(content, "#"+strings.ToUpper(oldC), "#"+strings.ToUpper(newC))
	}
	return content, nil
}

func mapHexColorsToScheme(content string, colors map[string]string) string {
	palette := extractPalette(colors)
	if len(palette) == 0 {
		return content
	}

	cache := map[string]string{}

	return hexColorRe.ReplaceAllStringFunc(content, func(match string) string {
		src := strings.ToLower(strings.TrimPrefix(match, "#"))
		if len(src) == 3 {
			src = fmt.Sprintf("%c%c%c%c%c%c", src[0], src[0], src[1], src[1], src[2], src[2])
		}
		if len(src) != 6 {
			return match
		}
		if repl, ok := cache[src]; ok {
			return "#" + repl
		}
		nearest := nearestColor(src, palette)
		cache[src] = nearest
		return "#" + nearest
	})
}

func stripLeadingSizeDir(rel string) string {
	clean := filepath.Clean(rel)
	parts := strings.Split(clean, string(filepath.Separator))
	if len(parts) > 1 && sizeDirPrefixRe.MatchString(parts[0]) {
		return filepath.Join(parts[1:]...)
	}
	return clean
}

// base16PaletteKeys is the ordered base16 (+ extended) keys used when mapping
// free hex colors onto the active scheme. Package-level so mapHexColorsToScheme
// does not rebuild the slice on every SVG.
var base16PaletteKeys = []string{
	"base00", "base01", "base02", "base03",
	"base04", "base05", "base06", "base07",
	"base08", "base09", "base0A", "base0B",
	"base0C", "base0D", "base0E", "base0F",
	"base10", "base11", "base12", "base13", "base14", "base15", "base16", "base17",
}

func extractPalette(colors map[string]string) []string {
	out := make([]string, 0, len(base16PaletteKeys))
	seen := make(map[string]bool, len(base16PaletteKeys))
	for _, k := range base16PaletteKeys {
		c := strings.ToLower(strings.TrimPrefix(colors[k], "#"))
		if len(c) != 6 || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func nearestColor(src string, palette []string) string {
	sr, sg, sb := parseHexRGB(src)
	best := palette[0]
	bestD := math.MaxFloat64
	for _, c := range palette {
		r, g, b := parseHexRGB(c)
		dr := float64(sr - r)
		dg := float64(sg - g)
		db := float64(sb - b)
		d := dr*dr + dg*dg + db*db
		if d < bestD {
			bestD = d
			best = c
		}
	}
	return best
}

func parseHexRGB(hex string) (int, int, int) {
	if len(hex) != 6 {
		return 0, 0, 0
	}
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return int(r), int(g), int(b)
}

func extractSVGAspectRatio(svg string) float64 {
	if m := svgViewBoxRe.FindStringSubmatch(svg); len(m) == 3 {
		w, _ := strconv.ParseFloat(m[1], 64)
		h, _ := strconv.ParseFloat(m[2], 64)
		if w > 0 && h > 0 {
			return w / h
		}
	}
	wm := svgWidthRe.FindStringSubmatch(svg)
	hm := svgHeightRe.FindStringSubmatch(svg)
	if len(wm) >= 2 && len(hm) >= 2 {
		w, _ := strconv.ParseFloat(wm[1], 64)
		h, _ := strconv.ParseFloat(hm[1], 64)
		if w > 0 && h > 0 {
			return w / h
		}
	}
	return 1.0
}

func fitSizePreservingAspect(ratio float64, size int) (int, int) {
	if ratio <= 0 {
		return size, size
	}
	if ratio >= 1.0 {
		w := size
		h := int(math.Round(float64(size) / ratio))
		if h < 1 {
			h = 1
		}
		return w, h
	}
	h := size
	w := int(math.Round(float64(size) * ratio))
	if w < 1 {
		w = 1
	}
	return w, h
}

func centerInSquare(img image.Image, size int) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	offX := (size - w) / 2
	offY := (size - h) / 2
	rect := image.Rect(offX, offY, offX+w, offY+h)
	imagedraw.Draw(dst, rect, img, b.Min, imagedraw.Over)
	return dst
}

func resizeBilinear(src image.Image, outW, outH int) image.Image {
	if outW <= 0 || outH <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, outW, outH))
	}
	if sw == outW && sh == outH {
		return src
	}

	dst := image.NewNRGBA(image.Rect(0, 0, outW, outH))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, sb, xdraw.Over, nil)
	return dst
}

func writeIndexTheme(outputDir string, themeName string, dirsUsed map[string]bool) error {
	dirs := make([]string, 0, len(dirsUsed))
	for d := range dirsUsed {
		dirs = append(dirs, filepath.ToSlash(d))
	}
	sort.Strings(dirs)

	var b strings.Builder
	fmt.Fprintf(&b, "[Icon Theme]\n")
	fmt.Fprintf(&b, "Name=%s\n", themeName)
	fmt.Fprintf(&b, "Comment=Generated by workspaced utils icons generate\n")
	fmt.Fprintf(&b, "Inherits=Adwaita,hicolor\n")
	fmt.Fprintf(&b, "Directories=%s\n\n", strings.Join(dirs, ","))

	for _, d := range dirs {
		fmt.Fprintf(&b, "[%s]\n", d)
		if strings.HasPrefix(d, "scalable/") {
			fmt.Fprintf(&b, "Size=128\n")
			fmt.Fprintf(&b, "MinSize=16\n")
			fmt.Fprintf(&b, "MaxSize=512\n")
			fmt.Fprintf(&b, "Type=Scalable\n")
		} else {
			sizePart := strings.SplitN(d, "/", 2)[0]
			sizePart = strings.SplitN(sizePart, "x", 2)[0]
			if n, err := strconv.Atoi(sizePart); err == nil {
				fmt.Fprintf(&b, "Size=%d\n", n)
			}
			fmt.Fprintf(&b, "Type=Fixed\n")
		}
		fmt.Fprintf(&b, "Context=Applications\n\n")
	}

	return os.WriteFile(filepath.Join(outputDir, "index.theme"), []byte(b.String()), 0644)
}
