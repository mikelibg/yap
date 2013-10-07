package morphparse

import (
	"chukuparser/algorithm/perceptron"
	"chukuparser/algorithm/transition"
	TransitionModel "chukuparser/algorithm/transition/model"
	"chukuparser/nlp/format/conll"
	"chukuparser/nlp/format/lattice"
	"chukuparser/nlp/format/segmentation"
	"chukuparser/nlp/parser/dependency"
	. "chukuparser/nlp/parser/dependency/transition"
	"chukuparser/nlp/parser/dependency/transition/morph"
	nlp "chukuparser/nlp/types"
	"chukuparser/util"

	"encoding/gob"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/gonuts/commander"
	"github.com/gonuts/flag"
)

var (
	MORPH_FEATURES [][2]string = [][2]string{
		{"S0|w", "S0|w"},
		{"S0|p", "S0|w"},
		{"S0|w|p", "S0|w"},

		{"N0|w", "N0|w"},
		{"N0|p", "N0|w"},
		{"N0|w|p", "N0|w"},

		{"N1|w", "N1|w"},
		{"N1|p", "N1|w"},
		{"N1|w|p", "N1|w"},

		{"N2|w", "N2|w"},
		{"N2|p", "N2|w"},
		{"N2|w|p", "N2|w"},

		{"S0h|w", "S0h|w"},
		{"S0h|p", "S0h|w"},
		{"S0|l", "S0h|w"},

		{"S0h2|w", "S0h2|w"},
		{"S0h2|p", "S0h2|w"},
		{"S0h|l", "S0h2|w"},

		{"S0l|w", "S0l|w"},
		{"S0l|p", "S0l|w"},
		{"S0l|l", "S0l|w"},

		{"S0r|w", "S0r|w"},
		{"S0r|p", "S0r|w"},
		{"S0r|l", "S0r|w"},

		{"S0l2|w", "S0l2|w"},
		{"S0l2|p", "S0l2|w"},
		{"S0l2|l", "S0l2|w"},

		{"S0r2|w", "S0r2|w"},
		{"S0r2|p", "S0r2|w"},
		{"S0r2|l", "S0r2|w"},

		{"N0l|w", "N0l|w"},
		{"N0l|p", "N0l|w"},
		{"N0l|l", "N0l|w"},

		{"N0l2|w", "N0l2|w"},
		{"N0l2|p", "N0l2|w"},
		{"N0l2|l", "N0l2|w"},

		{"S0|w|p+N0|w|p", "S0|w"},
		{"S0|w|p+N0|w", "S0|w"},
		{"S0|w+N0|w|p", "S0|w"},
		{"S0|w|p+N0|p", "S0|w"},
		{"S0|p+N0|w|p", "S0|w"},
		{"S0|w+N0|w", "S0|w"},
		{"S0|p+N0|p", "S0|w"},

		{"N0|p+N1|p", "S0|w,N0|w"},
		{"N0|p+N1|p+N2|p", "S0|w,N0|w"},
		{"S0|p+N0|p+N1|p", "S0|w,N0|w"},
		{"S0|p+N0|p+N0l|p", "S0|w,N0|w"},
		{"N0|p+N0l|p+N0l2|p", "S0|w,N0|w"},

		{"S0h|p+S0|p+N0|p", "S0|w"},
		{"S0h2|p+S0h|p+S0|p", "S0|w"},
		{"S0|p+S0l|p+N0|p", "S0|w"},
		{"S0|p+S0l|p+S0l2|p", "S0|w"},
		{"S0|p+S0r|p+N0|p", "S0|w"},
		{"S0|p+S0r|p+S0r2|p", "S0|w"},

		{"S0|w|d", "S0|w,N0|w"},
		{"S0|p|d", "S0|w,N0|w"},
		{"N0|w|d", "S0|w,N0|w"},
		{"N0|p|d", "S0|w,N0|w"},
		{"S0|w+N0|w|d", "S0|w,N0|w"},
		{"S0|p+N0|p|d", "S0|w,N0|w"},

		{"S0|w|vr", "S0|w"},
		{"S0|p|vr", "S0|w"},
		{"S0|w|vl", "S0|w"},
		{"S0|p|vl", "S0|w"},
		{"N0|w|vl", "N0|w"},
		{"N0|p|vl", "N0|w"},

		{"S0|w|sr", "S0|w"},
		{"S0|p|sr", "S0|w"},
		{"S0|w|sl", "S0|w"},
		{"S0|p|sl", "S0|w"},
		{"N0|w|sl", "N0|w"},
		{"N0|p|sl", "N0|w"},

		// {"N0|t", "S0|w"}, // all pos tags of morph queue
		// {"A0|g", "A0|g"}, // agreement
		// {"A0|p", "A0|p"},
		// {"A0|n", "A0|n"},
		// {"A0|t", "A0|t"},
		// {"A0|o", "A0|o"},
		// {"M0|w", "M0|w"}, // lattice bigram and trigram
		// {"M1|w", "M1|w"},
		// {"M2|w", "M2|w"},
		// {"M0|w+M1|w", "S0|w"}, // bi/tri gram combined
		// {"M0|w+M1|w+M2|w", "S0|w"},
	}

	LABELS []nlp.DepRel = []nlp.DepRel{
		"acc", "advmod", "amod", "appos",
		"aux", "cc", "ccomp", "comp",
		"complmn", "compound", "conj", "cop",
		"def", "dep", "det", "detmod",
		"gen", "ghd", "gobj", "hd",
		"mod", "mwe", "neg", "nn",
		"null", "num", "number", "obj",
		"parataxis", "pcomp", "pobj", "posspmod",
		"prd", "prep", "prepmod", "punct",
		"qaux", "rcmod", "rel", "relcomp",
		"subj", "tmod", "xcomp", "None",
		nlp.ROOT_LABEL,
	}

	Iterations               int
	BeamSize                 int
	ConcurrentBeam           bool
	tConll, tLatDis, tLatAmb string
	tSeg                     string
	input                    string
	outLat, outSeg           string
	modelFile                string
	REQUIRED_FLAGS           []string = []string{"it", "tc", "td", "tl", "in", "oc", "os", "ots"}

	// Global enumerations
	ERel, ETrans, EWord, EPOS, EWPOS *Util.EnumSet

	// Enumeration offsets of transitions
	SH, RE, PR, IDLE, LA, RA, MD Transition.Transition
)

