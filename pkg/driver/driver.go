// Package driver provides the generic pluggable driver system.
//
// All interaction with the host system (audio, clipboard, notifications, window
// managers, power, screenshots, terminals, etc.) goes through capability
// interfaces defined under subdirectories. Concrete implementations register
// themselves using driver.Register[T] (usually from an init function).
//
// Selection happens via driver.Get[T](ctx), which respects configured weights
// (from workspaced.cue) and calls CheckCompatibility on candidates.
//
// For testing and debugging you can force a particular implementation using an
// environment variable:
//
//	WORKSPACED_FORCE_DRIVER=rsync_gokrazy
//	WORKSPACED_FORCE_RSYNC_DRIVER=rsync_gokrazy
//
// The forced driver (if registered for the interface) is given an effective
// weight of 101 so it is considered first. If its CheckCompatibility fails,
// normal fallback to other candidates occurs.
//
// The central list of all driver implementations is pulled in via the
// pkg/driver/prelude package (imported with blank import from cmd/workspaced/root.go).
package driver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"

	"workspaced/internal/compat"
	"workspaced/pkg/logging"
)

var (
	ErrIncompatible = compat.ErrIncompatible
	ErrNotInterface = errors.New("driver is not an interface")
	ErrNotFound     = errors.New("driver not found")
)

type DriverFactory[T any] interface {
	ID() string   // Unique slug for the driver (e.g. "wayland_swaybg")
	Name() string // Human readable name
	CheckCompatibility(ctx context.Context) error
	New(ctx context.Context) (T, error)
}

type doctorEntry struct {
	InterfaceType reflect.Type
	InterfaceName string
	FactoryType   reflect.Type // Type of the factory struct
	DriverID      string
	DriverName    string
	Check         func(context.Context) error
}

type validationResult struct {
	once sync.Once
	err  error
}

var (
	mu            sync.RWMutex
	Drivers       = map[reflect.Type]map[string]any{}
	driverWeights = map[string]map[string]int{}
	doctorList    = []doctorEntry{}

	// validationCache allows each driver's CheckCompatibility to run at most once.
	// Using sync.Map so cachedCheck does not require the global mu (prevents
	// deadlock when Doctor snapshots under RLock and checks later call Get[]).
	validationCache sync.Map // string (driver ID) -> *validationResult
)

// cachedCheck runs check at most once per driver ID for the lifetime of the process.
func cachedCheck(id string, check func(context.Context) error, ctx context.Context) error {
	val, _ := validationCache.LoadOrStore(id, &validationResult{})
	vr := val.(*validationResult)
	vr.once.Do(func() { vr.err = check(ctx) })
	return vr.err
}

// SetWeights configures driver priorities. Weights must be between 0 and 100.
func SetWeights(w map[string]map[string]int) error {
	mu.Lock()
	defer mu.Unlock()

	for iface, drivers := range w {
		for id, weight := range drivers {
			if weight < 0 || weight > 100 {
				return fmt.Errorf("invalid weight %d for driver %q in interface %q: must be between 0 and 100", weight, id, iface)
			}
		}
	}
	for ifaceType, factories := range Drivers {
		ifaceName := getInterfaceName(ifaceType)
		configured, ok := w[ifaceName]
		if !ok {
			return fmt.Errorf("missing driver weights for interface %q", ifaceName)
		}
		for id := range factories {
			if _, ok := configured[id]; !ok {
				return fmt.Errorf("missing driver weight for %q in interface %q", id, ifaceName)
			}
		}
	}
	driverWeights = w
	return nil
}

