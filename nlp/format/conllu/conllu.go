package conllu

// Package ConllU reads ConLL-U format files
// Note that a ConLL-U sentence gets represented as a *lattice*
// For a description see
// https://universaldependencies.github.io/docs/format.html

import (
	"yap/alg/graph"
	"yap/nlp/parser/dependency/transition"
	morphtypes "yap/nlp/parser/dependency/transition/morph"
	nlp "yap/nlp/types"
	"yap/util"

	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	FIELD_SEPARATOR      = '\t'
	NUM_FIELDS           = 10
	FEATURES_SEPARATOR   = "|"
	FEATURE_SEPARATOR    = "="
	FEATURE_CONCAT_DELIM = ","
)

var (
	WORD_TYPE    = "lemma+f"
	IGNORE_LEMMA bool
)

type Features map[string]string

func (f Features) String() string {
	if f != nil || len(f) == 0 {
		return "_"
	}
	return fmt.Sprintf("%v", map[string]string(f))
}

func (f Features) MorphHost() string {
	hostStrs := make([]string, 0, len(f))
	for name, value := range f {
		if name[0:3] != "suf" {
			hostStrs = append(hostStrs, fmt.Sprintf("%v=%v", name, value))
		}
	}
	sort.Strings(hostStrs)
	return strings.Join(hostStrs, "|")
}

func (f Features) MorphSuffix() string {
	hostStrs := make([]string, 0, len(f))
	for name, value := range f {
		if name[0:3] == "suf" {
			hostStrs = append(hostStrs, fmt.Sprintf("%v=%v", name, value))
		}
	}
	sort.Strings(hostStrs)
	return strings.Join(hostStrs, "|")
}

func FormatFeatures(feat map[string]string) string {
	if feat == nil || len(feat) == 0 {
		return "_"
	}
	strs := make([]string, 0, len(feat))
	for k, v := range feat {
		strs = append(strs, fmt.Sprintf("%v%v%v", k, FEATURE_SEPARATOR, v))
	}
	sort.Strings(strs)
	return strings.Join(strs, FEATURES_SEPARATOR)
}

// A Row is a single parsed row of a conll data set
type Row struct {
	ID      int
	Form    string
	Lemma   string
	UPosTag string
	XPosTag string
	Feats   Features
	FeatStr string
	Head    int
	DepRel  string
	Deps    []string
	Misc    string
	TokenID int
}

func (r Row) String() string {
	fields := []string{
		fmt.Sprintf("%d", r.ID),
		r.Form,
		r.Lemma,
		r.UPosTag,
		r.XPosTag,
		r.FeatStr,
		fmt.Sprintf("%d", r.Head),
		r.DepRel,
		strings.Join(r.Deps, FEATURE_SEPARATOR),
		r.Misc}
	return strings.Join(fields, "\t")
}

// A Sentence is a map of Rows using their ids and a set of tokens
type Sentence struct {
	Deps   map[int]Row
	Tokens []string
}

func NewSentence() *Sentence {
	return &Sentence{
		Deps:   make(map[int]Row),
		Tokens: []string{},
	}
}

type Sentences []*Sentence

func ParseInt(value string) (int, error) {
	if value == "_" {
		return 0, nil
	}
	i, err := strconv.ParseInt(value, 10, 0)
	return int(i), err
}

func ParseString(value string) string {
	if value == "_" {
		return ""
	} else {
		return value
	}
}

func ParseFeatures(featuresStr string) (Features, error) {
	var featureMap Features
	if featuresStr == "_" {
		return featureMap, nil
	}

	featureList := strings.Split(featuresStr, FEATURES_SEPARATOR)
	if len(featureList) == 0 {
		return nil, errors.New("No features found, field should be '_'")
	}
	featureMap = make(Features, len(featureList))
	for _, featureStr := range featureList {
		featureKV := strings.Split(featureStr, FEATURE_SEPARATOR)
		if len(featureKV) != 2 {
			return nil, errors.New("Wrong number of fields for split of feature" + featureStr)
		}
		featName := featureKV[0]
		featValue := featureKV[1]
		existingFeatValue, featExist := featureMap[featName]
		if featExist {
			featureMap[featName] = existingFeatValue + FEATURE_CONCAT_DELIM + featValue
		} else {
			featureMap[featName] = featValue
		}
	}
	return featureMap, nil
}

