package sessionruntime

func preparePrivateB4SnapshotForTest(m *Manager, lease B4DefinitionLease, selection B4DefinitionSelectionKey) (PrivateB4DefinitionCapture, PrivateB4Status) {
	var snapshot PrivateB4DefinitionCapture
	_, status := m.PreparePrivateB4DefinitionBorrowed(lease, selection, func(b PrivateB4DefinitionBorrow) bool {
		snapshot = b.Capture
		snapshot.Result = append([]byte(nil), b.Result...)
		return true
	})
	return snapshot, status
}

func commitPrivateB4SnapshotForTest(m *Manager, lease B4DefinitionLease, selection B4DefinitionSelectionKey, prepare func(PrivateB4DefinitionCapture) bool, publish func(PrivateB4DefinitionCapture)) (PrivateB4DefinitionCapture, PrivateB4Status) {
	var published PrivateB4DefinitionCapture
	_, status := m.CommitPrivateB4DefinitionBorrowed(lease, selection, func(b PrivateB4DefinitionBorrow) bool {
		capture := b.Capture
		capture.Result = append([]byte(nil), b.Result...)
		return prepare(capture)
	}, func(b PrivateB4DefinitionBorrow) {
		published = b.Capture
		published.Result = append([]byte(nil), b.Result...)
		publish(published)
	})
	if status != PrivateB4Selected {
		return PrivateB4DefinitionCapture{}, status
	}
	return published, status
}

func consumePrivateB4SnapshotForTest(m *Manager, lease B4DefinitionLease, selection B4DefinitionSelectionKey) (PrivateB4DefinitionCapture, PrivateB4Status) {
	var snapshot PrivateB4DefinitionCapture
	_, status := m.ConsumePrivateB4DefinitionBorrowed(lease, selection, func(b PrivateB4DefinitionBorrow) bool {
		snapshot = b.Capture
		snapshot.Result = append([]byte(nil), b.Result...)
		return true
	})
	return snapshot, status
}
