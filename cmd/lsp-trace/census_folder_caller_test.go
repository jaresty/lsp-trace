//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/sessionruntime"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	ai "lsp-trace/internal/applicationinventory"
	dw "lsp-trace/internal/describeworker"
	product "lsp-trace/internal/privatehostproductv3"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
)

// Invoked by Go (or optional adapter) tests with a retained child process handle.
// The process uses the actual normal-folder entry; only the provider wire is fake.
func TestCensusFolderManagedHostProcess(t *testing.T) {
	if os.Getenv("CENSUS_MANAGED_PROCESS_FIXTURE") != "1" {
		t.Skip("owned external caller fixture only")
	}
	workspace, store := folderInventoryTemp(t), folderInventoryTemp(t)
	for name, body := range map[string]string{"go.mod": "module controlled.example/checkout\n\ngo 1.26\n", "checkout.go": "package checkout\nfunc Checkout() int { return price() }\n", "price.go": "package checkout\nfunc price() int { return 7 }\n"} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	wire := newFolderInventoryWire(t, workspace)
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(string) (string, error) { return "/controlled/checkout-provider", nil }
	deps.newStarter = func() (sessionruntime.Starter, error) { return terminalStarter{child: wire}, nil }
	refusal := os.Getenv("CENSUS_MANAGED_PROCESS_REFUSAL")
	if refusal == "missing" {
		t.Setenv(censusCallerFDEnv, "")
	}
	var out, diag bytes.Buffer
	args := []string{workspace, "--publication-root", store, "--machine"}
	if refusal == "withheld" {
		args = append(args, "--timeout", "3s", "--request-timeout", "3s")
	}
	code := runCensusWithDependencies(args, &out, &diag, deps)
	if refusal != "" {
		if code == 0 || out.Len() != 0 || !bytes.Contains(diag.Bytes(), []byte("ASSEMBLY_FAILED")) {
			t.Fatal("refusal leaked success", code, out.String(), diag.String())
		}
		if refusal == "missing" && !bytes.Contains(diag.Bytes(), []byte("CALLER_AGENT_UNAVAILABLE")) {
			t.Fatal("missing host was not actionable", diag.String())
		}
		paths, _ := filepath.Glob(filepath.Join(store, "run-*", "application", "terminal-*.json"))
		for _, path := range paths {
			raw, _ := os.ReadFile(path)
			var terminal applicationInventoryTerminalRecordV3
			if json.Unmarshal(raw, &terminal) != nil || terminal.Status == "PROVISIONAL" || terminal.InventoryHead != "" {
				t.Fatal("refusal published success", string(raw))
			}
		}
		t.Log("ASSERT_GO_MANAGED_REFUSAL: PASS", refusal)
		return
	}
	if code != 0 {
		paths, _ := filepath.Glob(filepath.Join(store, "run-*", "application", "terminal-*.json"))
		for _, path := range paths {
			raw, _ := os.ReadFile(path)
			t.Log("controlled terminal", string(raw))
		}
		t.Fatal("normal Go caller process", code, diag.String())
	}
	if err := folderInventorySelectedOriginals(store, wire.snapshot(), true); err != nil {
		t.Fatal(err)
	}
	requests, _ := filepath.Glob(filepath.Join(store, "run-*", "inventory-request.json"))
	if len(requests) != 1 {
		t.Fatal("request missing", requests)
	}
	raw, err := os.ReadFile(requests[0])
	if err != nil {
		t.Fatal(err)
	}
	q, err := product.DecodeInventoryRequestV3(raw)
	if err != nil {
		t.Fatal(err)
	}
	var terminal applicationInventoryTerminalRecordV3
	if err := json.Unmarshal(out.Bytes(), &terminal); err != nil {
		t.Fatal(err)
	}
	verifyFolderCallerPublication(t, folderCallerResult{request: q, terminal: terminal, descriptions: 2})
	t.Log("ASSERT_GO_MANAGED_PROCESS: PASS controller publication and selected originals")
}

type folderCallerResult struct {
	request      product.InventoryRequestV3
	terminal     applicationInventoryTerminalRecordV3
	descriptions int
	err          error
}

// An explicit tagged semantic fixture on the production local IPC, not a native
// callback or production canned output. No model or agent is exercised here.
func folderCallerFixture(t *testing.T, subagents bool, refuse bool) <-chan folderCallerResult {
	t.Helper()
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	child := os.NewFile(uintptr(pair[0]), "controlled-caller-child")
	parent := os.NewFile(uintptr(pair[1]), "controlled-caller-parent")
	conn, err := net.FileConn(parent)
	parent.Close()
	if err != nil {
		child.Close()
		t.Fatal(err)
	}
	t.Setenv(censusCallerFDEnv, strconv.Itoa(int(child.Fd())))
	done := make(chan folderCallerResult, 1)
	t.Cleanup(func() { conn.Close(); child.Close() })
	go func() { done <- serveFolderCallerFixture(conn, subagents, refuse, "") }()
	return done
}

