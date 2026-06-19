package evolution

type WALBridge struct {
    mem  *WAL
    disk *WALStore
}

func NewWALBridge(mem *WAL, disk *WALStore) *WALBridge {
    return &WALBridge{
        mem:  mem,
        disk: disk,
    }
}

func (b *WALBridge) Append(candidate, current int, weight float64, mode Mode) (Entry, error) {
	entry, err := b.mem.Append(candidate, current, weight, mode)
	if err != nil {
		return entry, err
	}
	if err := b.disk.Append(entry); err != nil {
		return entry, err
	}
	return entry, nil
}
