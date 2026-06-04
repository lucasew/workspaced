package apps

import (
	"fmt"
	"os"

	"github.com/schollz/progressbar/v3"
)

func newDownloadProgressBar(name string, size int64) *progressbar.ProgressBar {
	description := fmt.Sprintf("downloading %s", name)
	if size > 0 {
		return progressbar.NewOptions64(
			size,
			progressbar.OptionSetDescription(description),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(24),
			progressbar.OptionThrottle(65),
			progressbar.OptionShowCount(),
			progressbar.OptionClearOnFinish(),
		)
	}

	return progressbar.NewOptions64(
		-1,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetWidth(24),
		progressbar.OptionThrottle(65),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionClearOnFinish(),
	)
}
