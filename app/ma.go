package app

import (
	"yap/nlp/format/lattice"
	"yap/nlp/format/raw"

	"yap/nlp/parser/ma"
	nlp "yap/nlp/types"
	// "yap/util"

	"fmt"
	"log"
	// "os"
	"strings"

	"github.com/gonuts/commander"
	"github.com/gonuts/flag"
)

var (
	dictFile, inRawFile, outLatticeFile string
	maxOOVMSRPerPOS                     int = 10
)

func MAConfigOut() {
	log.Println("Configuration")
	log.Printf("MA Dict:\t%s", dictFile)
	log.Printf("Max OOV Msrs/POS:\t%v", maxOOVMSRPerPOS)
	log.Println()
	log.Printf("Raw Input:    \t%s", inRawFile)
	log.Printf("Output:    \t%s", outLatticeFile)
	log.Println()
}

func MA(cmd *commander.Command, args []string) {
	REQUIRED_FLAGS := []string{"dict", "raw", "out"}

	VerifyFlags(cmd, REQUIRED_FLAGS)

	MAConfigOut()

	log.Println("Reading Morphological Analyzer Dictionary")
	maData := new(ma.MADict)
	if err := maData.ReadFile(dictFile); err != nil {
		panic(fmt.Sprintf("Failed reading MA dict file - %v", err))
	}
	log.Println("OOV POSs:", strings.Join(maData.TopPOS, ", "))
	maData.ComputeOOVMSRs(maxOOVMSRPerPOS)
	log.Println()
	sents, err := raw.ReadFile(inRawFile)
	if err != nil {
		panic(fmt.Sprintf("Failed reading raw file - %v", err))
	}
	log.Println("Running Morphological Analysis")
	lattices := make([]nlp.LatticeSentence, len(sents))
	stats := new(ma.AnalyzeStats)
	stats.Init()
	for i, sent := range sents {
		lattices[i], _ = maData.Analyze(sent.Tokens(), stats)
	}
	log.Println("Analyzed", stats.TotalTokens, "occurences of", len(stats.UniqTokens), "unique tokens")
	log.Println("Encountered", stats.OOVTokens, "occurences of", len(stats.UniqOOVTokens), "unknown tokens")
	output := lattice.Sentence2LatticeCorpus(lattices)
	lattice.WriteFile(outLatticeFile, output)
	log.Println("Wrote", len(output), "lattices")
}

func MACmd() *commander.Command {
	cmd := &commander.Command{
		Run:       MA,
		UsageLine: "ma <file options> [arguments]",
		Short:     "run data-driven morphological analyzer on raw input",
		Long: `
run data-driven morphological analyzer on raw input

	$ ./yap ma -dict <dict file> -raw <raw file> -out <output file> [options]

`,
		Flag: *flag.NewFlagSet("ma", flag.ExitOnError),
	}
	cmd.Flag.StringVar(&dictFile, "dict", "", "Dictionary for morphological analyzer")
	cmd.Flag.StringVar(&inRawFile, "raw", "", "Input raw (tokenized) file")
	cmd.Flag.StringVar(&outLatticeFile, "out", "", "Output lattice file")
	cmd.Flag.IntVar(&maxOOVMSRPerPOS, "maxmsrperpos", 10, "For OOV tokens, max MSRs per POS to add")
	return cmd
}