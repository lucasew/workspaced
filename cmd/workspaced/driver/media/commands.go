package media

import (
	"context"

	"github.com/lucasew/workspaced/pkg/driver/media"
)

func init() {
	addAction := func(use, short, action string) {
		Registry.Add(use, short, func(ctx context.Context) error {
			return media.RunAction(ctx, action)
		})
	}
	addAction("next", "Next media", "next")
	addAction("previous", "Previous media", "previous")
	addAction("play-pause", "Play or pause media", "play-pause")
	addAction("stop", "Stop media", "stop")
	addAction("show", "Show media metadata", "show")
}