func ParseRow(record []string) (Row, error) {
	var row Row
	id, err := ParseInt(record[0])
	if err != nil {
		return row, errors.New(fmt.Sprintf("Error parsing ID field (%s): %s", record[0], err.Error()))
	}
	row.ID = id

	form := ParseString(record[1])
	if form == "" {
		return row, errors.New("Empty FORM field")
	}
	row.Form = form

	if !IGNORE_LEMMA {
		lemma := ParseString(record[2])
		// if lemma == "" {
		// 	return row, errors.New("Empty LEMMA field")
		// }
		row.Lemma = lemma
	}

	upostag := ParseString(record[3])
	if upostag == "" {
		return row, errors.New("Empty UPOSTAG field")
	}
	row.UPosTag = upostag

	xpostag := ParseString(record[4])
	if xpostag == "" {
		return row, errors.New("Empty XPOSTAG field")
	}
	row.XPosTag = xpostag

	features, err := ParseFeatures(record[5])
	if err != nil {
		return row, errors.New(fmt.Sprintf("Error parsing FEATS field (%s): %s", record[5], err.Error()))
	}
	row.Feats = features
	row.FeatStr = ParseString(record[5])

	head, err := ParseInt(record[6])
	if err != nil {
		return row, errors.New(fmt.Sprintf("Error parsing HEAD field (%s): %s", record[6], err.Error()))
	}
	row.Head = head

	deprel := ParseString(record[7])
	if deprel == "" {
		return row, errors.New("Empty DEPREL field")
	}
	row.DepRel = deprel

	deps := ParseString(record[8])
	if len(deps) > 0 {
		row.Deps = strings.Split(deps, FEATURE_SEPARATOR)
	}

	row.Misc = ParseString(record[9])

	return row, nil
}

func ParseTokenRow(record []string) (string, int, error) {
	// easier to debug if we know the token
	token := ParseString(record[1])
	if token == "" {
		return token, 0, errors.New("Empty TOKEN field for token row")
	}

	ids := strings.Split(record[0], "-")
	if len(ids) != 2 {
		return token, 0, errors.New(fmt.Sprintf("Error parsing ID span field (%s): wrong format for ID span for token row - needs <num>-<num>", record[0]))
	}
	id1, err := ParseInt(ids[0])
	if err != nil {
		return token, 0, errors.New(fmt.Sprintf("Error parsing ID span field (%s): %s for token row", record[0], err.Error()))
	}
	id2, err := ParseInt(ids[1])
	if err != nil {
		return token, 0, errors.New(fmt.Sprintf("Error parsing ID span field (%s): %s for token row", record[0], err.Error()))
	}
	if !(id2-id1 > 0) {
		return token, 0, errors.New(fmt.Sprintf("Error parsing ID span field (%s): wrong format for ID span for token row - needs second num (%d) - first num (%d) > 0", record[0], id2, id1))
	}

	return token, id2 - id1 + 1, nil
}

func Read(reader io.Reader) (Sentences, error) {
	var sentences []*Sentence
	bufReader := bufio.NewReader(reader)

	var (
		i        int
		token    string
		numForms int
	)
	currentSent := NewSentence()
	for curLine, isPrefix, err := bufReader.ReadLine(); err == nil; curLine, isPrefix, err = bufReader.ReadLine() {
		if isPrefix {
			panic("Buffer not large enough, fix me :(")
		}
		buf := bytes.NewBuffer(curLine)
		// log.Println("At record", i)
		// '#' is a start of comment for CONLL-U
		if len(curLine) == 0 {
			sentences = append(sentences, currentSent)
			currentSent = NewSentence()
			i++
			continue
		}

		record := strings.Split(buf.String(), "\t")
		if record[0][0] == '#' {
			// skip comment lines
			continue
		}
		if strings.Contains(record[0], "-") {
			token, numForms, err = ParseTokenRow(record)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Error processing record %d at statement %d: %s", i, len(sentences), err.Error()))
			}
			currentSent.Tokens = append(currentSent.Tokens, token)
		} else {
			row, err := ParseRow(record)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Error processing record %d at statement %d: %s", i, len(sentences), err.Error()))
			}
			if numForms > 0 {
				numForms--
			} else {
				currentSent.Tokens = append(currentSent.Tokens, row.Form)
			}
			row.TokenID = len(currentSent.Tokens) - 1
			currentSent.Deps[row.ID] = row
		}
	}
	return sentences, nil
}

func ReadFile(filename string) ([]*Sentence, error) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		return nil, err
	}

	return Read(file)
}

