package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"workspaced/pkg/executil"
	"workspaced/pkg/registry"
	"workspaced/pkg/types"

	_ "workspaced/pkg/driver/prelude"

	"github.com/gorilla/websocket"

	"github.com/spf13/cobra"
)

var Registry registry.CommandRegistry

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:              "utils",
		Short:            "Miscelaneous commands that are not necessarily related to a driver",
		TraverseChildren: true,
	}
	Registry.FillCommands(cmd)

	return cmd
}

func FindCommand(name string, args []string) (*cobra.Command, []string, error) {
	return GetCommand().Find(append([]string{name}, args...))
}

func getSocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(runtimeDir, "workspaced.sock")
}

func TryRemoteRaw(cmdName string, args []string) (string, bool, error) {
	socketPath := getSocketPath()
	slog.Info("connecting to daemon", "socket", socketPath, "cmd", cmdName, "args", args)

	dialer := websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout("unix", socketPath, 1*time.Second)
		},
	}

	conn, _, err := dialer.Dial("ws://localhost/ws", nil)
	if err != nil {
		slog.Info("daemon not reachable, running locally", "error", err)
		return "", false, nil
	}
	defer func() { _ = conn.Close() }()

	// Get client binary hash
	clientHash, _ := executil.GetBinaryHash()

	req := types.Request{
		Command:    cmdName,
		Args:       args,
		Env:        os.Environ(),
		BinaryHash: clientHash,
	}

	// Send request as a StreamPacket
	payload, _ := json.Marshal(req)
	packet := types.StreamPacket{
		Type:    "request",
		Payload: payload,
	}

	if err := conn.WriteJSON(packet); err != nil {
		return "", true, fmt.Errorf("failed to send request: %w", err)
	}

	for {
		var packet types.StreamPacket
		if err := conn.ReadJSON(&packet); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slog.Debug("ws read error", "error", err)
			}
			return "", true, fmt.Errorf("failed to read response: %w", err)
		}

		switch packet.Type {
		case "log":
			var entry types.LogEntry
			if err := json.Unmarshal(packet.Payload, &entry); err != nil {
				continue
			}
			level := slog.LevelInfo
			switch entry.Level {
			case "DEBUG":
				level = slog.LevelDebug
			case "WARN":
				level = slog.LevelWarn
			case "ERROR":
				level = slog.LevelError
			}
			attrs := []any{}
			for k, v := range entry.Attrs {
				attrs = append(attrs, slog.Any(k, v))
			}
			slog.Log(context.Background(), level, entry.Message, attrs...)
		case "stdout":
			var out string
			if err := json.Unmarshal(packet.Payload, &out); err == nil {
				fmt.Print(out)
			}
		case "stderr":
			var out string
			if err := json.Unmarshal(packet.Payload, &out); err == nil {
				fmt.Fprint(os.Stderr, out)
			}
		case "result":
			var resp types.Response
			if err := json.Unmarshal(packet.Payload, &resp); err != nil {
				return "", true, fmt.Errorf("failed to parse result: %w", err)
			}
			if resp.Error != "" {
				// Check if daemon is restarting itself
				if resp.Error == "DAEMON_RESTARTING" || resp.Error == "DAEMON_RESTART_NEEDED" {
					slog.Info("daemon restarting with new binary, retrying locally")

					// Daemon is exec'ing itself, just wait a bit and run locally
					// Next command will connect to the new daemon
					time.Sleep(200 * time.Millisecond)

					// Run locally this time, next call will hit new daemon
					return "", false, nil
				}
				return "", true, fmt.Errorf("%s", resp.Error)
			}
			return "", true, nil
		}
	}
}