func SetupRelationEnum() {
	if ERel != nil {
		return
	}
	ERel = Util.NewEnumSet(len(LABELS))
	for _, label := range LABELS {
		ERel.Add(label)
	}
	ERel.Frozen = true
}

// An approximation of the number of different MD-X:Y:Z transitions
// Pre-allocating the enumeration saves frequent reallocation during training and parsing
const (
	APPROX_MORPH_TRANSITIONS = 100
	APPROX_WORDS, APPROX_POS = 100, 100
)

func SetupMorphTransEnum() {
	ETrans = Util.NewEnumSet(len(LABELS)*2 + 2 + APPROX_MORPH_TRANSITIONS)
	_, _ = ETrans.Add("NO") // dummy for 0 action
	iSH, _ := ETrans.Add("SH")
	iRE, _ := ETrans.Add("RE")
	iPR, _ := ETrans.Add("PR")
	iIDLE, _ := ETrans.Add("IDLE")
	SH = Transition.Transition(iSH)
	RE = Transition.Transition(iRE)
	PR = Transition.Transition(iPR)
	IDLE = Transition.Transition(iIDLE)
	LA = IDLE + 1
	for _, transition := range LABELS {
		ETrans.Add("LA-" + string(transition))
	}
	RA = Transition.Transition(ETrans.Len())
	for _, transition := range LABELS {
		ETrans.Add("RA-" + string(transition))
	}
	MD = Transition.Transition(ETrans.Len())
}

func SetupEnum() {
	SetupRelationEnum()
	SetupMorphTransEnum()
	EWord, EPOS, EWPOS = Util.NewEnumSet(APPROX_WORDS), Util.NewEnumSet(APPROX_POS), Util.NewEnumSet(APPROX_WORDS*5)
}