func serveFolderCallerFixture(conn net.Conn, subagents, refuse bool, fault string, onTask ...func()) (result folderCallerResult) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(20 * time.Second))
	actor, reviewer := "scripted-main-context", "scripted-main-context"
	if subagents {
		actor, reviewer = "scripted-interpreter", "scripted-reviewer"
	}
	pin := digestCatalog("controlled caller fixture; not model provenance")
	hello := censusCallerHello{Protocol: censusCallerProtocol, Subagents: subagents, Interpreter: actor, Reviewer: reviewer, Pins: dw.HostPinsV2{WorkerSHA256: pin, ModelSHA256: pin, GrammarSHA256: pin}}
	if result.err = writeCensusCaller(conn, hello, 4096); result.err != nil {
		return
	}
	proposed := ""
	last := 0
	for n := 0; n < 18; n++ {
		var event censusCallerMessage
		if result.err = readCensusCaller(conn, &event, (1<<20)+4096); result.err != nil {
			return
		}
		if event.Protocol != censusCallerProtocol {
			result.err = errors.New("protocol")
			return
		}
		switch event.Event {
		case "ACTIVATION_READY":
			raw, e := os.ReadFile(event.RequestPath)
			if e != nil {
				result.err = e
				return
			}
			result.request, e = product.DecodeInventoryRequestV3(raw)
			if e != nil {
				result.err = e
				return
			}
			expected := cat.ReviewModeSameThreadV3
			if subagents {
				expected = cat.ReviewModeSubagentsV3
			}
			if event.ReviewMode != expected || result.request.Inventory.ReviewMode != expected {
				result.err = errors.New("mode not bound before activation")
				return
			}
		case "NEEDS_HOST":
			f := event.Frame
			if f == nil || f.Sequence <= last || f.RunID != event.RunID {
				result.err = errors.New("frame binding")
				return
			}
			last = f.Sequence
			if refuse || fault == "disconnect" {
				conn.Close()
				result.err = nil
				return
			}
			if fault == "cancel" && len(onTask) == 1 {
				onTask[0]()
				fault = "withheld"
			}
			if fault == "withheld" {
				var ignored censusCallerMessage
				result.err = readCensusCaller(conn, &ignored, 1<<20)
				return
			}
			if fault == "delayed" {
				time.Sleep(1100 * time.Millisecond)
				fault = ""
			}
			var payload []byte
			if f.Operation == "Describe" {
				if f.Role != "interpreter" || f.LaunchID != actor {
					result.err = errors.New("describe role")
					return
				}
				result.descriptions++
				payload = dw.EncodeAdapterResultV2(`{"verdict":"COMPLETE","target_role":"participates in controlled checkout","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"supplies a checkout relation","consumer_relative":false},"boundary_contribution":"two-file fixture","limitations":["scripted fixture, not live model or application completeness"]}`)
			} else {
				var catalog cat.NativeCatalogWireV3
				if result.err = cat.DecodeNativeV3(f.Payload, &catalog); result.err != nil {
					return
				}
				if len(catalog.Evidence) != result.descriptions {
					result.err = fmt.Errorf("description census got=%d served=%d catalog=%s", len(catalog.Evidence), result.descriptions, f.Payload)
					return
				}
				groups := []cat.NativeGroupV3{}
				members := []string{}
				// Leave one description unresolved, rather than force a total grouping.
				for i, e := range catalog.Evidence {
					if i+1 < len(catalog.Evidence) {
						members = append(members, e.ID)
					}
				}
				if len(members) > 0 {
					groups = append(groups, cat.NativeGroupV3{ID: "controlled-checkout", Label: "checkout candidate", Reason: "scripted grouping", Members: members})
				}
				affected := []string{}
				decisionActor := actor
				if f.Operation == "Reviewer" {
					if f.Role != "reviewer" || f.LaunchID != reviewer || proposed == "" || catalog.Predecessor != proposed {
						result.err = errors.New("reviewer did not receive proposal")
						return
					}
					decisionActor = reviewer
					groups = catalog.Groups
					if len(groups) > 0 {
						affected = append(affected, groups[0].ID)
						groups[0].Label = "separately reviewed checkout candidate"
						groups[0].Reason = "separate scripted Reviewer exchange; no context authentication"
					}
				} else if f.Operation == "GroupDecision" {
					proposed = catalog.Revision
				} else {
					result.err = errors.New("unexpected operation")
					return
				}
				payload, result.err = json.Marshal(cat.NativeDecisionV3{Predecessor: catalog.Revision, Actor: decisionActor, Reason: "controlled contract reply, not live agents", Affected: affected, Groups: groups})
				if result.err != nil {
					return
				}
			}
			reply := censusCallerReply{Protocol: censusCallerProtocol, RunID: f.RunID, Role: f.Role, RequestDigest: f.RequestDigest, Sequence: f.Sequence, Payload: payload}
			if fault == "stale" {
				reply.Sequence++
			}
			if fault == "mismatch" {
				reply.RequestDigest = digestCatalog("wrong request")
			}
			if fault == "malformed" {
				result.err = writeCensusCaller(conn, map[string]string{"Unknown": "rejected"}, 96<<10)
			} else {
				result.err = writeCensusCaller(conn, reply, 96<<10)
			}
			if result.err != nil {
				return
			}
		case "TERMINAL":
			if event.Terminal == nil {
				result.err = errors.New("missing terminal")
				return
			}
			result.terminal = *event.Terminal
			return
		default:
			result.err = fmt.Errorf("unknown event %s", event.Event)
			return
		}
	}
	result.err = errors.New("fixture service bound")
	return
}

