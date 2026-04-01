package runtime

import "time"

type Category string

const (
	CategoryBuild   Category = "build"
	CategoryTest    Category = "test"
	CategoryLint    Category = "lint"
	CategorySearch  Category = "search"
	CategoryCommand Category = "command"
)

type Request struct {
	Category       Category
	Command        string
	Args           []string
	Dir            string
	Env            []string
	Timeout        time.Duration
	MaxOutputBytes int64
}

type StartRequest struct {
	Request
	Name   string
	LogDir string
	Port   int
}

type ProcessState string

const (
	ProcessStateStarting ProcessState = "starting"
	ProcessStateRunning  ProcessState = "running"
	ProcessStateStopped  ProcessState = "stopped"
	ProcessStateExited   ProcessState = "exited"
	ProcessStateFailed   ProcessState = "failed"
	ProcessStateUnknown  ProcessState = "unknown"
)

type ProcessHandle struct {
	PID        int          `json:"pid"`
	Name       string       `json:"name,omitempty"`
	Category   Category     `json:"category"`
	Command    string       `json:"command"`
	Args       []string     `json:"args,omitempty"`
	Port       int          `json:"port,omitempty"`
	LogPath    string       `json:"log_path,omitempty"`
	State      ProcessState `json:"state"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at,omitempty"`
	ExitCode   int          `json:"exit_code,omitempty"`
	Running    bool         `json:"running"`
	Error      string       `json:"error,omitempty"`
}

type Result struct {
	Category        Category      `json:"category"`
	Command         string        `json:"command"`
	Args            []string      `json:"args,omitempty"`
	ExitCode        int           `json:"exit_code"`
	Stdout          string        `json:"stdout,omitempty"`
	Stderr          string        `json:"stderr,omitempty"`
	StartedAt       time.Time     `json:"started_at"`
	FinishedAt      time.Time     `json:"finished_at"`
	Duration        time.Duration `json:"duration"`
	TimedOut        bool          `json:"timed_out"`
	TruncatedStdout bool          `json:"truncated_stdout"`
	TruncatedStderr bool          `json:"truncated_stderr"`
}