func Write(writer io.Writer, sents []interface{}) {
	for _, genericsent := range sents {
		sent := genericsent.(Sentence)
		for i := 1; i <= len(sent.Deps); i++ {
			row := sent.Deps[i]
			writer.Write(append([]byte(row.String()), '\n'))
		}
		writer.Write([]byte{'\n'})
	}
}

func WriteFile(filename string, sents []interface{}) error {
	file, err := os.Create(filename)
	defer file.Close()
	if err != nil {
		return err
	}
	Write(file, sents)
	return nil
}

func GetMorphProperties(node *transition.TaggedDepNode, eMHost, eMSuffix *util.EnumSet) string {
	host := eMHost.ValueOf(node.MHost).(string)
	suffix := eMSuffix.ValueOf(node.MSuffix).(string)
	if len(host) > 0 && len(suffix) > 0 {
		return fmt.Sprintf("%v|%v", host, suffix)
	}
	if len(host) > 0 {
		return host
	}
	if len(suffix) > 0 {
		return suffix
	}
	return "_"
}
func Graph2ConllU(graph nlp.LabeledDependencyGraph, eMHost, eMSuffix *util.EnumSet) *Sentence {
	sent := NewSentence()
	arcIndex := make(map[int]nlp.LabeledDepArc, graph.NumberOfNodes())
	var (
		posTag string
		lemma  string
		node   nlp.DepNode
		arc    nlp.LabeledDepArc
		headID int
		depRel string
	)
	// log.Println(graph.(*transition.SimpleConfiguration).InternalArcs)
	for _, arcID := range graph.GetEdges() {
		// log.Println("Getting arc id", arcID)
		arc = graph.GetLabeledArc(arcID)
		if arc == nil {
			// log.Println("Failed edge", arcID)
			// panic("Can't find arc")
		} else {
			arcIndex[arc.GetModifier()] = arc
			// log.Println("Found edge", arcID)
		}
	}
	for _, nodeID := range graph.GetVertices() {
		node = graph.GetNode(nodeID)
		posTag = ""

		taggedToken, ok := node.(*transition.TaggedDepNode)
		if !ok {
			panic("Got node of type other than TaggedDepNode")
		}
		posTag = taggedToken.RawPOS
		if !IGNORE_LEMMA {
			lemma = taggedToken.RawLemma
		} else {
			lemma = ""
		}

		if node == nil {
			panic("Can't find node")
		}
		arc, exists := arcIndex[node.ID()]
		if exists {
			headID = arc.GetHead()
			depRel = string(arc.GetRelation())
			if depRel == nlp.ROOT_LABEL {
				headID = -1
			}
		} else {
			headID = -1
			depRel = "None"
		}
		row := Row{
			ID:      node.ID() + 1,
			Form:    node.String(),
			Lemma:   lemma,
			UPosTag: posTag,
			XPosTag: posTag,
			FeatStr: GetMorphProperties(taggedToken, eMHost, eMSuffix),
			Head:    headID + 1,
			DepRel:  depRel,
		}
		sent.Deps[row.ID] = row
	}
	return sent
}

func Graph2ConllUCorpus(corpus []interface{}, eMHost, eMSuffix *util.EnumSet) []interface{} {
	sentCorpus := make([]interface{}, len(corpus))
	for i, graph := range corpus {
		sentCorpus[i] = Graph2ConllU(graph.(nlp.LabeledDependencyGraph), eMHost, eMSuffix)
	}
	return sentCorpus
}

