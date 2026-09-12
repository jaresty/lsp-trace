package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"lsp-trace/internal/programc"
)

func runProgramCInstability(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("program-c instability", flag.ContinueOnError)
	fs.SetOutput(stderr)
	seedText := fs.String("seeds", "", "comma-separated distinct deterministic seeds")
	algorithmVersion := fs.String("algorithm-version", "", "exact implementation version")
	parametersSHA256 := fs.String("parameters-sha256", "", "canonical parameter digest")
	resourcePolicySHA256 := fs.String("resource-policy-sha256", "", "resource policy digest")
	if fs.Parse(args) != nil || fs.NArg() != 1 || *seedText == "" || *algorithmVersion == "" || *parametersSHA256 == "" || *resourcePolicySHA256 == "" {
		fmt.Fprintln(stderr, "usage: lsp-trace program-c instability --seeds N,N --algorithm-version VERSION --parameters-sha256 DIGEST --resource-policy-sha256 DIGEST PATH|-")
		return 1
	}
	var seeds []uint64
	for _, value := range strings.Split(*seedText, ",") {
		seed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			fmt.Fprintln(stderr, "invalid seed inventory")
			return 1
		}
		seeds = append(seeds, seed)
	}
	var raw []byte
	var err error
	if fs.Arg(0) == "-" {
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(fs.Arg(0))
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	artifact, err := programc.ComputeInstabilityV5(raw, seeds, *algorithmVersion, *parametersSHA256, *resourcePolicySHA256)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, err = stdout.Write(encoded); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
