package dag

// NodeType categorizes the kind of work a node represents.
type NodeType string

const (
	NodeJobStart NodeType = "JOB_START"
	NodeJobEnd   NodeType = "JOB_END"
	NodeStepRun  NodeType = "STEP_RUN"
	NodeCache    NodeType = "CACHE"
	NodeArtifact NodeType = "ARTIFACT"
)

// Node represents a single vertex in the causal DAG.
// It does NOT contain events; events are produced by the Executor.
type Node struct {
	ID           string
	Type         NodeType
	JobID        string
	StepID       string
	Action       ActionSpec
	Dependencies []string
}

// ActionSpec describes the action to perform for a StepRun node.
type ActionSpec struct {
	Command    string
	WorkingDir string
}

// Graph is the full causal DAG.
type Graph struct {
	Nodes map[string]*Node
}
