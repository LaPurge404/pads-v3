package evolution

import "sync"

type AuditLog struct {
mu    sync.Mutex
Items []string
}

func NewAuditLog() *AuditLog {
return &AuditLog{
Items: make([]string, 0),
}
}

func (a *AuditLog) Record(event string) {
a.mu.Lock()
defer a.mu.Unlock()

a.Items = append(a.Items, event)
}