// forceDriverFromEnv returns a driver ID that should be preferred for the given
// interface, if any is requested via environment variable.
//
// Supported variables (checked in order):
//   - WORKSPACED_FORCE_DRIVER=<id>          (applies to any interface)
//   - WORKSPACED_FORCE_RSYNC_DRIVER=<id>    (for rsync.Driver)
//   - WORKSPACED_FORCE_CLIPBOARD_DRIVER=<id>
//   - etc. (derived from the interface name)
//
// This is intended for testing and debugging (e.g. forcing the pure-Go
// gokrazy rsync implementation even when the native binary is present).
func forceDriverFromEnv(ifaceName string) string {
	if v := os.Getenv("WORKSPACED_FORCE_DRIVER"); v != "" {
		return v
	}
	// Derive a friendly key, e.g.
	//   "workspaced/pkg/driver/rsync.Driver"        -> WORKSPACED_FORCE_RSYNC_DRIVER
	//   "workspaced/pkg/driver/clipboard.Driver"    -> WORKSPACED_FORCE_CLIPBOARD_DRIVER
	//   "workspaced/pkg/driver/dialog.Chooser"      -> WORKSPACED_FORCE_DIALOG_CHOOSER_DRIVER
	key := ifaceName
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		key = key[idx+1:]
	}
	key = strings.TrimSuffix(key, ".Driver")
	key = strings.ReplaceAll(key, ".", "_")
	key = "WORKSPACED_FORCE_" + strings.ToUpper(key) + "_DRIVER"
	if v := os.Getenv(key); v != "" {
		return v
	}
	return ""
}

// effectiveWeight returns the weight to use for sorting, taking config weights
// plus any active environment variable force into account.
// A forced driver gets a weight > 100 so it reliably sorts first.
func effectiveWeight(weights map[string]int, driverID, ifaceName string) int {
	if forced := forceDriverFromEnv(ifaceName); forced != "" && forced == driverID {
		return 101
	}
	return getConfiguredWeight(weights, driverID)
}

func Register[T any](factory DriverFactory[T]) {
	mu.Lock()
	defer mu.Unlock()

	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Interface {
		panic(fmt.Errorf("driver %s is not an interface", t.String()))
	}
	id := factory.ID()
	if id == "" {
		panic(fmt.Errorf("driver for %s registered with empty ID", t.String()))
	}

	if _, ok := Drivers[t]; !ok {
		Drivers[t] = make(map[string]any)
	}
	if _, ok := Drivers[t][id]; ok {
		panic(fmt.Errorf("driver ID %q already registered for interface %s", id, t.String()))
	}
	Drivers[t][id] = factory

	doctorList = append(doctorList, doctorEntry{
		InterfaceType: t,
		InterfaceName: getInterfaceName(t),
		FactoryType:   reflect.TypeOf(factory),
		DriverID:      id,
		DriverName:    factory.Name(),
		Check:         factory.CheckCompatibility,
	})
}

func getInterfaceName(t reflect.Type) string {
	if t.PkgPath() != "" {
		return t.PkgPath() + "." + t.Name()
	}
	return t.String()
}