func TrainingSequences(trainingSet []*Morph.BasicMorphGraph, transitionSystem Transition.TransitionSystem, extractor Perceptron.FeatureExtractor) []Perceptron.DecodedInstance {
	// verify feature load

	mconf := &Morph.MorphConfiguration{
		SimpleConfiguration: SimpleConfiguration{
			EWord:  EWord,
			EPOS:   EPOS,
			EWPOS:  EWPOS,
			ERel:   ERel,
			ETrans: ETrans,
		},
	}

	deterministic := &Deterministic{
		TransFunc:          transitionSystem,
		FeatExtractor:      extractor,
		ReturnModelValue:   false,
		ReturnSequence:     true,
		ShowConsiderations: false,
		Base:               mconf,
		// NoRecover:          true,
	}

	decoder := Perceptron.EarlyUpdateInstanceDecoder(deterministic)
	updater := new(TransitionModel.AveragedModelStrategy)

	perceptron := &Perceptron.LinearPerceptron{Decoder: decoder, Updater: updater}
	model := TransitionModel.NewAvgMatrixSparse(len(MORPH_FEATURES), nil)

	tempModel := Dependency.TransitionParameterModel(&PerceptronModel{model})
	perceptron.Init(model)

	instances := make([]Perceptron.DecodedInstance, 0, len(trainingSet))
	var failedTraining int
	for i, graph := range trainingSet {
		if i%100 == 0 {
			log.Println("At line", i)
		}
		sent := graph.Lattice

		_, goldParams := deterministic.ParseOracle(graph, nil, tempModel)
		if goldParams != nil {
			seq := goldParams.(*ParseResultParameters).Sequence
			// log.Println("Gold seq:\n", seq)
			decoded := &Perceptron.Decoded{sent, seq[0]}
			instances = append(instances, decoded)
		} else {
			failedTraining++
		}
	}
	return instances
}

func ReadTraining(filename string) []Perceptron.DecodedInstance {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		panic(err)
	}

	var instances []Perceptron.DecodedInstance
	dec := gob.NewDecoder(file)
	err = dec.Decode(&instances)
	if err != nil {
		panic(err)
	}
	return instances
}

func WriteTraining(instances []Perceptron.DecodedInstance, filename string) {
	file, err := os.Create(filename)
	defer file.Close()
	if err != nil {
		panic(err)
	}
	enc := gob.NewEncoder(file)
	err = enc.Encode(instances)
	if err != nil {
		panic(err)
	}
}

func Train(trainingSet []Perceptron.DecodedInstance, Iterations, BeamSize int, filename string, model Perceptron.Model, transitionSystem Transition.TransitionSystem, extractor Perceptron.FeatureExtractor) *Perceptron.LinearPerceptron {
	conf := &Morph.MorphConfiguration{
		SimpleConfiguration: SimpleConfiguration{
			EWord:  EWord,
			EPOS:   EPOS,
			EWPOS:  EWPOS,
			ERel:   ERel,
			ETrans: ETrans,
		},
	}

	beam := Beam{
		TransFunc:      transitionSystem,
		FeatExtractor:  extractor,
		Base:           conf,
		NumRelations:   len(LABELS),
		Size:           BeamSize,
		ConcurrentExec: ConcurrentBeam,
	}

	varbeam := &VarBeam{beam}
	decoder := Perceptron.EarlyUpdateInstanceDecoder(varbeam)
	updater := new(TransitionModel.AveragedModelStrategy)

	perceptron := &Perceptron.LinearPerceptron{
		Decoder:   decoder,
		Updater:   updater,
		Tempfile:  filename,
		TempLines: 1000}

	perceptron.Iterations = Iterations
	perceptron.Init(model)
	// perceptron.TempLoad("model.b64.i1")
	perceptron.Log = true

	perceptron.Train(trainingSet)
	log.Println("TRAIN Total Time:", varbeam.DurTotal)
	log.Println("TRAIN Time Expanding (pct):\t", varbeam.DurExpanding.Seconds(), 100*varbeam.DurExpanding/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting (pct):\t", varbeam.DurInserting.Seconds(), 100*varbeam.DurInserting/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-Feat (pct):\t", varbeam.DurInsertFeat.Seconds(), 100*varbeam.DurInsertFeat/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-Modl (pct):\t", varbeam.DurInsertModl.Seconds(), 100*varbeam.DurInsertModl/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-ModA (pct):\t", varbeam.DurInsertModA.Seconds(), 100*varbeam.DurInsertModA/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-ModB (pct):\t", varbeam.DurInsertModB.Seconds(), 100*varbeam.DurInsertModB/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-ModC (pct):\t", varbeam.DurInsertModC.Seconds(), 100*varbeam.DurInsertModC/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-Scrp (pct):\t", varbeam.DurInsertScrp.Seconds(), 100*varbeam.DurInsertScrp/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-Scrm (pct):\t", varbeam.DurInsertScrm.Seconds(), 100*varbeam.DurInsertScrm/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-Heap (pct):\t", varbeam.DurInsertHeap.Seconds(), 100*varbeam.DurInsertHeap/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-Agen (pct):\t", varbeam.DurInsertAgen.Seconds(), 100*varbeam.DurInsertAgen/varbeam.DurTotal)
	log.Println("TRAIN Time Inserting-Init (pct):\t", varbeam.DurInsertInit.Seconds(), 100*varbeam.DurInsertInit/varbeam.DurTotal)
	log.Println("TRAIN Time Top (pct):\t\t", varbeam.DurTop.Seconds(), 100*varbeam.DurTop/varbeam.DurTotal)
	log.Println("TRAIN Time TopB (pct):\t\t", varbeam.DurTopB.Seconds(), 100*varbeam.DurTopB/varbeam.DurTotal)
	log.Println("TRAIN Time Clearing (pct):\t\t", varbeam.DurClearing.Seconds(), 100*varbeam.DurClearing/varbeam.DurTotal)
	log.Println("TRAIN Total Time:", varbeam.DurTotal.Seconds())

	return perceptron
}

