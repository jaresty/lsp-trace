// Package objectadmission defines the repository-internal callback boundary for
// transaction-owned logical-object admission. It owns no counters or lifecycle.
package objectadmission

// ObjectAdmitter reserves one logical-object admission before invoking fn.
// Implementations commit the admission only when fn returns nil and otherwise
// roll the reservation back. The callback must run without owner locks held.
type ObjectAdmitter interface {
	WithObjectAdmission(func() error) error
}
