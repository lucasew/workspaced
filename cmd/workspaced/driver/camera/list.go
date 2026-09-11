package camera

import (
	"context"
	"fmt"
	"os"

	"github.com/lucasew/workspaced/pkg/driver"
	cameraapi "github.com/lucasew/workspaced/pkg/driver/camera"
)

type List struct{}

func (List) Description() string { return "List cameras" }

func (*List) Run(ctx context.Context) error {
	drv, err := driver.Get[cameraapi.Driver](ctx)
	if err != nil {
		return err
	}
	cams, err := drv.List(ctx)
	if err != nil {
		return err
	}
	if len(cams) == 0 {
		return ErrNoCamerasFound
	}
	for _, cam := range cams {
		fmt.Fprintf(os.Stdout, "%s\t%s\n", cam.ID(), cam.Name())
	}
	return nil
}
