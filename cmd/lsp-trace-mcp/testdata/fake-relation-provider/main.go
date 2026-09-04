// Command fake-relation-provider is a deterministic test-only provider process.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if marker := os.Getenv("LSP_TRACE_PROVIDER_START_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte("started\n"), 0o600)
	}
	mode := os.Getenv("LSP_TRACE_PROVIDER_MODE")
	body, err := readFrame(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	if exitMarker := os.Getenv("LSP_TRACE_PROVIDER_EXIT_MARKER"); exitMarker != "" {
		defer os.WriteFile(exitMarker, []byte("exited\n"), 0o600) //nolint:errcheck
	}
	switch mode {
	case "hang":
		for {
			time.Sleep(time.Hour)
		}
	case "malformed-frame":
		_, _ = io.WriteString(os.Stdout, "Content-Length: nope\r\n\r\n{}")
	case "malformed-payload":
		writeFrame([]byte(`{"provider_id":`))
	case "oversized":
		_, _ = io.WriteString(os.Stdout, "Content-Length: 1048576\r\n\r\n{}")
	case "no-item":
		writeFrame([]byte(`{"provider_id":"fake@1","terminal":"PREPARE_RETURNED_NO_ITEM","complete":false,"truncated":false,"bounds":{"max_relations":8,"max_source_bytes":4096,"max_operations":4,"timeout_ms":1000,"protocol_messages":1,"cancelled":false},"relations":[]}`))
	default:
		var request any
		if json.Unmarshal(body, &request) != nil {
			os.Exit(2)
		}
		writeFrame([]byte(`{"provider_id":"fake@1","terminal":"COMPLETE_WITHIN_BOUNDS","complete":true,"truncated":false,"bounds":{"max_relations":8,"max_source_bytes":4096,"max_operations":4,"timeout_ms":1000,"protocol_messages":1,"cancelled":false},"relations":[{"relation_id":"r-fake","kind":"PASSES_CALLBACK"}]}`))
	}
}

func readFrame(r io.Reader) ([]byte, error) {
	br := bufio.NewReader(r)
	header, err := br.ReadString('\n')
	if err != nil || !strings.HasSuffix(header, "\r\n") || !strings.HasPrefix(header, "Content-Length: ") {
		return nil, fmt.Errorf("invalid header")
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "Content-Length: ")))
	if err != nil || n < 0 {
		return nil, fmt.Errorf("invalid length")
	}
	if separator, err := br.ReadString('\n'); err != nil || separator != "\r\n" {
		return nil, fmt.Errorf("invalid separator")
	}
	body := make([]byte, n)
	_, err = io.ReadFull(br, body)
	return body, err
}

func writeFrame(body []byte) {
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n", len(body))
	_, _ = os.Stdout.Write(body)
	_ = os.Stdout.Sync()
	time.Sleep(time.Millisecond)
}
