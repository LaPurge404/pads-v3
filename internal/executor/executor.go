package executor

import (
"fmt"
"os"
"os/exec"

"pads-v3/internal/agent"
"pads-v3/internal/storage"
)

// Executor is the ONLY component allowed to produce side effects.
type Executor struct {
DB     *storage.DB
DryRun bool
}

func New(db *storage.DB, dryRun bool) *Executor {
return &Executor{
DB:     db,
DryRun: dryRun,
}
}

// Execute validates and executes a Plan, writing an audit event to L2 for each action.
func (e *Executor) Execute(plan agent.Plan) error {
for _, step := range plan.Steps {
if !agent.IsValidActionKind(step.Kind) {
e.audit(step.Kind, step.Target, "REJECTED", fmt.Sprintf("unknown action kind: %s", step.Kind))
return fmt.Errorf("unknown action kind: %s", step.Kind)
}

var execErr error
switch step.Kind {
case agent.ActionLog:
fmt.Printf("[EXEC LOG] %s\n", step.Target)

case agent.ActionWriteFile:
if e.DryRun {
fmt.Printf("[EXEC WRITE] would write to %s\n", step.Target)
} else {
execErr = e.writeFile(step.Target, step.Patch)
}

case agent.ActionRunCommand:
if e.DryRun {
fmt.Printf("[EXEC RUN] would run %v\n", step.Command)
} else {
execErr = e.runCommand(step.Command)
}
}

if execErr != nil {
e.audit(step.Kind, step.Target, "FAILED", execErr.Error())
return fmt.Errorf("executor: %s: %w", step.Kind, execErr)
}
e.audit(step.Kind, step.Target, "EXECUTED", "")
}
return nil
}

func (e *Executor) writeFile(path, content string) error {
return os.WriteFile(path, []byte(content), 0644)
}

func (e *Executor) runCommand(args []string) error {
if len(args) == 0 {
return fmt.Errorf("empty command")
}
cmd := exec.Command(args[0], args[1:]...)
output, err := cmd.CombinedOutput()
if err != nil {
return fmt.Errorf("command failed: %s: %w", string(output), err)
}
return nil
}

// audit records an executor action event in L2.
func (e *Executor) audit(kind agent.ActionKind, target, status, details string) {
if e.DB == nil {
return
}
payload := fmt.Sprintf(`{"kind":"%s","target":"%s","status":"%s","details":"%s"}`, kind, target, status, details)
eventID := fmt.Sprintf("exec-%s-%s-%d", kind, target, os.Getpid())
_, err := e.DB.InsertEvent(eventID, "EXECUTOR_ACTION", payload, 0)
if err != nil {
fmt.Printf("[EXEC AUDIT ERROR] %v\n", err)
}
}
