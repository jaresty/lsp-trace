package provider

import (
	"context"
	"errors"
)

// ProvisionedAdmissionResolver resolves caller selectors against host-owned provisioning.
type ProvisionedAdmissionResolver struct{}

func NewAdmissionResolver(Provisioned, string) (*ProvisionedAdmissionResolver, error) {
	return &ProvisionedAdmissionResolver{}, nil
}

func (*ProvisionedAdmissionResolver) Admit(context.Context, Selection) (Admission, error) {
	return Admission{}, errors.New("provider admission not implemented")
}
