package main

import (
	"strings"
	"testing"
)

func TestBuildSummaryAggregatesTestsAndCoverage(t *testing.T) {
	testJSON := strings.NewReader(`
{"Action":"pass","Package":"example/a","Test":"TestOne","Elapsed":0.01}
{"Action":"skip","Package":"example/a","Test":"TestSkipped","Elapsed":0}
{"Action":"pass","Package":"example/a","Elapsed":0.02}
{"Action":"fail","Package":"example/b","Test":"TestBroken","Elapsed":0.03}
{"Action":"fail","Package":"example/b","Elapsed":0.04}
`)
	coverage := strings.NewReader(`mode: atomic
example/a/a.go:1.1,2.2 10 1
example/a/a.go:4.1,5.2 5 0
example/b/b.go:1.1,2.2 5 1
`)

	got, err := buildSummary(testJSON, coverage)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"# Test & Coverage Summary",
		"**Result:** Failed",
		"| Passed | 1 |",
		"| Failed | 1 |",
		"| Skipped | 1 |",
		"| `example/a` | 66.7% |",
		"| `example/b` | 100.0% |",
		"**Total coverage:** 75.0%",
		"`example/b.TestBroken`",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary missing %q:\n%s", want, got)
		}
	}
}

func TestBuildSummaryEscapesMarkdown(t *testing.T) {
	testJSON := strings.NewReader(`
{"Action":"fail","Package":"example/a|b","Test":"Test&lt;Bad&gt;"}
`)
	coverage := strings.NewReader("mode: atomic\n")

	got, err := buildSummary(testJSON, coverage)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "example/a|b") {
		t.Fatalf("pipe was not escaped:\n%s", got)
	}
	if !strings.Contains(got, `example/a\|b.Test&amp;lt;Bad&amp;gt;`) {
		t.Fatalf("summary was not escaped:\n%s", got)
	}
}

func TestBuildSummaryRejectsMalformedInput(t *testing.T) {
	_, err := buildSummary(strings.NewReader("{bad json}\n"), strings.NewReader("mode: atomic\n"))
	if err == nil {
		t.Fatal("buildSummary() error = nil")
	}
}

func TestBuildSummaryReportsPackageFailureWithoutTestEvents(t *testing.T) {
	testJSON := strings.NewReader(`
{"Action":"fail","Package":"example/broken","Elapsed":0.01}
`)

	got, err := buildSummary(testJSON, strings.NewReader("mode: atomic\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "**Result:** Failed") {
		t.Fatalf("summary did not report package failure:\n%s", got)
	}
}
