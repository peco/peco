// filterbench measures the query performance of peco's filter implementations
// against synthetically generated datasets of configurable size.
//
// Usage:
//
//	go run ./cmd/filterbench [flags]
//
// Examples:
//
//	go run ./cmd/filterbench -lines 1000000 -query "foo"
//	go run ./cmd/filterbench -lines 500000 -query "foo bar" -filter IgnoreCase
//	go run ./cmd/filterbench -lines 1000000 -query "f" -incremental "fo,foo,foob,fooba,foobar"
//	go run ./cmd/filterbench -lines 1000000 -pipeline -filter IgnoreCase -query "foo"
//	go run ./cmd/filterbench -lines 1000000 -input /path/to/largefile.txt
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"runtime"
	"strings"
	"time"

	peco "github.com/peco/peco"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

type result struct {
	Filter      string  `json:"filter"`
	Query       string  `json:"query"`
	Lines       int     `json:"lines"`
	Matches     int     `json:"matches"`
	Duration    string  `json:"duration"`
	DurationMs  float64 `json:"duration_ms"`
	LinesPerSec float64 `json:"lines_per_sec"`
	Mode        string  `json:"mode"` // "direct", "pipeline", "FULL", "INCR"
}

type benchConfig struct {
	numLines    int
	query       string
	incremental string
	filterName  string
	inputFile   string
	jsonOutput  bool
	seed        uint64
	lineLen     int
	usePipeline bool
}

func main() {
	var cfg benchConfig

	flag.IntVar(&cfg.numLines, "lines", 1_000_000, "number of lines to generate")
	flag.StringVar(&cfg.query, "query", "foo", "query string to filter with")
	flag.StringVar(&cfg.incremental, "incremental", "", "comma-separated queries to run in sequence (simulates typing)")
	flag.StringVar(&cfg.filterName, "filter", "", "filter to test (empty = all filters)")
	flag.StringVar(&cfg.inputFile, "input", "", "read lines from file instead of generating them")
	flag.BoolVar(&cfg.jsonOutput, "json", false, "output results as JSON")
	flag.Uint64Var(&cfg.seed, "seed", 42, "random seed for data generation")
	flag.IntVar(&cfg.lineLen, "line-length", 80, "average length of generated lines")
	flag.BoolVar(&cfg.usePipeline, "pipeline", false, "use full pipeline (Source -> filterProcessor -> MemoryBuffer) instead of direct Apply")
	flag.Parse()

	lines := loadOrGenerate(cfg)

	filters := buildFilters(cfg.filterName)
	if len(filters) == 0 {
		fmt.Fprintf(os.Stderr, "unknown filter: %s\n", cfg.filterName)
		fmt.Fprintf(os.Stderr, "available: IgnoreCase, CaseSensitive, SmartCase, Regexp, IRegexp, Fuzzy, FuzzyLongest\n")
		os.Exit(1)
	}

	queries := buildQueries(cfg)

	if !cfg.jsonOutput {
		fmt.Fprintf(os.Stderr, "Dataset: %d lines\n", len(lines))
		fmt.Fprintf(os.Stderr, "Filters: %d\n", len(filters))
		fmt.Fprintf(os.Stderr, "Queries: %v\n", queries)
		fmt.Fprintf(os.Stderr, "GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
		fmt.Fprintf(os.Stderr, "Pipeline: %v\n\n", cfg.usePipeline)
	}

	var results []result

	if cfg.incremental != "" {
		for name, f := range filters {
			results = append(results, benchIncremental(name, f, lines, queries, cfg)...)
		}
	} else {
		for name, f := range filters {
			for _, q := range queries {
				var r result
				if cfg.usePipeline {
					r = benchPipeline(name, f, lines, q)
				} else {
					r = benchDirect(name, f, lines, q)
				}
				results = append(results, r)
				if !cfg.jsonOutput {
					printResult(r)
				}
			}
		}
	}

	if cfg.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
	}
}

func buildQueries(cfg benchConfig) []string {
	if cfg.incremental != "" {
		return strings.Split(cfg.incremental, ",")
	}
	return []string{cfg.query}
}

func buildFilters(name string) map[string]filter.Filter {
	all := map[string]filter.Filter{
		"IgnoreCase":    filter.NewIgnoreCase(),
		"CaseSensitive": filter.NewCaseSensitive(),
		"SmartCase":     filter.NewSmartCase(),
		"Regexp":        filter.NewRegexp(),
		"IRegexp":       filter.NewIRegexp(),
		"Fuzzy":         filter.NewFuzzy(false),
		"FuzzyLongest":  filter.NewFuzzy(true),
	}

	if name == "" {
		return all
	}

	if f, ok := all[name]; ok {
		return map[string]filter.Filter{name: f}
	}
	return nil
}