func Parse(sents []nlp.LatticeSentence, BeamSize int, model Dependency.TransitionParameterModel, transitionSystem Transition.TransitionSystem, extractor Perceptron.FeatureExtractor) []nlp.MorphDependencyGraph {
	conf := &Morph.MorphConfiguration{
		SimpleConfiguration: SimpleConfiguration{
			EWord:  EWord,
			EPOS:   EPOS,
			EWPOS:  EWPOS,
			ERel:   ERel,
			ETrans: ETrans,
		},
	}

	beam := Beam{
		TransFunc:       transitionSystem,
		FeatExtractor:   extractor,
		Base:            conf,
		Size:            BeamSize,
		NumRelations:    len(LABELS),
		Model:           model,
		ConcurrentExec:  ConcurrentBeam,
		ShortTempAgenda: true}

	varbeam := &VarBeam{beam}

	parsedGraphs := make([]nlp.MorphDependencyGraph, len(sents))
	for i, sent := range sents {
		// if i%100 == 0 {
		runtime.GC()
		log.Println("Parsing sent", i)
		// }
		graph, _ := varbeam.Parse(sent, nil, model)
		labeled := graph.(nlp.MorphDependencyGraph)
		parsedGraphs[i] = labeled
	}
	log.Println("PARSE Time Expanding (pct):\t", varbeam.DurExpanding.Seconds(), 100*varbeam.DurExpanding/varbeam.DurTotal)
	log.Println("PARSE Time Inserting (pct):\t", varbeam.DurInserting.Seconds(), 100*varbeam.DurInserting/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-Feat (pct):\t", varbeam.DurInsertFeat.Seconds(), 100*varbeam.DurInsertFeat/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-Modl (pct):\t", varbeam.DurInsertModl.Seconds(), 100*varbeam.DurInsertModl/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-ModA (pct):\t", varbeam.DurInsertModA.Seconds(), 100*varbeam.DurInsertModA/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-ModB (pct):\t", varbeam.DurInsertModB.Seconds(), 100*varbeam.DurInsertModB/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-ModC (pct):\t", varbeam.DurInsertModC.Seconds(), 100*varbeam.DurInsertModC/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-Scrp (pct):\t", varbeam.DurInsertScrp.Seconds(), 100*varbeam.DurInsertScrp/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-Scrm (pct):\t", varbeam.DurInsertScrm.Seconds(), 100*varbeam.DurInsertScrm/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-Heap (pct):\t", varbeam.DurInsertHeap.Seconds(), 100*varbeam.DurInsertHeap/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-Agen (pct):\t", varbeam.DurInsertAgen.Seconds(), 100*varbeam.DurInsertAgen/varbeam.DurTotal)
	log.Println("PARSE Time Inserting-Init (pct):\t", varbeam.DurInsertInit.Seconds(), 100*varbeam.DurInsertInit/varbeam.DurTotal)
	log.Println("PARSE Time Top (pct):\t\t", varbeam.DurTop.Seconds(), 100*varbeam.DurTop/varbeam.DurTotal)
	log.Println("PARSE Time TopB (pct):\t\t", varbeam.DurTopB.Seconds(), 100*varbeam.DurTopB/varbeam.DurTotal)
	log.Println("PARSE Time Clearing (pct):\t\t", varbeam.DurClearing.Seconds(), 100*varbeam.DurClearing/varbeam.DurTotal)
	log.Println("PARSE Total Time:", varbeam.DurTotal.Seconds())

	return parsedGraphs
}

// func WriteModel(model Perceptron.Model, filename string) {
// 	file, err := os.Create(filename)
// 	defer file.Close()
// 	if err != nil {
// 		panic(err)
// 	}
// 	model.Write(file)
// }

// func ReadModel(filename string) *Perceptron.LinearPerceptron {
// 	file, err := os.Open(filename)
// 	defer file.Close()
// 	if err != nil {
// 		panic(err)
// 	}
// 	model := new(Perceptron.LinearPerceptron)
// 	model.Read(file)
// 	return model
// }

func RegisterTypes() {
	gob.Register(Transition.ConfigurationSequence{})
	gob.Register(&Morph.BasicMorphGraph{})
	gob.Register(&nlp.Morpheme{})
	gob.Register(&BasicDepArc{})
	gob.Register(&Beam{})
	gob.Register(&Morph.MorphConfiguration{})
	gob.Register(&Morph.ArcEagerMorph{})
	gob.Register(&GenericExtractor{})
	gob.Register(&PerceptronModel{})
	gob.Register(&Perceptron.AveragedStrategy{})
	gob.Register(&Perceptron.Decoded{})
	gob.Register(nlp.LatticeSentence{})
	gob.Register(&StackArray{})
	gob.Register(&ArcSetSimple{})
	gob.Register([3]interface{}{})
	gob.Register(new(Transition.Transition))
}

func CombineTrainingInputs(graphs []nlp.LabeledDependencyGraph, goldLats, ambLats []nlp.LatticeSentence) ([]*Morph.BasicMorphGraph, int) {
	if len(graphs) != len(goldLats) || len(graphs) != len(ambLats) {
		panic(fmt.Sprintf("Got mismatched training slice inputs (graphs, gold lattices, ambiguous lattices):", len(graphs), len(goldLats), len(ambLats)))
	}
	morphGraphs := make([]*Morph.BasicMorphGraph, len(graphs))
	var (
		numLatticeNoGold int
		noGold           bool
	)
	prefix := log.Prefix()
	for i, goldGraph := range graphs {
		goldLat := goldLats[i]
		ambLat := ambLats[i]
		log.SetPrefix(fmt.Sprintf("%v graph# %v ", prefix, i))
		morphGraphs[i], noGold = Morph.CombineToGoldMorph(goldGraph, goldLat, ambLat)
		if noGold {
			numLatticeNoGold++
		}
	}
	log.SetPrefix(prefix)
	return morphGraphs, numLatticeNoGold
}

func VerifyExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		log.Println("Error accessing file", filename)
		log.Println(err.Error())
		return false
	}
	return true
}

