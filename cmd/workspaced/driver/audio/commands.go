package audio

import (
	"github.com/lucasew/workspaced/pkg/driver/audio"
)

func init() {
	Registry.Add("up", "Increase volume", audio.IncreaseVolume)
	Registry.Add("down", "Decrease volume", audio.DecreaseVolume)
	Registry.Add("mute", "Toggle mute", audio.ToggleMute)
	Registry.AddWithAliases("show", "Show current volume", []string{"status"}, audio.ShowStatus)
}
