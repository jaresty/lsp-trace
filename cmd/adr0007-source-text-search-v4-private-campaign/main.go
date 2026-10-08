package main

import (
	"flag"
	"fmt"
	"os"

	campaign "lsp-trace/internal/adr0007sourcetextsearchv4campaign"
)

func main() {
	mode := flag.String("mode", "", "prepare|predispatch|execute-production|execute-oracle|review|audit|seal|verify|build-binaries")
	root := flag.String("root", ".", "repository root")
	campaignID := flag.String("campaign-id", campaign.CampaignID, "campaign id")
	production := flag.String("production-bin", "", "production binary")
	oracle := flag.String("oracle-bin", "", "oracle binary")
	validator := flag.String("validator-bin", "", "validator binary")
	out := flag.String("out", "", "binary output directory for build-binaries")
	flag.Parse()
	o := campaign.Options{Root: *root, CampaignID: *campaignID, ProductionBin: *production, OracleBin: *oracle, ValidatorBin: *validator}
	var err error
	switch *mode {
	case "prepare":
		err = campaign.Prepare(o)
	case "predispatch":
		err = campaign.Predispatch(o)
	case "execute-production":
		err = campaign.Execute(o, "production")
	case "execute-oracle":
		err = campaign.Execute(o, "oracle")
	case "review":
		err = campaign.Review(o)
	case "audit", "seal", "verify":
		err = campaign.AuditSealVerify(o, *mode)
	case "build-binaries":
		if *out == "" {
			err = fmt.Errorf("-out required")
		} else {
			err = campaign.BuildBinaries(*root, *out)
		}
	default:
		err = fmt.Errorf("unknown -mode %q", *mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