func VerifyFlags(cmd *commander.Command) {
	for _, flag := range REQUIRED_FLAGS {
		f := cmd.Flag.Lookup(flag)
		if f.Value.String() == "" {
			log.Printf("Required flag %s not set", f.Name)
			cmd.Usage()
			os.Exit(1)
		}
	}
}

func ConfigOut(outModelFile string) {
	log.Println("Configuration")
	log.Printf("Beam:             \tVariable Length")
	log.Printf("Transition System:\tIDLE + Morph + ArcEager")
	log.Printf("Features:         \tRich + Q Tags + Morph + Agreement")
	log.Printf("Iterations:\t\t%d", Iterations)
	log.Printf("Beam Size:\t\t%d", BeamSize)
	log.Printf("Beam Concurrent:\t%v", ConcurrentBeam)
	log.Printf("Model file:\t\t%s", outModelFile)
	log.Println()
	log.Println("Data")
	log.Printf("Train file (conll):\t\t\t%s", tConll)
	if !VerifyExists(tConll) {
		return
	}
	log.Printf("Train file (disamb. lattice):\t%s", tLatDis)
	if !VerifyExists(tLatDis) {
		return
	}
	log.Printf("Train file (ambig.  lattice):\t%s", tLatAmb)
	if !VerifyExists(tLatAmb) {
		return
	}
	log.Printf("Test file  (ambig.  lattice):\t%s", input)
	if !VerifyExists(input) {
		return
	}
	log.Printf("Out (disamb.) file:\t\t\t%s", outLat)
	log.Printf("Out (segmt.) file:\t\t\t%s", outSeg)
	log.Printf("Out Train (segmt.) file:\t\t%s", tSeg)
}

