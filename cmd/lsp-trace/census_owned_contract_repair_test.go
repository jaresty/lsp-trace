//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Snapshot only disposable test roots, including directory creation and file
// content/mtime/mode. It is an audit assertion, not caller coordination.
func repairTree(t *testing.T, roots ...string) string {
	t.Helper()
	var rows []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			s, e := d.Info()
			if e != nil {
				return e
			}
			var b []byte
			if d.Type().IsRegular() {
				b, e = os.ReadFile(p)
				if e != nil {
					return e
				}
			}
			rows = append(rows, fmt.Sprintf("%s %v %d %x", p, s.Mode(), s.ModTime().UnixNano(), sha256.Sum256(b)))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return strings.Join(rows, "\n")
}
func TestCensusOwnedEmptyStoreRepair(t *testing.T) {
	for _, mode := range []string{"default", "environment", "missing-default"} {
		for _, op := range []string{"status", "acquire", "seal"} {
			t.Run(mode+"/"+op, func(t *testing.T) {
				c := newPublicCLI(t)
				var v publicOwnerReply
				if mode == "default" {
					if json.Unmarshal(c.call([]string{"lsp-trace", "census", "."}, nil, true), &v) != nil {
						t.Fatal("start")
					}
				}
				if mode == "environment" {
					v = c.start()
				}
				if mode != "missing-default" && op == "seal" {
					v = c.acquire(c.acquire(v))
				}
				if mode == "missing-default" {
					v.RunID = strings.Repeat("0", 64)
					v.Head = strings.Repeat("1", 64)
				}
				t.Setenv("HOME", c.home)
				t.Setenv(censusDurableStoreEnv, "")
				if mode == "environment" {
					t.Setenv(censusDurableStoreEnv, v.Store)
				}
				before := repairTree(t, c.home, c.store)
				args := []string{op, v.RunID}
				if op != "status" {
					args = append(args, v.Head)
				}
				args = append(args, "--store", "")
				var out, diag bytes.Buffer
				constructors := 0
				code := runCensusOwnerOperationWithFactory(args, bytes.NewReader(nil), &out, &diag, func(ai.InventoryNativeBatchV1, time.Time) (sessionruntime.PrivateDiscoveryConstruction, error) {
					constructors++
					return sessionruntime.PrivateDiscoveryConstruction{}, errors.New("constructor must not be reached")
				})
				after := repairTree(t, c.home, c.store)
				if code != 1 || out.Len() != 0 || constructors != 0 || before != after {
					t.Errorf("EMPTY_EXPLICIT_STORE_EFFECT code=%d constructors=%d tree_changed=%t out=%s diagnostic=%s", code, constructors, before != after, out.String(), diag.String())
				}
				t.Logf("EMPTY_STORE_AUDIT mode=%s op=%s code=%d constructors=%d unchanged=%t", mode, op, code, constructors, before == after)
			})
		}
	}
}
func TestCensusOwnedRecoveryAliasRepair(t *testing.T) {
	c := newPublicCLI(t)
	v := c.acquire(c.acquire(c.start()))
	task := c.task(v.Operations["seal"], nil)
	for i := 0; i < 2; i++ {
		if task.Phase != "Describe" {
			t.Fatal("expected Describe")
		}
		c.input(task)
		saved := task.Contract.Operations
		oldID := task.TaskID
		c.call(saved["submit"], []byte(task.Contract.Example), true) // deliberately discard successor response
		current := c.task(saved["current_task"], nil)
		before := repairTree(t, v.Store)
		t.Logf("RECOVERY_ALIAS_AFTER_DESCRIBE stage=%d predecessor=%s", i+1, oldID)
		alias := c.task(saved["task"], nil)
		if current.TaskID == oldID || alias.TaskID != current.TaskID {
			t.Fatal("RECOVERY_ALIAS_NOT_CURRENT")
		}
		if len(saved["exact_task"]) == 0 {
			t.Fatal("EXACT_TASK_MISSING")
		}
		c.call(saved["exact_task"], nil, false)
		if repairTree(t, v.Store) != before {
			t.Fatal("RECOVERY_OBSERVATION_MUTATED_STORE")
		}
		task = alias
	}
}
func TestCensusOwnedDecoratedAliasesRepair(t *testing.T) {
	for _, version := range []string{"v1", "v2-legacy", "v2-api", "v3"} {
		for _, status := range []string{"PENDING", "PUBLISHED", "EXPIRED", "UNCERTAIN_SPENT"} {
			t.Run(version+"/"+status, func(t *testing.T) {
				schema := strings.Split(version, "-")[0]
				task := ai.DurableTask{Schema: "lsp-trace.durable-inventory-task." + schema, RunID: "run", TaskID: "task", Role: "interpreter", Status: status}
				task.Contract.Rules = "run current_task (task is its alias) to observe the current run task/status, including a successor. exact_task is only a current-ID check and becomes stale after advancement."
				current := []string{"lsp-trace", "census", "durable", "task", "run"}
				exact := append(append([]string(nil), current...), "task")
				if version == "v1" || version == "v2-legacy" {
					task.Contract.Operations = map[string][]string{"task": append([]string(nil), current...), "current_task": append([]string(nil), current...), "exact_task": exact}
				}
				result := censusOwnerTask(task, "/controlled/store")
				want := append(current, "--store", "/controlled/store")
				wantExact := append(exact, "--store", "/controlled/store")
				if !reflect.DeepEqual(result.Contract.Operations["task"], want) || !reflect.DeepEqual(result.Contract.Operations["current_task"], want) || !reflect.DeepEqual(result.Contract.Operations["exact_task"], wantExact) || result.Contract.Rules != task.Contract.Rules {
					t.Fatal("DECORATED_ALIAS_CONTRACT", result.Contract)
				}
			})
		}
	}
}
