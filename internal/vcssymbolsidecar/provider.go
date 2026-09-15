package vcssymbolsidecar

import (
	"context"
	"errors"
	"sort"
	"strings"
)

type RevisionRequest struct {
	Repository string
	Revision   string
	Paths      []string
	LanguageID string
}
type HistoricalWorkspace struct {
	Root     string
	Revision string
	Close    func() error
}
type WorkspaceProvider interface {
	Open(context.Context, string, string) (HistoricalWorkspace, error)
}
type SymbolClient interface {
	Symbols(context.Context, string, string, string) ([]Symbol, error)
	Close() error
}
type SessionProvider interface {
	Start(context.Context, string) (SymbolClient, error)
}
type FileOutcome struct {
	Path    string   `json:"path"`
	Status  string   `json:"status"`
	Error   string   `json:"error,omitempty"`
	Symbols []Symbol `json:"symbols"`
}
type RevisionSymbols struct {
	Revision string        `json:"revision"`
	Outcomes []FileOutcome `json:"outcomes"`
}

func AcquireRevision(ctx context.Context, req RevisionRequest, workspaces WorkspaceProvider, sessions SessionProvider) (result RevisionSymbols, err error) {
	if workspaces == nil || sessions == nil || req.Repository == "" || req.Revision == "" || req.LanguageID == "" || len(req.Paths) == 0 || len(req.Paths) > 128 {
		return result, errors.New("invalid historical symbol request")
	}
	paths := append([]string(nil), req.Paths...)
	sort.Strings(paths)
	for i, p := range paths {
		if strings.TrimSpace(p) == "" || (i > 0 && paths[i-1] == p) {
			return result, errors.New("invalid or duplicate historical path")
		}
	}
	workspace, e := workspaces.Open(ctx, req.Repository, req.Revision)
	if e != nil {
		return result, e
	}
	if workspace.Close == nil || workspace.Root == "" || !validSymbolCommit(workspace.Revision) {
		if workspace.Close != nil {
			_ = workspace.Close()
		}
		return result, errors.New("workspace omitted exact revision identity")
	}
	defer func() {
		if e := workspace.Close(); err == nil && e != nil {
			err = e
		}
	}()
	client, e := sessions.Start(ctx, workspace.Root)
	if e != nil {
		return result, e
	}
	defer func() {
		if e := client.Close(); err == nil && e != nil {
			err = e
		}
	}()
	result.Revision = workspace.Revision
	result.Outcomes = make([]FileOutcome, 0, len(paths))
	for _, path := range paths {
		symbols, e := client.Symbols(ctx, workspace.Root, path, req.LanguageID)
		row := FileOutcome{Path: path, Symbols: []Symbol{}}
		if e != nil {
			row.Status = "FAILED"
			row.Error = e.Error()
		} else if len(symbols) == 0 {
			row.Status = "EMPTY"
		} else {
			row.Status = "COMPLETE"
			row.Symbols = symbols
		}
		result.Outcomes = append(result.Outcomes, row)
	}
	return result, nil
}
