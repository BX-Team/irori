package models

import "time"

type ServerState string

const (
	StateStopped  ServerState = "stopped"
	StateStarting ServerState = "starting"
	StateRunning  ServerState = "running"
	StateStopping ServerState = "stopping"
	StateCrashed  ServerState = "crashed"
)

func (s ServerState) Label() string {
	switch s {
	case StateStopped:
		return "OFFLINE"
	case StateStarting:
		return "STARTING"
	case StateRunning:
		return "RUNNING"
	case StateStopping:
		return "STOPPING"
	case StateCrashed:
		return "CRASHED"
	}
	return "UNKNOWN"
}

func (s ServerState) IsUp() bool {
	return s == StateStarting || s == StateRunning || s == StateStopping
}

type LogLevel string

const (
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
	LevelDebug LogLevel = "debug"
	LevelChat  LogLevel = "chat"
	LevelIrori LogLevel = "irori"
)

type LogLine struct {
	Seq   uint64    `json:"seq"`
	TS    time.Time `json:"ts"`
	Text  string    `json:"text"`
	Level LogLevel  `json:"level"`
}

type Stats struct {
	CPU       float64 `json:"cpu"`
	RSSMB     float64 `json:"rssMB"`
	UptimeSec int64   `json:"uptimeSec"`
	Players   int     `json:"players"`
	MaxPlayer int     `json:"maxPlayers"`
}

type Status struct {
	State   ServerState `json:"state"`
	PID     int         `json:"pid"`
	Since   time.Time   `json:"since"`
	Stats   Stats       `json:"stats"`
	LastErr string      `json:"lastErr,omitempty"`
}
