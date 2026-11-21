// Command genschema publishes the diagnostic-local V3 shape. Run through
// go generate ./internal/capacitydiagnostic from the repository root.
package main

import (
	"log"
	"os"

	"lsp-trace/internal/capacitydiagnostic"
)

func main() {
	raw, err := capacitydiagnostic.CapacityDiagnosticSchema()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("../schema/schemas/"+capacitydiagnostic.CapacityDiagnosticSchemaVersion+".schema.json", raw, 0644); err != nil {
		log.Fatal(err)
	}
}