func MorphTrainAndParse(cmd *commander.Command, args []string) {
	VerifyFlags(cmd)
	RegisterTypes()

	outModelFile := fmt.Sprintf("%s.b%d.i%d", modelFile, BeamSize, Iterations)

	ConfigOut(outModelFile)

	log.Println()
	// start processing - setup enumerations
	log.Println("Setup enumerations")
	SetupEnum()
	log.Println()

	log.Println("Generating Gold Sequences For Training")
	log.Println("Conll:\tReading training conll sentences from", tConll)
	s, e := Conll.ReadFile(tConll)
	if e != nil {
		log.Println(e)
		return
	}
	log.Println("Conll:\tRead", len(s), "sentences")
	log.Println("Conll:\tConverting from conll to internal structure")
	goldConll := Conll.Conll2GraphCorpus(s, EWord, EPOS, EWPOS, ERel)

	log.Println("Dis. Lat.:\tReading training disambiguated lattices from", tLatDis)
	lDis, lDisE := Lattice.ReadFile(tLatDis)
	if lDisE != nil {
		log.Println(lDisE)
		return
	}
	log.Println("Dis. Lat.:\tRead", len(lDis), "disambiguated lattices")
	log.Println("Dis. Lat.:\tConverting lattice format to internal structure")
	goldDisLat := Lattice.Lattice2SentenceCorpus(lDis, EWord, EPOS, EWPOS)

	log.Println("Amb. Lat:\tReading ambiguous lattices from", tLatAmb)
	lAmb, lAmbE := Lattice.ReadFile(tLatAmb)
	if lAmbE != nil {
		log.Println(lAmbE)
		return
	}
	log.Println("Amb. Lat:\tRead", len(lAmb), "ambiguous lattices")
	log.Println("Amb. Lat:\tConverting lattice format to internal structure")
	goldAmbLat := Lattice.Lattice2SentenceCorpus(lAmb, EWord, EPOS, EWPOS)

	log.Println("Combining train files into gold morph graphs with original lattices")
	combined, missingGold := CombineTrainingInputs(goldConll, goldDisLat, goldAmbLat)

	log.Println("Combined", len(combined), "graphs, with", missingGold, "missing at least one gold path in lattice")

	log.Println()

	log.Println("Loading features")
	extractor := &GenericExtractor{
		EFeatures:  Util.NewEnumSet(len(MORPH_FEATURES)),
		Concurrent: false,
	}
	extractor.Init()
	for _, featurePair := range MORPH_FEATURES {
		if err := extractor.LoadFeature(featurePair[0], featurePair[1]); err != nil {
			log.Panicln("Failed to load feature", err.Error())
		}
	}

	morphArcSystem := &Morph.ArcEagerMorph{
		ArcEager: ArcEager{
			ArcStandard: ArcStandard{
				SHIFT:       SH,
				LEFT:        LA,
				RIGHT:       RA,
				Relations:   ERel,
				Transitions: ETrans,
			},
			REDUCE:  RE,
			POPROOT: PR},
		MD: MD,
	}
	morphArcSystem.AddDefaultOracle()

	arcSystem := &Morph.Idle{morphArcSystem, IDLE}
	transitionSystem := Transition.TransitionSystem(arcSystem)

	log.Println()

	log.Println("Parsing with gold to get training sequences")
	// const NUM_SENTS = 20
	// combined = combined[:NUM_SENTS]
	goldSequences := TrainingSequences(combined, transitionSystem, extractor)
	log.Println("Generated", len(goldSequences), "training sequences")
	log.Println()
	// Util.LogMemory()
	log.Println("Training", Iterations, "iteration(s)")
	formatters := make([]Util.Format, len(extractor.FeatureTemplates))
	for i, formatter := range extractor.FeatureTemplates {
		formatters[i] = formatter
	}
	model := TransitionModel.NewAvgMatrixSparse(len(MORPH_FEATURES), formatters)
	_ = Train(goldSequences, Iterations, BeamSize, modelFile, model, transitionSystem, extractor)
	log.Println("Done Training")
	// Util.LogMemory()
	log.Println()
	// log.Println("Writing final model to", outModelFile)
	// WriteModel(model, outModelFile)
	// log.Println()
	log.Print("Parsing test")

	log.Println("Reading ambiguous lattices from", input)
	lAmb, lAmbE = Lattice.ReadFile(input)
	if lAmbE != nil {
		log.Println(lAmbE)
		return
	}
	// lAmb = lAmb[:NUM_SENTS]

	log.Println("Read", len(lAmb), "ambiguous lattices from", input)
	log.Println("Converting lattice format to internal structure")
	predAmbLat := Lattice.Lattice2SentenceCorpus(lAmb, EWord, EPOS, EWPOS)

	parsedGraphs := Parse(predAmbLat, BeamSize, Dependency.TransitionParameterModel(&PerceptronModel{model}), transitionSystem, extractor)

	log.Println("Converting", len(parsedGraphs), "to conll")
	graphAsConll := Conll.MorphGraph2ConllCorpus(parsedGraphs)
	log.Println("Writing to output file")
	Conll.WriteFile(outLat, graphAsConll)
	log.Println("Wrote", len(graphAsConll), "in conll format to", outLat)

	log.Println("Writing to segmentation file")
	Segmentation.WriteFile(outSeg, parsedGraphs)
	log.Println("Wrote", len(parsedGraphs), "in segmentation format to", outSeg)

	log.Println("Writing to gold segmentation file")
	Segmentation.WriteFile(tSeg, ToMorphGraphs(combined))
	log.Println("Wrote", len(combined), "in segmentation format to", tSeg)
}

