package history

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"workspaced/pkg/types"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "History management",
	}
	Registry.FillCommands(cmd)
	return cmd
}

func ingestBash() ([]types.HistoryEvent, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".bash_history")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []types.HistoryEvent
	scanner := bufio.NewScanner(file)
	var lastTimestamp int64
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			ts, err := strconv.ParseInt(line[1:], 10, 64)
			if err == nil {
				lastTimestamp = ts
				continue
			}
		}
		if line == "" {
			continue
		}
		events = append(events, types.HistoryEvent{
			Command:   line,
			Timestamp: lastTimestamp,
			Cwd:       "/dev/null",
			ExitCode:  0,
			Duration:  0,
		})
	}
	return events, scanner.Err()
}

func ingestAtuin() ([]types.HistoryEvent, error) {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".local/share/atuin/history.db")

	// Open atuin database using the registered sqlite driver
	dbConn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open atuin database: %w", err)
	}
	defer dbConn.Close()

	rows, err := dbConn.Query("SELECT command, cwd, timestamp, exit, duration FROM history")
	if err != nil {
		return nil, fmt.Errorf("failed to query atuin database: %w", err)
	}
	defer rows.Close()

	var events []types.HistoryEvent
	for rows.Next() {
		var e types.HistoryEvent
		var ts int64
		var exitCode int
		var duration int64
		if err := rows.Scan(&e.Command, &e.Cwd, &ts, &exitCode, &duration); err != nil {
			return nil, err
		}
		// Atuin timestamp is nanoseconds or microseconds? Usually nanoseconds in newer versions.
		// Let's assume it needs conversion to seconds if it's too large.
		if ts > 2000000000 {
			ts = ts / 1000000000
		}
		if e.Cwd == "" {
			e.Cwd = "/dev/null"
		}
		e.Timestamp = ts
		e.ExitCode = exitCode
		e.Duration = duration / 1000000 // nano to milli
		events = append(events, e)
	}
	return events, nil
}

func getSocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(runtimeDir, "workspaced.sock")
}

func sendHistoryEvent(event types.HistoryEvent) error {
	socketPath := getSocketPath()
	dialer := websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout("unix", socketPath, 200*time.Millisecond)
		},
	}

	conn, _, err := dialer.Dial("ws://localhost/ws", nil)
	if err != nil {
		return err // Return error so caller can fallback
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	payload, _ := json.Marshal(event)
	packet := types.StreamPacket{
		Type:    "history_event",
		Payload: payload,
	}

	return conn.WriteJSON(packet)
}
