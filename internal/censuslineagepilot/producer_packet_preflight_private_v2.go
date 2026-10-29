package censuslineagepilot

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
)

// Stream the small packet wrapper before allocating typed arrays or decoding
// base64 carriers. Original embedded JSON is opaque here and is separately
// bounded/validated by its owning parser. This is not a general JSON CPU meter.
func preflightProducerPacketWirePrivateV2(raw []byte) error {
	type frame struct {
		kind         json.Delim
		field        string
		key          bool
		count, limit int
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var stack []frame
	tokens := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if len(stack) != 0 {
				return ErrInvalid
			}
			return nil
		}
		if err != nil {
			return err
		}
		tokens++
		if tokens > 100000 {
			return ErrProducerPacketOverBoundPrivateV2
		}
		if delim, ok := token.(json.Delim); ok && (delim == '}' || delim == ']') {
			if len(stack) == 0 {
				return ErrInvalid
			}
			stack = stack[:len(stack)-1]
			continue
		}
		field := ""
		if len(stack) > 0 {
			parent := &stack[len(stack)-1]
			if parent.kind == '{' && parent.key {
				key, ok := token.(string)
				if !ok || len(key) > 64 {
					return ErrInvalid
				}
				parent.field = key
				parent.key = false
				continue
			}
			field = parent.field
			if parent.kind == '{' {
				parent.key = true
			} else {
				parent.count++
				if parent.count > parent.limit {
					return ErrProducerPacketOverBoundPrivateV2
				}
			}
		}
		if delim, ok := token.(json.Delim); ok {
			if len(stack) >= 8 {
				return ErrProducerPacketOverBoundPrivateV2
			}
			f := frame{kind: delim, key: delim == '{'}
			if delim == '[' {
				if len(stack) != 1 {
					return ErrInvalid
				}
				switch field {
				case "files":
					f.limit = 16
				case "batches":
					f.limit = 63
				case "ordered_roles":
					f.limit = 7 + 16 + 4*63
				default:
					return ErrInvalid
				}
			}
			stack = append(stack, f)
			continue
		}
		if value, ok := token.(string); ok {
			limit := 8192
			switch field {
			case "frozen_spec_base64", "frozen_manifest_base64", "nomination_selection_base64", "file_plan_base64", "effective_policy_base64", "actual_enumeration_origin_base64", "runtime_metadata_base64", "canonical_batch_seeds_base64", "full_capture_origin_base64", "canonical_manifest_input_base64":
				limit = base64.StdEncoding.EncodedLen(scopeMetadataPrivateV2)
			case "body_base64":
				limit = base64.StdEncoding.EncodedLen(1 << 20)
			case "source_artifact_base64":
				limit = base64.StdEncoding.EncodedLen(producerPacketArtifactBytesPrivateV2)
			}
			if len(value) > limit {
				return ErrProducerPacketOverBoundPrivateV2
			}
		}
	}
}
