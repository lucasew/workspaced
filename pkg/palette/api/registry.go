package api

import (
	"fmt"
	"maps"
	"slices"
)

// drivers holds palette extraction implementations keyed by Name().
// Registration happens during init(); no mutex is needed at runtime.
var drivers = map[string]Driver{}

// Register adds a palette extraction driver.
// Panics if name is empty or already registered.
func Register(d Driver) {
	name := d.Name()
	if name == "" {
		panic("palette: driver name cannot be empty")
	}
	if _, ok := drivers[name]; ok {
		panic(fmt.Sprintf("palette: driver %q registered twice", name))
	}
	drivers[name] = d
}

// Get returns the registered driver with the given name.
func Get(name string) (Driver, error) {
	d, ok := drivers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDriverNotFound, name)
	}
	return d, nil
}

// List returns registered drivers sorted by name.
func List() []Driver {
	names := Names()
	out := make([]Driver, 0, len(names))
	for _, name := range names {
		out = append(out, drivers[name])
	}
	return out
}

// Names returns registered driver names in sorted order.
func Names() []string {
	return slices.Sorted(maps.Keys(drivers))
}