func ToMorphGraphs(graphs []*Morph.BasicMorphGraph) []nlp.MorphDependencyGraph {
	morphs := make([]nlp.MorphDependencyGraph, len(graphs))
	for i, g := range graphs {
		morphs[i] = nlp.MorphDependencyGraph(g)
	}
	return morphs
}

func MorphCmd() *commander.Command {
	cmd := &commander.Command{
		Run:       MorphTrainAndParse,
		UsageLine: "morph <file options> [arguments]",
		Short:     "runs morpho-syntactic training and parsing",
		Long: `
runs morpho-syntactic training and parsing

	$ ./chukuparser morph -tc <conll> -td <train disamb. lat> -tl <train amb. lat> -in <input lat> -oc <out lat> -os <out seg> -ots <out train seg> [options]

`,
		Flag: *flag.NewFlagSet("morph", flag.ExitOnError),
	}
	cmd.Flag.BoolVar(&ConcurrentBeam, "bconc", false, "Concurrent Beam")
	cmd.Flag.IntVar(&Iterations, "it", 1, "Number of Perceptron Iterations")
	cmd.Flag.IntVar(&BeamSize, "b", 4, "Beam Size")
	cmd.Flag.StringVar(&modelFile, "m", "model", "Prefix for model file ({m}.b{b}.i{i}.model)")

	cmd.Flag.StringVar(&tConll, "tc", "", "Training Conll File")
	cmd.Flag.StringVar(&tLatDis, "td", "", "Training Disambiguated Lattices File")
	cmd.Flag.StringVar(&tLatAmb, "tl", "", "Training Ambiguous Lattices File")
	cmd.Flag.StringVar(&input, "in", "", "Test Ambiguous Lattices File")
	cmd.Flag.StringVar(&outLat, "oc", "", "Output Conll File")
	cmd.Flag.StringVar(&outSeg, "os", "", "Output Segmentation File")
	cmd.Flag.StringVar(&tSeg, "ots", "", "Output Training Segmentation File")
	return cmd
}