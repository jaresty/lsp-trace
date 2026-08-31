package source

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func FileURI(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	p := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" && !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String(), nil
}
func ResolveTarget(workspace, target string) (string, string, error) {
	w, err := filepath.Abs(workspace)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(w)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("workspace is not a directory: %s", w)
	}
	p := target
	if !filepath.IsAbs(p) {
		p = filepath.Join(w, p)
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(w, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("target is outside workspace: %s", p)
	}
	if _, err = os.Stat(p); err != nil {
		return "", "", err
	}
	wu, _ := FileURI(w)
	tu, _ := FileURI(p)
	return wu, tu, nil
}
func LanguageID(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".cs":
		return "csharp"
	case ".ex", ".exs":
		return "elixir"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	default:
		return "plaintext"
	}
}
