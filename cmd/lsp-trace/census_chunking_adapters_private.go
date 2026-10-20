package main

import (
	"context"
	"errors"
	"net/url"
	"path"
	"strings"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/censuschunkingruntime"
	"lsp-trace/internal/publication"
)

type privateCensusCoordinatorStore = censuschunkingruntime.CoordinatorStore

func newPrivateCensusCoordinatorStore(root *publication.Root) (*privateCensusCoordinatorStore, error) {
	return censuschunkingruntime.NewCoordinatorStore(root)
}

func privateCensusObjectSelector(raw []byte) string {
	return censuschunkingruntime.ObjectSelector(raw)
}

func validPrivateCensusObjectSelector(selector string) bool {
	return censuschunkingruntime.ValidObjectSelector(selector)
}

type privateCensusHeadStore = censuschunkingruntime.HeadStore

func newPrivateCensusHeadStore(root *publication.Root) (*privateCensusHeadStore, error) {
	return censuschunkingruntime.NewHeadStore(root)
}

func privateCensusHeadSelector(key string) (string, error) {
	return censuschunkingruntime.HeadSelector(key)
}

type privateHeldCensusSession struct {
	expected censuschunking.ExpectedSession
	core     censusacquisition.Core
}

func newPrivateHeldCensusSession(expected censuschunking.ExpectedSession, core censusacquisition.Core) (*privateHeldCensusSession, error) {
	if strings.TrimSpace(expected.SessionID) == "" || expected.Generation == 0 || !canonicalPrivateWorkspaceURI(expected.WorkspaceURI) {
		return nil, errors.New("invalid held census session identity")
	}
	if core.Discoverer == nil || core.Acquirer == nil {
		return nil, errors.New("invalid held census session core")
	}
	return &privateHeldCensusSession{expected: expected, core: core}, nil
}

func canonicalPrivateWorkspaceURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" || u.Path == "" || !strings.HasPrefix(u.Path, "/") || path.Clean(u.Path) != u.Path {
		return false
	}
	canonical := (&url.URL{Scheme: "file", Path: u.Path}).String()
	return canonical == raw
}

func (s *privateHeldCensusSession) WithReadySession(ctx context.Context, expected censuschunking.ExpectedSession, callback func(censusacquisition.Core) error) error {
	if s == nil || ctx == nil || callback == nil {
		return errors.New("invalid held census session")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if expected != s.expected {
		return errors.New("held census session identity mismatch")
	}
	return callback(s.core)
}
