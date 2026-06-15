package ci

// Status represents execution state
type Status string

const (
StatusPending Status = "PENDING"
StatusRunning Status = "RUNNING"
StatusSuccess Status = "SUCCESS"
StatusFailed  Status = "FAILED"
StatusSkipped Status = "SKIPPED"
)

// Step is a single CI unit
type Step struct {
ID         string
Name       string
Run        string
WorkingDir string
Needs      []string
}

// Job is a DAG node
type Job struct {
ID     string
Steps  []Step
Matrix Matrix
Needs  []string
}

// Context is runtime execution context
type Context struct {
JobID  string
StepID string
Vars   map[string]string
}
