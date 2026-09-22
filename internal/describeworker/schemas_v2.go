package describeworker

import (
	"embed"
	"errors"
)

const SemanticSchemaV2 = "lsp-trace.describe-semantic.v2"

//go:embed schemas/*.v2.schema.json
var responseV2Schemas embed.FS

// SchemaV2 returns immutable package-owned V2 schema bytes. V1 has no dependency on this registry.
func SchemaV2(schemaID string) ([]byte, error) {
	name := ""
	switch schemaID {
	case SemanticSchemaV2:
		name = "schemas/describe-semantic.v2.schema.json"
	case ResponseSchemaV2:
		name = "schemas/describe-response.v2.schema.json"
	case InvocationSchemaV2:
		name = "schemas/describe-invocation.v2.schema.json"
	default:
		return nil, errors.New("unknown describe v2 schema")
	}
	raw, err := responseV2Schemas.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), raw...), nil
}