func loadOrGenerate(cfg benchConfig) []line.Line {
	if cfg.inputFile != "" {
		return loadFromFile(cfg.inputFile)
	}
	return generateLines(cfg)
}

func loadFromFile(path string) []line.Line {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	var lines []line.Line
	scanner := bufio.NewScanner(f)
	// Allow long lines
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	var id uint64
	for scanner.Scan() {
		lines = append(lines, line.NewRaw(id, scanner.Text(), false))
		id++
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
		os.Exit(1)
	}
	return lines
}

// generateLines creates a synthetic dataset. The data is designed so that:
//   - Common short queries (e.g. "f", "fo") match many lines
//   - Longer queries (e.g. "foobar") match progressively fewer lines
//   - Lines have realistic variety in length and content
func generateLines(cfg benchConfig) []line.Line {
	rng := rand.New(rand.NewPCG(cfg.seed, cfg.seed+1))

	// Words that will be scattered through the corpus.
	// "foo", "bar", "baz" appear frequently so short queries hit many lines.
	// Longer compound words like "foobar" appear rarely.
	words := []struct {
		word string
		freq float64 // probability of appearing in a line
	}{
		{"foo", 0.30},
		{"bar", 0.25},
		{"baz", 0.15},
		{"qux", 0.10},
		{"foobar", 0.03},
		{"foobaz", 0.02},
		{"foobarbaz", 0.005},
	}

	// Base character set for random filler
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789_-./: "

	lines := make([]line.Line, cfg.numLines)
	for i := range lines {
		var sb strings.Builder
		targetLen := cfg.lineLen/2 + rng.IntN(cfg.lineLen)

		for sb.Len() < targetLen {
			// Randomly insert a known word
			for _, w := range words {
				if rng.Float64() < w.freq*0.3 { // Scale down per-character check
					if sb.Len() > 0 {
						sb.WriteByte(' ')
					}
					sb.WriteString(w.word)
				}
			}
			// Add random filler
			n := 3 + rng.IntN(8)
			for j := 0; j < n && sb.Len() < targetLen; j++ {
				sb.WriteByte(alphabet[rng.IntN(len(alphabet))])
			}
			if sb.Len() < targetLen {
				sb.WriteByte(' ')
			}
		}

		lines[i] = line.NewRaw(uint64(i), sb.String(), false)
	}

	return lines
}

// benchDirect runs a single filter+query combination using direct Apply() and returns timing results.
func benchDirect(name string, f filter.Filter, lines []line.Line, query string) result {
	ctx := f.NewContext(context.Background(), query)

	// Count matches using a channel consumer
	matchCount := 0
	ch := make(chan interface{}, 4096)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for range ch {
			matchCount++
		}
	}()

	out := pipeline.ChanOutput(ch)

	// Force GC before measurement
	runtime.GC()

	start := time.Now()
	err := f.Apply(ctx, lines, out)
	elapsed := time.Since(start)

	close(ch)
	<-done

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s/%s: %v\n", name, query, err)
	}

	lps := float64(len(lines)) / elapsed.Seconds()

	return result{
		Filter:      name,
		Query:       query,
		Lines:       len(lines),
		Matches:     matchCount,
		Duration:    elapsed.String(),
		DurationMs:  float64(elapsed.Milliseconds()),
		LinesPerSec: lps,
		Mode:        "direct",
	}
}

// benchPipeline runs a filter using the full pipeline (Source -> filterProcessor -> MemoryBuffer).
func benchPipeline(name string, f filter.Filter, lines []line.Line, query string) result {
	ctx := f.NewContext(context.Background(), query)

	// Build a MemoryBuffer with all lines to use as source
	srcBuf := peco.NewMemoryBuffer()
	for _, l := range lines {
		func() {
			srcBuf.AppendLine(l)
		}()
	}
	src := peco.NewMemoryBufferSource(srcBuf)

	// Setup pipeline
	p := pipeline.New()
	p.SetSource(src)
	p.Add(&directFilterProcessor{filter: f, query: query})

	dst := peco.NewMemoryBuffer()
	p.SetDestination(dst)

	runtime.GC()
	start := time.Now()

	if err := p.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s/%s: %v\n", name, query, err)
	}

	elapsed := time.Since(start)
	matchCount := dst.Size()
	lps := float64(len(lines)) / elapsed.Seconds()

	return result{
		Filter:      name,
		Query:       query,
		Lines:       len(lines),
		Matches:     matchCount,
		Duration:    elapsed.String(),
		DurationMs:  float64(elapsed.Milliseconds()),
		LinesPerSec: lps,
		Mode:        "pipeline",
	}
}

