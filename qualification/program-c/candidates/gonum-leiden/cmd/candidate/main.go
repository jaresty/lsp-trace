package main

import (
	"encoding/json"
	"fmt"
	q "lsp-trace/qualification/program-c/gonum-leiden"
	"os"
)

func main() {
	var req q.Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	switch os.Getenv("QUALIFICATION_TEST_MODE") {
	case "panic":
		panic("test panic")
	case "nonzero":
		os.Exit(23)
	}
	out, err := q.Run(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
