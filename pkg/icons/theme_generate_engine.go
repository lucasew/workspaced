package icons

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"workspaced/pkg/config"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/svgraster"

	"github.com/schollz/progressbar/v3"
)

var sizeDirPrefixRe = regexp.MustCompile(`^\d+x\d+$`)
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

	colors, err := loadBase16Colors()
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
		return fmt.Errorf("no .svg or .svg.tmpl files found in %s", inputDir)
	}

	maxSize := 0
	if !opts.NoRaster {
		maxSize = sizes[len(sizes)-1]
	}
	jobs, err := resolveJobs(opts.Jobs)
	if err != nil {
		return err
	}

	bar := progressbar.NewOptions(
		len(paths),
		progressbar.OptionSetWriter(opts.Stderr),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowCount(),
		progressbar.OptionSetDescription(fmt.Sprintf("icons(%d jobs)", jobs)),
		progressbar.OptionThrottle(200*time.Millisecond),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprintln(opts.Stderr)
		}),
	)

	dirsUsed := map[string]bool{}
	var dirsUsedMu sync.Mutex
	var written int64

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tasks := make(chan string)
	errCh := make(chan error, 1)
	reportErr := func(err error) {
		if err == nil {
			return
		}
		select {
		case errCh <- err:
		default:
		}
		cancel()
	}

	processOne := func(path string) error {
		localDirs := map[string]bool{}

		rel, err := filepath.Rel(inputDir, path)
		if err != nil {
			return err
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
		svgContent, err := renderSVG(path, colors, colorReplacements, opts.MapScheme, opts.ThemeName, iconName)
		if err != nil {
			return err
		}

		targetSVG := filepath.Join(outputDir, "scalable", relOut)
		if err := os.MkdirAll(filepath.Dir(targetSVG), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(targetSVG, []byte(svgContent), 0644); err != nil {
			return err
		}
		localDirs[filepath.ToSlash(filepath.Join("scalable", ctxDir))] = true

		if !opts.NoRaster {
			ratio := extractSVGAspectRatio(svgContent)
			maxW, maxH := fitSizePreservingAspect(ratio, maxSize)
			renderedMax, err := svgraster.RasterizeSVG(workerCtx, svgContent, maxW, maxH)
			if err != nil {
				return err
			}

			for _, s := range sizes {
				w, h := fitSizePreservingAspect(ratio, s)
				var rendered image.Image
				if s == maxSize {
					rendered = renderedMax
				} else {
					rendered = resizeBilinear(renderedMax, w, h)
				}

				final := centerInSquare(rendered, s)
				sizeDir := filepath.Join(fmt.Sprintf("%dx%d", s, s), ctxDir)
				targetPNG := filepath.Join(outputDir, sizeDir, iconName+".png")
				if err := os.MkdirAll(filepath.Dir(targetPNG), 0755); err != nil {
					return err
				}
				f, err := os.Create(targetPNG)
				if err != nil {
					return err
				}
				if err := fastPNGEncoder.Encode(f, final); err != nil {
					f.Close()
					return err
				}
				if err := f.Close(); err != nil {
					return err
				}
				localDirs[filepath.ToSlash(sizeDir)] = true
			}
		}

		dirsUsedMu.Lock()
		for d := range localDirs {
			dirsUsed[d] = true
		}
		dirsUsedMu.Unlock()

		atomic.AddInt64(&written, 1)
		return nil
	}

	progressStop := make(chan struct{})
	var progressWG sync.WaitGroup
	progressWG.Add(1)
	go func() {
		defer progressWG.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		last := 0
		flush := func() {
			cur := int(atomic.LoadInt64(&written))
			if cur > last {
				_ = bar.Add(cur - last)
				last = cur
			}
		}
		for {
			select {
			case <-ticker.C:
				flush()
			case <-progressStop:
				flush()
				return
			case <-workerCtx.Done():
				flush()
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				case path, ok := <-tasks:
					if !ok {
						return
					}
					if err := processOne(path); err != nil {
						reportErr(err)
						return
					}
				}
			}
		}()
	}

produceLoop:
	for _, path := range paths {
		select {
		case <-workerCtx.Done():
			break produceLoop
		case tasks <- path:
		}
	}
	close(tasks)
	wg.Wait()
	close(progressStop)
	progressWG.Wait()

	select {
	case err := <-errCh:
		return err
	default:
	}
	if err := workerCtx.Err(); err != nil && err != context.Canceled {
		return err
	}

	if err := writeIndexTheme(outputDir, opts.ThemeName, dirsUsed); err != nil {
		return err
	}

	if opts.UpdateCache && execdriver.IsBinaryAvailable(ctx, "gtk-update-icon-cache") {
		cacheCmd, err := execdriver.Run(ctx, "gtk-update-icon-cache", "-f", "-q", outputDir)
		if err == nil {
			_ = cacheCmd.Run()
		}
	}

	_, _ = fmt.Fprintf(opts.Stdout, "generated icon theme %q in %s (%d SVG files, deduped %d/%d)\n", opts.ThemeName, outputDir, int(atomic.LoadInt64(&written)), dedupedCount, originalCount)
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
			return nil, fmt.Errorf("invalid size %q", p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid sizes provided")
	}
	sort.Ints(out)
	return out, nil
}

