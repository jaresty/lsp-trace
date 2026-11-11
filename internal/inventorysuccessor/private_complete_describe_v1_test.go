package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"lsp-trace/internal/publication"
	"os"
	"testing"
	"time"
)

func TestCompleteDescribeContract(t *testing.T) {
	q := []byte("Explain Query")
	request, schema, e := NewPrivateDescribeRequestV1(q)
	if e != nil || string(schema) != PrivateDescribeResponseMarkerV1 {
		t.Fatal("constructor")
	}
	var r PrivateDescribeRequestV1
	if completeDescribeDecodeV1(request, &r, 128<<10) != nil || r.QuestionSHA256 != digest(q) || r.ResponseSchemaSHA256 != digest(schema) {
		t.Fatal("ASSERT_COMPLETE_CONTRACT")
	}
	for _, fault := range []string{"unknown", "duplicate", "trailing", "newline", "noncanonical", "oversize"} {
		t.Run(fault, func(t *testing.T) {
			raw := bytes.Clone(request)
			switch fault {
			case "unknown":
				raw = append([]byte("{\"HOST\":true,"), raw[1:]...)
			case "duplicate":
				raw = append([]byte("{\"schema\":\""+PrivateDescribeRequestSchemaV1+"\","), raw[1:]...)
			case "trailing":
				raw = append(raw, []byte("{}")...)
			case "newline":
				raw = append(raw, '\n')
			case "noncanonical":
				raw = append([]byte(" "), raw...)
			case "oversize":
				raw = bytes.Repeat([]byte("x"), (128<<10)+1)
			}
			var v PrivateDescribeRequestV1
			if completeDescribeDecodeV1(raw, &v, 128<<10) == nil {
				t.Fatal("ASSERT_COMPLETE_STRICT_CANONICAL", fault)
			}
		})
	}
	if out, e := CompleteDescribeV1(context.Background(), CompleteDescribeOperationV1{}, request); e == nil || out.ObjectID != "" {
		t.Fatal("zero operation granted")
	}
	if _, _, e := NewPrivateDescribeRequestV1(nil); e == nil {
		t.Fatal("empty question")
	}
}
func TestCompleteDescribeQuote(t *testing.T) {
	now := time.Now()
	c, e := completeDescribeChargeV1(now, now.Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	base, e := preparedDescribeChargeV1(preparedDescribeRecordMaxV1, now, now.Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	for i, n := range c.vector() {
		if n != 4*base.vector()[i] {
			t.Fatal("ASSERT_COMPLETE_CLOSED_QUOTE", i)
		}
	}
}
func TestCompleteDescribeReceiptPredicate(t *testing.T) {
	// Predicate-only values are NOT a publication or native admission witness.
	raw := []byte("bounded receipt predicate specimen")
	r := publication.BoundFileReceipt{FinalSelector: "selector", Digest: digest(raw), ByteLength: uint64(len(raw)), Mechanism: publication.BoundFileMechanism, NamespaceAtomic: true, VerificationStatus: "VERIFIED", CloseStatus: publication.CloseComplete, DirectorySyncStatus: publication.DirectorySyncComplete, CrashDurability: publication.DirectorySyncComplete}
	if !completeDescribeReceiptV1(&r, "selector", raw) || completeDescribeReceiptV1(nil, "selector", raw) {
		t.Fatal("ASSERT_COMPLETE_ALL_RECEIPT_STATUSES")
	}
	for _, fault := range []string{"selector", "digest", "length", "mechanism", "atomic", "verification", "close", "sync", "durability"} {
		t.Run(fault, func(t *testing.T) {
			v := r
			switch fault {
			case "selector":
				v.FinalSelector = "other"
			case "digest":
				v.Digest = "other"
			case "length":
				v.ByteLength++
			case "mechanism":
				v.Mechanism = "other"
			case "atomic":
				v.NamespaceAtomic = false
			case "verification":
				v.VerificationStatus = "other"
			case "close":
				v.CloseStatus = "other"
			case "sync":
				v.DirectorySyncStatus = "other"
			case "durability":
				v.CrashDurability = "other"
			}
			if completeDescribeReceiptV1(&v, "selector", raw) {
				t.Fatal("ASSERT_COMPLETE_ALL_RECEIPT_STATUSES", fault)
			}
		})
	}
}
func TestCompleteDescribeNoExecution(t *testing.T) {
	raw, e := os.ReadFile("private_complete_describe_v1.go")
	if e != nil {
		t.Fatal(e)
	}
	f, e := parser.ParseFile(token.NewFileSet(), "complete.go", raw, 0)
	if e != nil {
		t.Fatal(e)
	}
	allowed := map[string]bool{}
	for _, p := range []string{"bytes", "context", "encoding/json", "errors", "strings", "sync", "time", "lsp-trace/internal/describeworker", "lsp-trace/internal/publication"} {
		b, _ := json.Marshal(p)
		allowed[string(b)] = true
	}
	for _, i := range f.Imports {
		if !allowed[i.Path.Value] {
			t.Error("ASSERT_COMPLETE_NO_EXECUTION", i.Path.Value)
		}
	}
	for _, needle := range []string{".PrepareDescribeV1(", ".Run(", ".Start(", ".Finish(", ".CompleteHost(", ".ReserveMixedInputV2("} {
		if bytes.Contains(raw, []byte(needle)) {
			t.Error("ASSERT_COMPLETE_NO_EXECUTION", needle)
		}
	}
	fields := map[string]bool{}
	for _, n := range []string{"Root", "Open", "ExpectedHead", "Attempt", "Outputs", "DestinationIdentities", "Source", "Protected"} {
		fields[n] = true
	}
	ast.Inspect(f, func(n ast.Node) bool {
		s, ok := n.(*ast.TypeSpec)
		if !ok || s.Name.Name != "CompleteDescribeRequestV1" {
			return true
		}
		st, ok := s.Type.(*ast.StructType)
		if !ok {
			t.Fatal("request not struct")
		}
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				if !fields[name.Name] {
					t.Error("ASSERT_COMPLETE_NO_EXECUTION unexpected input", name.Name)
				}
			}
		}
		return false
	})
	if !t.Failed() {
		t.Log("ASSERT_COMPLETE_NO_EXECUTION: PASS (bounded direct source/import/control-shape guard)")
	}
}
