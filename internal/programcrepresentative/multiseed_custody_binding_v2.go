package programcrepresentative

import "fmt"

// AdmitMechanicsArtifactV2 checks the typed artifact before any frozen accessor
// clones it. It does not add qualification or replace source reconstruction.
func AdmitMechanicsArtifactV2(m MechanicsV2) error {
	if _, err := CanonicalSizeMechanicsIdentityDTOv2(m.dto); err != nil {
		return err
	}
	if !m.Trusted() {
		return fmt.Errorf("untrusted v2 mechanics")
	}
	return nil
}

// BindReconstructedCustodyV2 retains the frozen v1 domain/bytes. The census
// owner supplies values reconstructed from the verified receipt/source only.
func BindReconstructedCustodyV2(c CustodyIdentityDTO) (CustodyIdentityDTO, error) {
	if _, err := CanonicalSizeCustodyIdentityDTO(c); err != nil {
		return CustodyIdentityDTO{}, err
	}
	if c.SchemaVersion != CustodySchemaVersion || c.WorkflowKey == "" || c.ReceiptOrdinal <= 0 || c.ConstituentOrdinal < 0 || c.PublicationSelector == "" || c.ConstituentSelector == "" || c.ExecutionBundleID == "" || !validDigest(c.TerminalProgramCHeadSelector) || !validDigest(c.ReceiptSelector) || !validDigest(c.NativeV5SHA256) || c.NativeV5ByteLength == 0 {
		return CustodyIdentityDTO{}, fmt.Errorf("invalid reconstructed v1 custody")
	}
	c.Identity = ""
	n, err := CanonicalSizeCustodyIdentityDTO(c)
	raw, err := marshalSizedV2(c, n, err)
	if err != nil {
		return CustodyIdentityDTO{}, err
	}
	c.Identity = multiSeedIdentityV2(custodyDomain, raw)
	return c, nil
}