func resolveJobs(raw string) (int, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" || s == "auto" {
		n := runtime.NumCPU() * 2
		if n > 8 {
			n = 8
		}
		if n < 1 {
			n = 1
		}
		return n, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid --jobs %q (use integer >= 1 or 'auto')", raw)
	}
	return n, nil
}

func loadBase16Colors() (map[string]string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	raw, ok := cfg.Modules["base16"]
	if !ok {
		return nil, fmt.Errorf("module base16 is not configured")
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid modules.base16 config")
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
		return nil, fmt.Errorf("modules.base16 must include at least base00/base05/base0D")
	}
	return out, nil
}

func parseColorReplacements(rules []string) (map[string]string, error) {
	out := map[string]string{}
	for _, r := range rules {
		pair := strings.SplitN(r, "=", 2)
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid --replace rule %q (expected old=new)", r)
		}
		oldC := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(pair[0]), "#"))
		newC := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(pair[1]), "#"))
		if len(oldC) != 6 || len(newC) != 6 {
			return nil, fmt.Errorf("invalid --replace rule %q (expected 6-digit hex)", r)
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

	hexRe := regexp.MustCompile(`(?i)#([0-9a-f]{3}|[0-9a-f]{6})\b`)
	cache := map[string]string{}

	return hexRe.ReplaceAllStringFunc(content, func(match string) string {
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

func extractPalette(colors map[string]string) []string {
	keys := []string{
		"base00", "base01", "base02", "base03",
		"base04", "base05", "base06", "base07",
		"base08", "base09", "base0A", "base0B",
		"base0C", "base0D", "base0E", "base0F",
		"base10", "base11", "base12", "base13", "base14", "base15", "base16", "base17",
	}
	out := make([]string, 0, len(keys))
	seen := map[string]bool{}
	for _, k := range keys {
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
	viewBoxRe := regexp.MustCompile(`(?i)viewBox\s*=\s*"[^"]*([0-9.]+)\s+([0-9.]+)"`)
	if m := viewBoxRe.FindStringSubmatch(svg); len(m) == 3 {
		w, _ := strconv.ParseFloat(m[1], 64)
		h, _ := strconv.ParseFloat(m[2], 64)
		if w > 0 && h > 0 {
			return w / h
		}
	}
	widthRe := regexp.MustCompile(`(?i)\bwidth\s*=\s*"([0-9.]+)(px)?"`)
	heightRe := regexp.MustCompile(`(?i)\bheight\s*=\s*"([0-9.]+)(px)?"`)
	wm := widthRe.FindStringSubmatch(svg)
	hm := heightRe.FindStringSubmatch(svg)
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
	draw.Draw(dst, rect, img, b.Min, draw.Over)
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
	scaleX := float64(sw) / float64(outW)
	scaleY := float64(sh) / float64(outH)

	for y := 0; y < outH; y++ {
		fy := (float64(y)+0.5)*scaleY - 0.5
		y0 := clampInt(int(math.Floor(fy)), 0, sh-1)
		y1 := clampInt(y0+1, 0, sh-1)
		wy := fy - float64(y0)
		if wy < 0 {
			wy = 0
		}
		for x := 0; x < outW; x++ {
			fx := (float64(x)+0.5)*scaleX - 0.5
			x0 := clampInt(int(math.Floor(fx)), 0, sw-1)
			x1 := clampInt(x0+1, 0, sw-1)
			wx := fx - float64(x0)
			if wx < 0 {
				wx = 0
			}

			c00 := color.NRGBAModel.Convert(src.At(sb.Min.X+x0, sb.Min.Y+y0)).(color.NRGBA)
			c10 := color.NRGBAModel.Convert(src.At(sb.Min.X+x1, sb.Min.Y+y0)).(color.NRGBA)
			c01 := color.NRGBAModel.Convert(src.At(sb.Min.X+x0, sb.Min.Y+y1)).(color.NRGBA)
			c11 := color.NRGBAModel.Convert(src.At(sb.Min.X+x1, sb.Min.Y+y1)).(color.NRGBA)

			r := bilerp(c00.R, c10.R, c01.R, c11.R, wx, wy)
			g := bilerp(c00.G, c10.G, c01.G, c11.G, wx, wy)
			b := bilerp(c00.B, c10.B, c01.B, c11.B, wx, wy)
			a := bilerp(c00.A, c10.A, c01.A, c11.A, wx, wy)
			dst.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: a})
		}
	}
	return dst
}

func bilerp(c00, c10, c01, c11 uint8, wx, wy float64) uint8 {
	v00 := float64(c00)
	v10 := float64(c10)
	v01 := float64(c01)
	v11 := float64(c11)
	top := v00 + (v10-v00)*wx
	bottom := v01 + (v11-v01)*wx
	v := top + (bottom-top)*wy
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(math.Round(v))
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
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
