package adr0011methodresult

// notifyPrivateCEntry is a test-only notification with no return channel.
// Panics from diagnostic code cannot alter the private candidate decision.
func notifyPrivateCEntry(observer func()) {
	if observer == nil {
		return
	}
	defer func() { _ = recover() }()
	observer()
}
