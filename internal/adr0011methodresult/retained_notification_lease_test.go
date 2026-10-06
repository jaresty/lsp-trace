package adr0011methodresult

import (
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

func privateRetainedNotificationCount(result sessionruntime.RoundTripResult) (int, bool) {
	lease, ok := result.PrivateRetainedNotificationLease()
	if !ok {
		return 0, false
	}
	count := 0
	if err := lease.WithNotifications(func(messages []lspwire.Message) error {
		count = len(messages)
		return nil
	}); err != nil {
		return 0, false
	}
	return count, lease.Release()
}
