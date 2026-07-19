package dbus

import (
	"context"
	"fmt"
	"strings"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/media"

	"github.com/godbus/dbus/v5"
)

func init() {
	driver.Register[media.Driver](&Factory{})
}

type Factory struct{}

func (f *Factory) ID() string   { return "media_mpris" }
func (f *Factory) Name() string { return "MPRIS (DBus)" }

func (f *Factory) CheckCompatibility(ctx context.Context) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("%w: connect to session bus: %w", driver.ErrIncompatible, err)
	}
	var names []string
	err = conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
		return fmt.Errorf("%w: list dbus names: %w", driver.ErrIncompatible, err)
	}

	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			return nil
		}
	}

	return fmt.Errorf("%w: no MPRIS players found on DBus", driver.ErrIncompatible)
}

func (f *Factory) New(ctx context.Context) (media.Driver, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}
	return &Driver{conn: conn}, nil
}

type Driver struct {
	conn *dbus.Conn
}

func (d *Driver) getBestPlayer(ctx context.Context) (dbus.BusObject, string, error) {
	var names []string
	err := d.conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
		return nil, "", err
	}

	var players []string
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			players = append(players, name)
		}
	}

	if len(players) == 0 {
		return nil, "", driver.ErrNotFound
	}

	type playerInfo struct {
		name   string
		status string
		obj    dbus.BusObject
	}

	var infos []playerInfo
	for _, p := range players {
		obj := d.conn.Object(p, "/org/mpris/MediaPlayer2")
		statusVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus")
		if err != nil {
			continue
		}
		status, ok := statusVar.Value().(string)
		if !ok {
			continue
		}
		infos = append(infos, playerInfo{
			name:   p,
			status: status,
			obj:    obj,
		})
	}

	if len(infos) == 0 {
		return nil, "", driver.ErrNotFound
	}

	var best *playerInfo
	statusPriority := map[string]int{"Playing": 3, "Paused": 2, "Stopped": 1}

	for i := range infos {
		p := &infos[i]
		if best == nil || statusPriority[p.status] > statusPriority[best.status] {
			best = p
		}
	}

	return best.obj, best.name, nil
}

func (d *Driver) callAction(ctx context.Context, action string) error {
	obj, _, err := d.getBestPlayer(ctx)
	if err != nil {
		return err
	}
	return obj.Call("org.mpris.MediaPlayer2.Player."+action, 0).Err
}

func (d *Driver) Next(ctx context.Context) error      { return d.callAction(ctx, "Next") }
func (d *Driver) Previous(ctx context.Context) error  { return d.callAction(ctx, "Previous") }
func (d *Driver) PlayPause(ctx context.Context) error { return d.callAction(ctx, "PlayPause") }
func (d *Driver) Stop(ctx context.Context) error      { return d.callAction(ctx, "Stop") }

func (d *Driver) GetMetadata(ctx context.Context) (*media.Metadata, error) {
	obj, name, err := d.getBestPlayer(ctx)
	if err != nil {
		return nil, err
	}

	statusVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus")
	if err != nil {
		return nil, err
	}
	status, ok := statusVar.Value().(string)
	if !ok {
		return nil, fmt.Errorf("playback status: unexpected type %T", statusVar.Value())
	}

	metadataVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Metadata")
	if err != nil {
		return nil, err
	}

	m, ok := metadataVar.Value().(map[string]dbus.Variant)
	if !ok {
		return nil, fmt.Errorf("metadata: unexpected type %T", metadataVar.Value())
	}
	res := &media.Metadata{
		Player: name,
		Status: media.PlaybackStatus(status),
	}

	if v, ok := m["xesam:title"]; ok {
		if title, ok := v.Value().(string); ok {
			res.Title = title
		}
	}
	if v, ok := m["xesam:artist"]; ok {
		switch val := v.Value().(type) {
		case []string:
			res.Artist = strings.Join(val, ", ")
		case []any:
			var artists []string
			for _, a := range val {
				if s, ok := a.(string); ok {
					artists = append(artists, s)
				}
			}
			res.Artist = strings.Join(artists, ", ")
		case string:
			res.Artist = val
		}
	}
	if v, ok := m["mpris:artUrl"]; ok {
		if artUrl, ok := v.Value().(string); ok {
			res.ArtUrl = artUrl
		}
	}
	if v, ok := m["mpris:length"]; ok {
		switch val := v.Value().(type) {
		case int64:
			res.Length = val
		case uint64:
			res.Length = int64(val)
		}
	}

	posVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Position")
	if err == nil {
		switch val := posVar.Value().(type) {
		case int64:
			res.Position = val
		case uint64:
			res.Position = int64(val)
		}
	}

	return res, nil
}

func (d *Driver) Watch(ctx context.Context, callback func(*media.Metadata)) error {
	rule := "type='signal',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged',path='/org/mpris/MediaPlayer2'"
	if err := d.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Err; err != nil {
		return err
	}

	c := make(chan *dbus.Signal, 10)
	d.conn.Signal(c)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case signal := <-c:
			if len(signal.Body) < 2 {
				continue
			}
			iface, ok := signal.Body[0].(string)
			if !ok || iface != "org.mpris.MediaPlayer2.Player" {
				continue
			}
			changed, ok := signal.Body[1].(map[string]dbus.Variant)
			if !ok {
				continue
			}
			if _, ok := changed["Metadata"]; ok {
				meta, err := d.GetMetadata(ctx)
				if err == nil {
					callback(meta)
				}
			}
		}
	}
}
