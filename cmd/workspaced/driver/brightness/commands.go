package brightness

import (
	"workspaced/pkg/driver/brightness"
)

func init() {
	Registry.Add("up", "Increase brightness", brightness.IncreaseBrightness)
	Registry.Add("down", "Decrease brightness", brightness.DecreaseBrightness)
	Registry.AddWithAliases("show", "Show current brightness", []string{"status"}, brightness.ShowStatus)
}