func verifyFolderCallerPublication(t *testing.T, result folderCallerResult) {
	t.Helper()
	if result.err != nil {
		t.Fatal("ASSERT_FOLDER_CALLER_SERVICE: FAIL", result.err)
	}
	q := result.request
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if q.Configuration.ProducerExecutableSHA256 != censusInventoryDigest(executable) || q.Configuration.ProducerExecutableSHA256 == q.Inventory.Pins.WorkerSHA256 {
		t.Fatal("ASSERT_FOLDER_PRODUCER: wrong executable binding")
	}
	if q.Inventory.HostWindowMS <= 1000 || q.Inventory.HostWindowMS > 30000 {
		t.Fatal("ASSERT_FOLDER_HOST_WINDOW: not allowance-derived", q.Inventory.HostWindowMS)
	}
	bootstrap, err := os.Stat(filepath.Join(q.Configuration.Store, "bootstrap"))
	if err != nil || bootstrap.Size() != 2 || q.Configuration.Grant.BootstrapAttempts != 2 {
		t.Fatal("ASSERT_FOLDER_FRESH_GRANT: expected create/run admissions only", err)
	}
	t.Log("ASSERT_FOLDER_FRESH_GRANT: PASS two admissions, no renewed bootstrap allowance")
	t.Log("ASSERT_FOLDER_PRODUCER: PASS measured Go executable differs from caller declaration")
	var publication product.InventoryPublicationV3
	if err := product.ReadApplicationRecord(q.Configuration.Store, result.terminal.InventoryHead, &publication); err != nil {
		t.Fatal("ASSERT_FOLDER_PUBLICATION_REOPEN: FAIL", err)
	}
	if err := product.VerifyInventoryPublicationV3(publication, q); err != nil {
		t.Fatal("ASSERT_FOLDER_PUBLICATION_REOPEN: FAIL", err)
	}
	path := filepath.Join(publication.Directory, "report.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report ai.Report
	if cat.DecodeNativeV3(raw, &report) != nil || !report.Published || report.SuccessfulDescriptions != result.descriptions || result.descriptions != len(q.Inventory.Manifest.Leads) || len(report.Census) != len(report.Dispositions) || len(report.Census) != result.descriptions || report.WithoutMembership != 1 || report.WithMembership != result.descriptions-1 || report.Stages["review"].Successful != 1 {
		t.Fatal("ASSERT_FOLDER_CONSERVATION: FAIL", report, result.descriptions)
	}
	for _, d := range report.Dispositions {
		if d.Status != "DESCRIBED_WITH_MEMBERSHIP" && d.Status != "DESCRIBED_UNMATCHED" {
			t.Fatal("ASSERT_FOLDER_CONSERVATION: FAIL", d)
		}
	}
	t.Log("ASSERT_FOLDER_CONSERVATION: PASS grouped plus unresolved equals admitted census")
	for _, bad := range []string{"withheld", "corrupt"} {
		if bad == "withheld" {
			err = os.Remove(path)
		} else {
			err = os.WriteFile(path, []byte("{}\n"), 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
		rejected := product.VerifyInventoryPublicationV3(publication, q) != nil
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if !rejected {
			t.Fatal("ASSERT_FOLDER_PUBLICATION_REOPEN: FAIL accepted", bad)
		}
		t.Log("ASSERT_FOLDER_PUBLICATION_REOPEN: rejected", bad, "report")
	}
	if err = product.VerifyInventoryPublicationV3(publication, q); err != nil {
		t.Fatal("ASSERT_FOLDER_PUBLICATION_REOPEN: FAIL restored", err)
	}
	t.Log("ASSERT_FOLDER_PUBLICATION_REOPEN: PASS exact restored report")
}
