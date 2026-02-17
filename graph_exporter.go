// canvas_tool: convert Obsidian .canvas (JSON) to semicolon-separated CSV triples: from;label;to
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Canvas struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Text  string `json:"text"`
	File  string `json:"file"`
	URL   string `json:"url"`
	Label string `json:"label"`
}

type Edge struct {
	FromNode string `json:"fromNode"`
	ToNode   string `json:"toNode"`
	Label    string `json:"label"`
	Text     string `json:"text"` // some exports use "text" instead of "label"
}

func main() {
	inPath := flag.String("in", "", "input .canvas path (or - for stdin)")
	outPath := flag.String("out", "", "output .csv path (or - for stdout). Default: input basename + .csv")
	keepPath := flag.Bool("keep-path", false, "for file nodes, keep full path instead of base name")
	flag.Parse()

	if *inPath == "" && flag.NArg() > 0 {
		*inPath = flag.Arg(0)
	}
	if *inPath == "" {
		fatalf("missing -in (or first arg)")
	}

	if *outPath == "" {
		if *inPath == "-" {
			*outPath = "-"
		} else {
			base := strings.TrimSuffix(filepath.Base(*inPath), filepath.Ext(*inPath))
			*outPath = base + ".csv"
		}
	}

	in, closeIn, err := openIn(*inPath)
	if err != nil {
		fatalf("open input: %v", err)
	}
	defer closeIn()

	data, err := io.ReadAll(in)
	if err != nil {
		fatalf("read input: %v", err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // optional UTF-8 BOM

	var c Canvas
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		// fall back to lenient decode (Obsidian may add fields)
		if err2 := json.Unmarshal(data, &c); err2 != nil {
			fatalf("parse .canvas JSON: %v", err)
		}
	}

	nodeByID := make(map[string]Node, len(c.Nodes))
	for _, n := range c.Nodes {
		nodeByID[n.ID] = n
	}

	out, closeOut, err := openOut(*outPath)
	if err != nil {
		fatalf("open output: %v", err)
	}
	defer func() {
		if err := closeOut(); err != nil {
			fatalf("close output: %v", err)
		}
	}()

	w := csv.NewWriter(out)
	w.Comma = ';'
	w.UseCRLF = false

	for _, e := range c.Edges {
		from := nodeDisplay(nodeByID[e.FromNode], *keepPath)
		to := nodeDisplay(nodeByID[e.ToNode], *keepPath)

		label := e.Label
		if label == "" {
			label = e.Text
		}

		from = singleLine(from)
		label = singleLine(label)
		to = singleLine(to)

		if err := w.Write([]string{from, label, to}); err != nil {
			fatalf("write csv: %v", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		fatalf("flush csv: %v", err)
	}
}

func nodeDisplay(n Node, keepPath bool) string {
	if n.ID == "" && n.Type == "" && n.Text == "" && n.File == "" && n.URL == "" && n.Label == "" {
		return ""
	}
	switch {
	case n.Text != "":
		return n.Text
	case n.Label != "":
		return n.Label
	case n.File != "":
		if keepPath {
			return n.File
		}
		return filepath.Base(n.File)
	case n.URL != "":
		return n.URL
	case n.ID != "":
		return n.ID
	default:
		return ""
	}
}

func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func openIn(path string) (io.Reader, func() error, error) {
	if path == "-" {
		return os.Stdin, func() error { return nil }, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func openOut(path string) (io.Writer, func() error, error) {
	if path == "-" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "canvas_tool: "+format+"\n", args...)
	os.Exit(1)
}