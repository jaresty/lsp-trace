package vcssymbolsidecar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-trace/internal/jsonrpc"
	"lsp-trace/internal/lsp"
)

const maxHistoricalFileBytes = 8 * 1024 * 1024

type LSPProcessProvider struct {
	Command        string
	Args           []string
	Env            []string
	RequestTimeout time.Duration
}
type lspProcessClient struct {
	root      string
	timeout   time.Duration
	client    *lsp.Client
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stderr    *limitedWriter
	closeOnce sync.Once
	closeErr  error
}

func (p LSPProcessProvider) Start(parent context.Context, root string) (SymbolClient, error) {
	if !filepath.IsAbs(p.Command) || !filepath.IsAbs(root) {
		return nil, errors.New("historical LSP requires absolute executable and workspace")
	}
	timeout := p.RequestTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cmd := exec.CommandContext(parent, p.Command, p.Args...)
	cmd.Dir = root
	if p.Env != nil {
		cmd.Env = append([]string(nil), p.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &limitedWriter{limit: 64 * 1024}
	cmd.Stderr = stderr
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	client := lsp.NewClient(jsonrpc.New(stdout, stdin))
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	rootURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String()
	if err = client.Initialize(ctx, rootURI); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("initialize historical LSP: %w", err)
	}
	if !client.SupportsDocumentSymbols() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, errors.New("historical LSP lacks documentSymbol capability")
	}
	return &lspProcessClient{root: root, timeout: timeout, client: client, cmd: cmd, stdin: stdin, stderr: stderr}, nil
}
func (c *lspProcessClient) Symbols(parent context.Context, root, path, language string) ([]Symbol, error) {
	if root != c.root || path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("invalid historical document path")
	}
	full := filepath.Join(root, path)
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("historical document escapes workspace")
	}
	info, err := os.Stat(real)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxHistoricalFileBytes {
		return nil, errors.New("historical document is not an admitted regular file")
	}
	raw, err := os.ReadFile(real)
	if err != nil {
		return nil, err
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(real)}).String()
	if err = c.client.DidOpen(uri, language, string(raw)); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	defer cancel()
	rows, err := c.client.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		return nil, err
	}
	out := []Symbol{}
	flattenHistoricalSymbols(&out, "", filepath.ToSlash(path), rows)
	return out, nil
}
func flattenHistoricalSymbols(out *[]Symbol, parent, path string, rows []lsp.DocumentSymbol) {
	for _, row := range rows {
		name := row.Name
		if parent != "" {
			name = parent + "::" + name
		}
		*out = append(*out, Symbol{Path: path, Name: name, Kind: row.Kind, Range: Range{Start: Position{Line: int(row.Range.Start.Line), Character: int(row.Range.Start.Character)}, End: Position{Line: int(row.Range.End.Line), Character: int(row.Range.End.Character)}}})
		flattenHistoricalSymbols(out, name, path, row.Children)
	}
}
func (c *lspProcessClient) Close() error {
	c.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
		_ = c.client.Shutdown(ctx)
		_ = c.stdin.Close()
		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()
		select {
		case err := <-done:
			c.closeErr = err
		case <-ctx.Done():
			_ = c.cmd.Process.Kill()
			c.closeErr = <-done
		}
	})
	return c.closeErr
}
