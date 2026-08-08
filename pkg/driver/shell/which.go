package shell

import (
	"context"

	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
)

// whichFactory selects a shell by resolving a binary name on PATH.
// bash and sh share this implementation; only id/name/binary differ.
type whichFactory struct {
	id, name, binary string
}

func (f *whichFactory) ID() string   { return f.id }
func (f *whichFactory) Name() string { return f.name }

func (f *whichFactory) CheckCompatibility(ctx context.Context) error {
	_, err := execdriver.Which(ctx, f.binary)
	return err
}

func (f *whichFactory) New(ctx context.Context) (Driver, error) {
	return &whichDriver{binary: f.binary}, nil
}

type whichDriver struct {
	binary string
}

func (d *whichDriver) Path(ctx context.Context) (string, error) {
	return execdriver.Which(ctx, d.binary)
}

// RegisterWhich registers a PATH-resolved shell implementation.
func RegisterWhich(id, name, binary string) {
	driver.Register[Driver](&whichFactory{id: id, name: name, binary: binary})
}