func ConllU2MorphGraph(sent *Sentence, eWord, ePOS, eWPOS, eRel, eMFeat, eMHost, eMSuffix *util.EnumSet) nlp.MorphDependencyGraph {
	var (
		arc        *transition.BasicDepArc
		node       *transition.TaggedDepNode
		index      int
		curLatNode int
	)
	mappings := make(nlp.Mappings, len(sent.Tokens))
	lattices := make(nlp.LatticeSentence, len(sent.Tokens))
	nodes := make([]nlp.DepNode, 0, len(sent.Deps)+2)
	// log.Println("\tNum Nodes:", len(nodes))
	arcs := make([]*transition.BasicDepArc, len(sent.Deps))
	// node.Token, _ = eWord.Add(nlp.ROOT_TOKEN)
	// node.POS, _ = ePOS.Add(nlp.ROOT_TOKEN)
	// node.TokenPOS, _ = eWPOS.Add([2]string{nlp.ROOT_TOKEN, nlp.ROOT_TOKEN})
	// nodes = append(nodes, nlp.DepNode(node)) // add root node

	// Initialize mappings and lattice per token
	for i, token := range sent.Tokens {
		lattices[i] = nlp.Lattice{
			Token:     nlp.Token(token),
			Morphemes: nlp.Morphemes{},
			Next:      make(map[int][]int),
		}
	}

	for i := 1; i <= len(sent.Deps); i++ {
		row, _ := sent.Deps[i]
		// for i, row := range sent {
		node = &transition.TaggedDepNode{
			Id:       i - 1,
			RawToken: row.Form,
			RawPOS:   row.UPosTag,
		}

		switch WORD_TYPE {
		case "form":
			node.Token, _ = eWord.Add(row.Form)
			node.TokenPOS, _ = eWPOS.Add([2]string{row.Form, row.UPosTag})
		case "lemma":
			node.Token, _ = eWord.Add(row.Lemma)
			node.TokenPOS, _ = eWPOS.Add([2]string{row.Lemma, row.UPosTag})
		case "lemma+f":
			if row.Lemma != "" {
				node.Token, _ = eWord.Add(row.Lemma)
				node.TokenPOS, _ = eWPOS.Add([2]string{row.Lemma, row.UPosTag})
			} else {
				node.Token, _ = eWord.Add(row.Form)
				node.TokenPOS, _ = eWPOS.Add([2]string{row.Form, row.UPosTag})
			}
		default:
			panic(fmt.Sprintf("Unknown WORD_TYPE %s", WORD_TYPE))
		}
		node.POS, _ = ePOS.Add(row.UPosTag)
		node.MHost, _ = eMHost.Add(row.Feats.MorphHost())
		node.MSuffix, _ = eMSuffix.Add(row.Feats.MorphSuffix())
		index, _ = eRel.IndexOf(nlp.DepRel(row.DepRel))
		arc = &transition.BasicDepArc{row.Head - 1, index, i - 1, nlp.DepRel(row.DepRel)}
		// log.Println("Adding node", node, node.TokenPOS, eWPOS.ValueOf(node.TokenPOS))
		nodes = append(nodes, nlp.DepNode(node))
		// log.Println("Adding arc", i-1, arc)
		arcs[i-1] = arc

		lattice := &lattices[row.TokenID]
		if len(lattice.Next) == 0 {
			lattice.BottomId = curLatNode
		}
		lattice.TopId = curLatNode + 1
		lattice.Next[curLatNode] = []int{curLatNode}
		morph := nlp.Morpheme{
			graph.BasicDirectedEdge{curLatNode, curLatNode, curLatNode + 1},
			row.Form,
			row.Lemma,
			row.UPosTag,
			row.UPosTag,
			row.Feats,
			row.TokenID,
			row.FeatStr,
		}
		eFeat, _ := eMFeat.Add(row.FeatStr)
		lattice.Morphemes = append(lattice.Morphemes, &nlp.EMorpheme{
			morph,
			node.Token,
			node.Token, // TODO: should use ELemma
			node.TokenPOS,
			node.POS,
			eFeat,
			node.MHost,
			node.MSuffix,
		})

		curLatNode++
	}

	for i, lat := range lattices {
		lat.GenSpellouts()
		mappings[i] = &nlp.Mapping{lat.Token, lat.Spellouts[0]}
	}

	morphGraph := &morphtypes.BasicMorphGraph{
		transition.BasicDepGraph{nodes, arcs},
		mappings,
		lattices,
	}
	return nlp.MorphDependencyGraph(morphGraph)
}

func ConllU2MorphGraphCorpus(corpus []*Sentence, eWord, ePOS, eWPOS, eRel, eMFeat, eMHost, eMSuffix *util.EnumSet) []interface{} {
	graphCorpus := make([]interface{}, len(corpus))
	for i, sent := range corpus {
		// log.Println("Converting sentence", i)
		graphCorpus[i] = ConllU2MorphGraph(sent, eWord, ePOS, eWPOS, eRel, eMFeat, eMHost, eMSuffix)
	}
	return graphCorpus
}

