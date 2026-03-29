package preset

// RunStatus describes the state of a preset's last scheduler run.
type RunStatus string

const (
	RunStatusIdle      RunStatus = "idle"
	RunStatusRunning   RunStatus = "running"
	RunStatusSuccess   RunStatus = "success"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)
