package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/publication"
)

type bootstrap struct { Continuation struct { PublicationRoot string `json:"publication_root"`; MaxObjectBytes int64 `json:"max_object_bytes"` } `json:"continuation"` }
type wire struct { SchemaVersion string `json:"schema_version"`; CatalogSelector string `json:"catalog_selector"`; CheckpointSelector string `json:"checkpoint_selector"`; CompositeSelector string `json:"composite_selector"` }

func main() {
	if len(os.Args) != 2 { panic("config operand required") }
	raw, err := os.ReadFile(os.Args[1]); if err != nil { panic("config read") }
	var cfg bootstrap; if json.Unmarshal(raw, &cfg) != nil { panic("config decode") }
	objects := filepath.Join(cfg.Continuation.PublicationRoot, "continuations", "objects", "sha256")
	entries, _ := os.ReadDir(objects)
	checkpoint := ""
	for _, e := range entries {
		if e.IsDir() { continue }
		b, readErr := os.ReadFile(filepath.Join(objects, e.Name())); if readErr != nil { continue }
		c, parseErr := censuscontinuation.ParseCheckpoint(b)
		if parseErr == nil {
			fmt.Printf("OBJECT checkpoint stage=%s status=%s bytes=%d id=%s\n", c.Stage(), c.Status(), len(b), e.Name()[:16])
			if c.Stage() == censuscontinuation.StageRequestsRendered && c.Status() == censuscontinuation.StatusRunning { checkpoint = "sha256:"+e.Name() }
		} else { fmt.Printf("OBJECT other bytes=%d id=%s\n", len(b), e.Name()[:16]) }
	}
	if checkpoint == "" { fmt.Println("PAUSED_CHECKPOINT_ABSENT"); return }
	input := continuationhost.DescriptorInput{CatalogSelector: checkpoint, CheckpointSelector: checkpoint, CompositeSelector: checkpoint}
	encoded, _ := json.Marshal(wire{continuationhost.DescriptorSchemaVersion, checkpoint, checkpoint, checkpoint})
	sum := sha256.Sum256(encoded); private := fmt.Sprintf("continuations/catalogs/%x.json", sum[:])
	before, readErr := os.ReadFile(filepath.Join(cfg.Continuation.PublicationRoot, filepath.FromSlash(private)))
	fmt.Printf("CHECKPOINT_PRESENT canonical=%t descriptor_bytes=%d target_preexisting=%t target_equal=%t target_digest=%s\n", continuationhost.IsCanonicalSelector(checkpoint), len(encoded), readErr==nil, readErr==nil && string(before)==string(encoded), hex.EncodeToString(sum[:8]))
	root, err := publication.OpenRoot(cfg.Continuation.PublicationRoot); if err != nil { fmt.Println("ROOT_OPEN_FAILED"); return }; defer root.Close()
	store, err := continuationhost.NewStore(root, cfg.Continuation.MaxObjectBytes); if err != nil { fmt.Println("STORE_OPEN_FAILED"); return }
	pub := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: private, Bytes: encoded, ArtifactSchemaID: continuationhost.DescriptorSchemaVersion})
	if pub.Failure != nil { fmt.Printf("DIRECT_PUBLICATION stage=%s code=%s atomic=%t cleanup=%t\n", pub.Failure.Stage, pub.Failure.Code, pub.Failure.AtomicRename, pub.Failure.Cleanup) } else { fmt.Printf("DIRECT_PUBLICATION success receipt=%t\n", pub.Receipt != nil) }
	selector, err := store.PublishDescriptor(context.Background(), input)
	fmt.Printf("STORE_PUBLICATION success=%t public_selector_valid=%t error_class=%s\n", err==nil, continuationhost.IsPublicDescriptorSelector(selector), classify(err))
}
func classify(err error) string { if err==nil{return "NONE"}; s:=err.Error(); switch {case strings.Contains(s,"collision"):return "COLLISION";case strings.Contains(s,"publication failed"):return "PUBLICATION_FAILED";case strings.Contains(s,"invalid descriptor"):return "INVALID_DESCRIPTOR";default:return "OTHER"} }