func RegisteredWeightShape() map[string][]string {
	mu.RLock()
	defer mu.RUnlock()

	shape := make(map[string][]string, len(Drivers))
	for ifaceType, factories := range Drivers {
		ifaceName := getInterfaceName(ifaceType)
		ids := make([]string, 0, len(factories))
		for id := range factories {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		shape[ifaceName] = ids
	}
	return shape
}

func getConfiguredWeight(weights map[string]int, driverID string) int {
	return weights[driverID]
}

func Get[T any](ctx context.Context) (T, error) {
	mu.RLock()
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Interface {
		mu.RUnlock()
		var zero T
		return zero, ErrNotInterface
	}

	ifaceName := getInterfaceName(t)
	weights := driverWeights[ifaceName]
	logger := logging.GetLogger(ctx)
	logger.Debug("loading driver weights", "interface", ifaceName, "weights", weights, "all_weights", driverWeights)

	factories := make([]DriverFactory[T], 0)
	if byID, ok := Drivers[t]; ok {
		for _, entry := range byID {
			factories = append(factories, entry.(DriverFactory[T]))
		}
	}

	// Apply any WORKSPACED_FORCE_*_DRIVER override for testing/debug.
	// We only consider it "active" for this interface if one of the registered
	// drivers actually has the forced ID (prevents log spam from the generic
	// WORKSPACED_FORCE_DRIVER var).
	forced := forceDriverFromEnv(ifaceName)
	if forced != "" {
		hasMatching := false
		for _, f := range factories {
			if f.ID() == forced {
				hasMatching = true
				break
			}
		}
		if hasMatching {
			logger.Info("driver force active via environment variable",
				"interface", ifaceName, "forced_driver", forced)
		}
	}

	mu.RUnlock()

	var zero T
	if len(factories) == 0 {
		return zero, ErrNotFound
	}

	// Log all factories before sorting
	logger.Debug("available driver factories", "interface", ifaceName, "count", len(factories))
	for _, f := range factories {
		w := effectiveWeight(weights, f.ID(), ifaceName)
		logger.Debug("factory registered", "interface", ifaceName, "id", f.ID(), "name", f.Name(), "weight", w)
	}

	// Sort factories by (effective) weight then ID.
	// A forced driver via env gets weight 101 so it is tried first.
	sort.Slice(factories, func(i, j int) bool {
		wi := effectiveWeight(weights, factories[i].ID(), ifaceName)
		wj := effectiveWeight(weights, factories[j].ID(), ifaceName)

		if wi != wj {
			return wi > wj // Higher weight first
		}
		return factories[i].ID() < factories[j].ID() // Deterministic fallback
	})

	var report []string

	for _, factory := range factories {
		weight := effectiveWeight(weights, factory.ID(), ifaceName)

		if err := cachedCheck(factory.ID(), factory.CheckCompatibility, ctx); err != nil {
			report = append(report, fmt.Sprintf("❌ [SKIP] %s (%s) weight=%d: %v", factory.ID(), factory.Name(), weight, err))
			logger.Debug("driver skipped", "interface", ifaceName, "id", factory.ID(), "name", factory.Name(), "weight", weight, "error", err)
			continue
		}

		instance, err := factory.New(ctx)
		if err != nil {
			report = append(report, fmt.Sprintf("⚠️ [FAIL] %s (%s) weight=%d: initialization failed: %v", factory.ID(), factory.Name(), weight, err))
			logger.Debug("driver init failed", "interface", ifaceName, "id", factory.ID(), "name", factory.Name(), "weight", weight, "error", err)
			continue
		}

		logger.Debug("driver selected", "interface", ifaceName, "id", factory.ID(), "name", factory.Name(), "weight", weight)
		return instance, nil
	}

	return zero, fmt.Errorf("no available driver for %s:\n%s", t.String(), strings.Join(report, "\n"))
}

type DriverStatus struct {
	ID          string
	Name        string
	FactoryType reflect.Type
	Weight      int
	Available   bool
	Selected    bool
	Error       error
}

type InterfaceStatus struct {
	Name    string
	Drivers []DriverStatus
}

// Doctor returns the status of all registered drivers
func Doctor(ctx context.Context) []InterfaceStatus {
	mu.RLock()

	byType := make(map[reflect.Type][]doctorEntry)
	for _, d := range doctorList {
		byType[d.InterfaceType] = append(byType[d.InterfaceType], d)
	}

	weightsSnapshot := make(map[string]map[string]int, len(driverWeights))
	for name, w := range driverWeights {
		inner := make(map[string]int, len(w))
		for k, v := range w {
			inner[k] = v
		}
		weightsSnapshot[name] = inner
	}

	mu.RUnlock()

	var types []reflect.Type
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		return getInterfaceName(types[i]) < getInterfaceName(types[j])
	})

	var result []InterfaceStatus

	for _, t := range types {
		entries := byType[t]
		ifaceName := getInterfaceName(t)
		weights := weightsSnapshot[ifaceName]
		ifaceStatus := InterfaceStatus{
			Name: ifaceName,
		}

		for _, d := range entries {
			err := cachedCheck(d.DriverID, d.Check, ctx)
			weight := effectiveWeight(weights, d.DriverID, ifaceName)
			status := DriverStatus{
				ID:          d.DriverID,
				Name:        d.DriverName,
				FactoryType: d.FactoryType,
				Weight:      weight,
				Available:   err == nil,
				Error:       err,
			}
			ifaceStatus.Drivers = append(ifaceStatus.Drivers, status)
		}

		// Sort drivers in doctor report
		sort.Slice(ifaceStatus.Drivers, func(i, j int) bool {
			if ifaceStatus.Drivers[i].Weight != ifaceStatus.Drivers[j].Weight {
				return ifaceStatus.Drivers[i].Weight > ifaceStatus.Drivers[j].Weight
			}
			return ifaceStatus.Drivers[i].ID < ifaceStatus.Drivers[j].ID
		})

		// Mark selected candidate
		for i := range ifaceStatus.Drivers {
			if ifaceStatus.Drivers[i].Available {
				ifaceStatus.Drivers[i].Selected = true
				break
			}
		}

		result = append(result, ifaceStatus)
	}

	return result
}