func ConllU2Graph(sent *Sentence, eWord, ePOS, eWPOS, eRel, eMHost, eMSuffix *util.EnumSet) nlp.LabeledDependencyGraph {
	var (
		arc   *transition.BasicDepArc
		node  *transition.TaggedDepNode
		index int
	)
	nodes := make([]nlp.DepNode, 0, len(sent.Deps)+2)
	// log.Println("\tNum Nodes:", len(nodes))
	arcs := make([]*transition.BasicDepArc, len(sent.Deps))
	// node.Token, _ = eWord.Add(nlp.ROOT_TOKEN)
	// node.POS, _ = ePOS.Add(nlp.ROOT_TOKEN)
	// node.TokenPOS, _ = eWPOS.Add([2]string{nlp.ROOT_TOKEN, nlp.ROOT_TOKEN})
	// nodes = append(nodes, nlp.DepNode(node)) // add root node

	for i := 1; i <= len(sent.Deps); i++ {
		row, _ := sent.Deps[i]
		// for i, row := range sent {
		node = &transition.TaggedDepNode{
			Id:       i - 1,
			RawToken: row.Form,
			RawPOS:   row.UPosTag,
		}

		switch WORD_TYPE {
		case "form":
			node.Token, _ = eWord.Add(row.Form)
			node.TokenPOS, _ = eWPOS.Add([2]string{row.Form, row.UPosTag})
		case "lemma":
			node.Token, _ = eWord.Add(row.Lemma)
			node.TokenPOS, _ = eWPOS.Add([2]string{row.Lemma, row.UPosTag})
		case "lemma+f":
			if row.Lemma != "" {
				node.Token, _ = eWord.Add(row.Lemma)
				node.TokenPOS, _ = eWPOS.Add([2]string{row.Lemma, row.UPosTag})
			} else {
				node.Token, _ = eWord.Add(row.Form)
				node.TokenPOS, _ = eWPOS.Add([2]string{row.Form, row.UPosTag})
			}
		default:
			panic(fmt.Sprintf("Unknown WORD_TYPE %s", WORD_TYPE))
		}
		node.POS, _ = ePOS.Add(row.UPosTag)
		node.MHost, _ = eMHost.Add(row.Feats.MorphHost())
		node.MSuffix, _ = eMSuffix.Add(row.Feats.MorphSuffix())
		index, _ = eRel.IndexOf(nlp.DepRel(row.DepRel))
		arc = &transition.BasicDepArc{row.Head - 1, index, i - 1, nlp.DepRel(row.DepRel)}
		// log.Println("Adding node", node, node.TokenPOS, eWPOS.ValueOf(node.TokenPOS))
		nodes = append(nodes, nlp.DepNode(node))
		// log.Println("Adding arc", i-1, arc)
		arcs[i-1] = arc
	}
	return nlp.LabeledDependencyGraph(&transition.BasicDepGraph{nodes, arcs})
}

func ConllU2GraphCorpus(corpus []*Sentence, eWord, ePOS, eWPOS, eRel, eMHost, eMSuffix *util.EnumSet) []interface{} {
	graphCorpus := make([]interface{}, len(corpus))
	for i, sent := range corpus {
		// log.Println("Converting sentence", i)
		graphCorpus[i] = ConllU2Graph(sent, eWord, ePOS, eWPOS, eRel, eMHost, eMSuffix)
	}
	return graphCorpus
}

func MorphGraph2ConllU(graph nlp.MorphDependencyGraph) *Sentence {
	sent := NewSentence()
	arcIndex := make(map[int]nlp.LabeledDepArc, graph.NumberOfNodes())
	var (
		node   *nlp.EMorpheme
		arc    nlp.LabeledDepArc
		headID int
		depRel string
	)
	for _, arcID := range graph.GetEdges() {
		arc = graph.GetLabeledArc(arcID)
		if arc == nil {
			// panic("Can't find arc")
			// log.Println("Can't find arc", arcID)
		} else {
			arcIndex[arc.GetModifier()] = arc
		}
	}
	for i, nodeID := range graph.GetVertices() {
		node = graph.GetMorpheme(nodeID)

		if node == nil {
			panic("Can't find node")
		}

		arc, exists := arcIndex[i]
		if exists {
			headID = arc.GetHead()
			depRel = string(arc.GetRelation())
			if depRel == nlp.ROOT_LABEL {
				headID = -1
			}
		} else {
			headID = 0
			depRel = "None"
		}
		row := Row{
			ID:      i + 1,
			Form:    node.Form,
			UPosTag: node.CPOS,
			XPosTag: node.POS,
			Feats:   node.Features,
			Head:    headID + 1,
			DepRel:  depRel,
		}
		sent.Deps[row.ID] = row
	}
	return sent
}

func MorphGraph2ConllCorpus(corpus []interface{}) []interface{} {
	sentCorpus := make([]interface{}, len(corpus))
	for i, graph := range corpus {
		sentCorpus[i] = MorphGraph2ConllU(graph.(nlp.MorphDependencyGraph))
	}
	return sentCorpus
}