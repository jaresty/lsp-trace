package programcrepresentative_test

import (
	"testing"

	"lsp-trace/internal/programcrepresentative"
)

var mechanicsV2BenchmarkSink []programcrepresentative.MechanicsV2

func BenchmarkMechanicsV2PreparedBatch(b *testing.B) {
	base := mechanicsV2Input(b, 32)
	common := preparedMechanicsV2Common(base)
	contexts := preparedMechanicsV2Contexts(base, 58)

	b.Run("old-repeated", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			out := make([]programcrepresentative.MechanicsV2, len(contexts))
			for j, context := range contexts {
				item, err := programcrepresentative.NewMechanicsV2(preparedMechanicsV2Legacy(common, context))
				if err != nil {
					b.Fatal(err)
				}
				out[j] = item
			}
			mechanicsV2BenchmarkSink = out
		}
	})

	b.Run("prepare-batch", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			prepared, err := programcrepresentative.PrepareMechanicsV2(common)
			if err != nil {
				b.Fatal(err)
			}
			out, err := programcrepresentative.NewPreparedMechanicsBatchV2(prepared, contexts)
			if err != nil {
				b.Fatal(err)
			}
			mechanicsV2BenchmarkSink = out
		}
	})
}