// isQueryRefinement checks whether new is a refinement of prev.
func isQueryRefinement(prev, new string) bool {
	prev = strings.TrimSpace(prev)
	new = strings.TrimSpace(new)
	if prev == "" || new == "" {
		return false
	}
	return strings.HasPrefix(new, prev)
}

// benchIncremental runs a sequence of queries, using previous results as source when the query is a refinement.
func benchIncremental(name string, f filter.Filter, allLines []line.Line, queries []string, cfg benchConfig) []result {
	var results []result
	var cumulative time.Duration

	var prevQuery string
	var prevResults []line.Line

	for _, q := range queries {
		var sourceLines []line.Line
		var mode string

		// Check if we can use incremental filtering
		if prevResults != nil && isQueryRefinement(prevQuery, q) {
			sourceLines = prevResults
			mode = "INCR"
		} else {
			sourceLines = allLines
			mode = "FULL"
		}

		var r result
		if cfg.usePipeline {
			r = benchPipelineOnLines(name, f, sourceLines, q)
		} else {
			r = benchDirect(name, f, sourceLines, q)
		}
		r.Mode = mode
		cumulative += time.Duration(r.DurationMs * float64(time.Millisecond))

		if !cfg.jsonOutput {
			fmt.Printf("[%s] ", mode)
			printResult(r)
			fmt.Printf("  source lines: %d  cumulative: %s\n", len(sourceLines), cumulative)
		}

		results = append(results, r)

		// Save results for next iteration
		prevQuery = q
		prevResults = collectMatchedLines(f, sourceLines, q)
	}

	if !cfg.jsonOutput {
		fmt.Printf("\nCumulative time: %s\n\n", cumulative)
	}

	return results
}

// collectMatchedLines runs the filter and returns matched lines as a slice.
func collectMatchedLines(f filter.Filter, lines []line.Line, query string) []line.Line {
	ctx := f.NewContext(context.Background(), query)
	ch := make(chan interface{}, 4096)
	done := make(chan struct{})

	var matched []line.Line
	go func() {
		defer close(done)
		for v := range ch {
			if l, ok := v.(line.Line); ok {
				matched = append(matched, l)
			}
		}
	}()

	f.Apply(ctx, lines, pipeline.ChanOutput(ch))
	close(ch)
	<-done
	return matched
}

// benchPipelineOnLines runs a filter through the pipeline on a specific set of lines.
func benchPipelineOnLines(name string, f filter.Filter, lines []line.Line, query string) result {
	ctx := f.NewContext(context.Background(), query)

	srcBuf := peco.NewMemoryBuffer()
	for _, l := range lines {
		srcBuf.AppendLine(l)
	}
	src := peco.NewMemoryBufferSource(srcBuf)

	p := pipeline.New()
	p.SetSource(src)
	p.Add(&directFilterProcessor{filter: f, query: query})

	dst := peco.NewMemoryBuffer()
	p.SetDestination(dst)

	runtime.GC()
	start := time.Now()

	if err := p.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s/%s: %v\n", name, query, err)
	}

	elapsed := time.Since(start)
	matchCount := dst.Size()
	lps := float64(len(lines)) / elapsed.Seconds()

	return result{
		Filter:      name,
		Query:       query,
		Lines:       len(lines),
		Matches:     matchCount,
		Duration:    elapsed.String(),
		DurationMs:  float64(elapsed.Milliseconds()),
		LinesPerSec: lps,
	}
}

// directFilterProcessor wraps a filter.Filter for use in a pipeline, using the
// same acceptAndFilter mechanism as peco's filterProcessor.
type directFilterProcessor struct {
	filter filter.Filter
	query  string
}

func (p *directFilterProcessor) Accept(ctx context.Context, in chan interface{}, out pipeline.ChanOutput) {
	peco.AcceptAndFilter(ctx, p.filter, 0, in, out)
}

func printResult(r result) {
	fmt.Printf("%-16s  query=%-20s  %d lines  %6d matches  %10s  (%.0f lines/sec)\n",
		r.Filter, fmt.Sprintf("%q", r.Query), r.Lines, r.Matches, r.Duration, r.LinesPerSec)
}
