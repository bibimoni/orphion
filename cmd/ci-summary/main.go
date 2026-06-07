package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
}

type testStats struct {
	Passed        int
	Failed        int
	Skipped       int
	PackageFailed bool
	FailedTests   []string
}

type coverageStats struct {
	Covered int
	Total   int
}

func main() {
	testPath := flag.String("test-json", "", "path to go test -json output")
	coveragePath := flag.String("coverage", "", "path to Go coverage profile")
	outputPath := flag.String("output", os.Getenv("GITHUB_STEP_SUMMARY"), "summary output path")
	flag.Parse()

	if *testPath == "" || *coveragePath == "" {
		fmt.Fprintln(os.Stderr, "ci-summary: -test-json and -coverage are required")
		os.Exit(2)
	}
	tests, err := os.Open(*testPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ci-summary:", err)
		os.Exit(1)
	}
	defer func() {
		_ = tests.Close()
	}()
	coverage, err := os.Open(*coveragePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ci-summary:", err)
		os.Exit(1)
	}
	defer func() {
		_ = coverage.Close()
	}()

	summary, err := buildSummary(tests, coverage)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ci-summary:", err)
		os.Exit(1)
	}
	if *outputPath == "" {
		fmt.Print(summary)
		return
	}
	output, err := os.OpenFile(*outputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ci-summary:", err)
		os.Exit(1)
	}
	if _, err := io.WriteString(output, summary); err != nil {
		_ = output.Close()
		fmt.Fprintln(os.Stderr, "ci-summary:", err)
		os.Exit(1)
	}
	if err := output.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "ci-summary:", err)
		os.Exit(1)
	}
}

func buildSummary(testJSON, coverage io.Reader) (string, error) {
	tests, err := parseTests(testJSON)
	if err != nil {
		return "", err
	}
	packages, err := parseCoverage(coverage)
	if err != nil {
		return "", err
	}

	result := "Passed"
	if tests.Failed > 0 || tests.PackageFailed {
		result = "Failed"
	}
	var out strings.Builder
	fmt.Fprintln(&out, "# Test & Coverage Summary")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "**Result:** %s\n\n", result)
	fmt.Fprintln(&out, "| Tests | Count |")
	fmt.Fprintln(&out, "|---|---:|")
	fmt.Fprintf(&out, "| Passed | %d |\n", tests.Passed)
	fmt.Fprintf(&out, "| Failed | %d |\n", tests.Failed)
	fmt.Fprintf(&out, "| Skipped | %d |\n\n", tests.Skipped)

	if len(tests.FailedTests) > 0 {
		fmt.Fprintln(&out, "## Failed Tests")
		fmt.Fprintln(&out)
		for _, name := range tests.FailedTests {
			fmt.Fprintf(&out, "- `%s`\n", escapeMarkdown(name))
		}
		fmt.Fprintln(&out)
	}

	fmt.Fprintln(&out, "## Coverage")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "| Package | Statements |")
	fmt.Fprintln(&out, "|---|---:|")
	names := make([]string, 0, len(packages))
	total := coverageStats{}
	for name, stats := range packages {
		names = append(names, name)
		total.Covered += stats.Covered
		total.Total += stats.Total
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&out, "| `%s` | %.1f%% |\n", escapeMarkdown(name), percentage(packages[name]))
	}
	fmt.Fprintf(&out, "\n**Total coverage:** %.1f%%\n", percentage(total))
	return out.String(), nil
}

func parseTests(input io.Reader) (testStats, error) {
	var stats testStats
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event testEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return stats, fmt.Errorf("decode test event: %w", err)
		}
		if event.Test == "" {
			if event.Action == "fail" && event.Package != "" {
				stats.PackageFailed = true
			}
			continue
		}
		switch event.Action {
		case "pass":
			stats.Passed++
		case "fail":
			stats.Failed++
			stats.FailedTests = append(stats.FailedTests, event.Package+"."+event.Test)
		case "skip":
			stats.Skipped++
		}
	}
	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("read test events: %w", err)
	}
	sort.Strings(stats.FailedTests)
	return stats, nil
}

func parseCoverage(input io.Reader) (map[string]coverageStats, error) {
	result := make(map[string]coverageStats)
	scanner := bufio.NewScanner(input)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			if !strings.HasPrefix(line, "mode: ") {
				return nil, errors.New("coverage profile is missing mode")
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid coverage record %q", line)
		}
		statements, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("invalid statement count: %w", err)
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid execution count: %w", err)
		}
		filename := strings.SplitN(fields[0], ":", 2)[0]
		packageName := path.Dir(filename)
		stats := result[packageName]
		stats.Total += statements
		if count > 0 {
			stats.Covered += statements
		}
		result[packageName] = stats
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read coverage profile: %w", err)
	}
	if first {
		return nil, errors.New("coverage profile is empty")
	}
	return result, nil
}

func percentage(stats coverageStats) float64 {
	if stats.Total == 0 {
		return 0
	}
	return float64(stats.Covered) * 100 / float64(stats.Total)
}

func escapeMarkdown(value string) string {
	value = html.EscapeString(value)
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.ReplaceAll(value, "`", "\\`")
	return value
}
