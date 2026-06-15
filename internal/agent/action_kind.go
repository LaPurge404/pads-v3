package agent

// ActionKind defines the type of an action.
type ActionKind string

const (
ActionLog        ActionKind = "log"
ActionWriteFile  ActionKind = "write_file"
ActionRunCommand ActionKind = "run_command"
)

// TaskKind defines the type of a task.
type TaskKind string

const (
TaskFixBroken TaskKind = "fix_broken"
)

// IsValidActionKind validates strict ActionKind contracts.
func IsValidActionKind(k ActionKind) bool {
switch k {
case ActionLog, ActionWriteFile, ActionRunCommand:
return true
default:
return false
}
}
