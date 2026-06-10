package main

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── ANSI color codes ──────────────────────────────────────────────────────────

const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"
	bgBlue  = "\033[44m"
	bgCyan  = "\033[46m"
)

// ── Random data pools ─────────────────────────────────────────────────────────

var (
	firstNames  = []string{"Oliver", "Phoebe", "Marcus", "Ingrid", "Tariq", "Yuki", "Soren", "Amara", "Declan", "Priya"}
	lastNames   = []string{"Nakamura", "Osei", "Lindqvist", "Ferrara", "Patel", "Kowalski", "Okafor", "Reyes", "Svensson", "Mbeki"}
	employers   = []string{"Horizon Robotics", "Starfall Media", "Ironclad Materials", "Vivant Health", "Obsidian Logistics", "Luminary Tech", "Verdant Farms", "Nexus Analytics", "Solaris Energy", "Phalanx Security"}
	adjusters   = []string{"T. Hargrove", "M. Delacroix", "A. Fujimoto", "R. Oduya", "S. Bergmann", "C. Abramowitz", "D. Kazakov", "F. Osei-Mensah", "L. Cartwright", "P. Iyer"}
	claimTypes  = []string{"Workers Comp", "Liability", "Property", "Medical", "Disability", "Auto", "Product Liability", "Environmental"}
	supportLvls = []string{"Full Support", "Partial", "Minimal", "Psychiatric", "Physical Therapy", "None", "Pending Review"}
	jurisdicts  = []string{"California", "New York", "Texas", "Florida", "Illinois", "Washington", "Colorado", "Georgia", "Ohio", "Michigan"}
	filePairs   = []struct{ path, label string }{
		{"two.png", "two.png"},
		{"lenna.jpg", "lenna.jpg"},
		{"test.pdf", "test.pdf"},
		{"CrossDocTesting", "CrossDocTesting"},
	}
	statusMessages = []string{
		"Dispatching request…",
		"Knocking on the server door…",
		"Firing off multipart form…",
		"Launching claim into the void…",
		"Sending bytes across the wire…",
	}
)

func rnd(pool []string) string { return pool[rand.Intn(len(pool))] }

func randDate() string {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	days := rand.Intn(730)
	return start.AddDate(0, 0, days).Format("2006-01-02")
}

func randClaimNum() string { return fmt.Sprintf("CLM-%05d", rand.Intn(90000)+10000) }
func randPolicyNum() string {
	return fmt.Sprintf("POL-%06d", rand.Intn(900000)+100000)
}
func randActsID(i int) string { return fmt.Sprintf("ACTS_%03d", i) }

// ── Pretty printing helpers ───────────────────────────────────────────────────

func header() {
	w := 60
	line := strings.Repeat("─", w)
	fmt.Printf("\n%s%s╔%s╗%s\n", bold, cyan, strings.Repeat("═", w), reset)
	fmt.Printf("%s%s║%s  %-*s%s%s║%s\n", bold, cyan, white, w-2, "  🧪  CLAIM UPLOAD TEST SUITE", cyan, bold, reset)
	fmt.Printf("%s%s╚%s╝%s\n\n", bold, cyan, strings.Repeat("═", w), reset)
	_ = line
}

func sectionBanner(n int, total int, label string) {
	bar := fmt.Sprintf("[%d/%d]", n, total)
	fmt.Printf("%s%s  %s %-45s%s\n", bold, yellow, bar, label, reset)
	fmt.Printf("%s%s%s\n", dim, strings.Repeat("·", 60), reset)
}

func fieldLine(key, value string) {
	fmt.Printf("  %s%-20s%s %s%s%s\n", dim, key, reset, cyan, value, reset)
}

func statusLine(msg string) {
	fmt.Printf("\n  %s⟳  %s%s\n", magenta, msg, reset)
}

func successLine(code int, took time.Duration) {
	color := green
	icon := "✓"
	if code >= 400 {
		color = red
		icon = "✗"
	} else if code >= 300 {
		color = yellow
		icon = "↪"
	}
	fmt.Printf("  %s%s%s  HTTP %s%d%s  %s(%s)%s\n\n",
		bold, color, icon, bold, code, reset, dim, took.Round(time.Millisecond), reset)
}

func errorLine(err error) {
	fmt.Printf("  %s✗  ERROR: %v%s\n\n", red, err, reset)
}

func separator() {
	fmt.Printf("%s%s%s\n", dim, strings.Repeat("─", 60), reset)
}

func summary(passed, failed int, total time.Duration) {
	fmt.Printf("\n%s%s╔%s╗%s\n", bold, cyan, strings.Repeat("═", 60), reset)
	fmt.Printf("%s%s║%s  %-58s%s%s║%s\n", bold, cyan, white, "  RESULTS", cyan, bold, reset)
	fmt.Printf("%s%s╠%s╣%s\n", bold, cyan, strings.Repeat("═", 60), reset)
	fmt.Printf("%s%s║%s  %-58s%s%s║%s\n", bold, cyan, green,
		fmt.Sprintf("  ✓  Passed : %d", passed), cyan, bold, reset)
	fmt.Printf("%s%s║%s  %-58s%s%s║%s\n", bold, cyan, red,
		fmt.Sprintf("  ✗  Failed : %d", failed), cyan, bold, reset)
	fmt.Printf("%s%s║%s  %-58s%s%s║%s\n", bold, cyan, dim,
		fmt.Sprintf("  ⏱  Total  : %s", total.Round(time.Millisecond)), cyan, bold, reset)
	fmt.Printf("%s%s╚%s╝%s\n\n", bold, cyan, strings.Repeat("═", 60), reset)
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func doGET(url string) (int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func doUpload(url string, fields map[string]string, filePath string) (int, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return 0, err
		}
	}

	// file field (skip if path empty or file missing)
	if filePath != "" {
		f, err := os.Open(filePath)
		if err == nil {
			defer f.Close()
			fw, err := w.CreateFormFile("file", filepath.Base(filePath))
			if err != nil {
				return 0, err
			}
			io.Copy(fw, f)
		}
		// if file doesn't exist we just skip it — testing infra may not have them
	}

	w.Close()

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// ── Test cases ────────────────────────────────────────────────────────────────

type TestCase struct {
	label  string
	run    func() (int, error)
	fields map[string]string
}

func buildCases(base string) []TestCase {
	cases := make([]TestCase, 0, len(filePairs)+1)

	// GET health check
	cases = append(cases, TestCase{
		label:  "GET /  (health check)",
		fields: map[string]string{"url": base + "/"},
		run:    func() (int, error) { return doGET(base + "/") },
	})

	// One upload per file
	for i, fp := range filePairs {
		fp := fp // capture
		idx := i + 1
		fname := fmt.Sprintf("%s %s", rnd(firstNames), rnd(lastNames))
		fields := map[string]string{
			"claim_number":    randClaimNum(),
			"claimant_name":   fname,
			"date_of_injury":  randDate(),
			"employer":        rnd(employers),
			"adjuster":        rnd(adjusters),
			"support":         rnd(supportLvls),
			"claim_type":      rnd(claimTypes),
			"jurisdiction":    rnd(jurisdicts),
			"policy_number":   randPolicyNum(),
			"acts_id":         randActsID(idx),
			"data":            fmt.Sprintf("run-%d-data-%04x", idx, rand.Intn(0xffff)),
		}
		fCopy := make(map[string]string, len(fields))
		for k, v := range fields {
			fCopy[k] = v
		}
		cases = append(cases, TestCase{
			label:  fmt.Sprintf("POST /upload  [%s]", fp.label),
			fields: fCopy,
			run: func() (int, error) {
				return doUpload(base+"/upload", fCopy, fp.path)
			},
		})
	}
	return cases
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	rand.Seed(time.Now().UnixNano())

	base := "http://localhost:8080"
	if len(os.Args) > 1 {
		base = strings.TrimRight(os.Args[1], "/")
	}

	header()
	fmt.Printf("  %sTarget:%s %s%s%s\n\n", dim, reset, bold, base, reset)

	cases := buildCases(base)
	total := len(cases)
	passed, failed := 0, 0
	start := time.Now()

	for i, tc := range cases {
		sectionBanner(i+1, total, tc.label)

		// print fields (skip internal ones)
		skip := map[string]bool{"url": true}
		for _, k := range []string{
			"claim_number", "claimant_name", "date_of_injury", "employer",
			"adjuster", "support", "claim_type", "jurisdiction",
			"policy_number", "acts_id", "data",
		} {
			if v, ok := tc.fields[k]; ok && !skip[k] {
				fieldLine(k+":", v)
			}
		}

		statusLine(rnd(statusMessages))

		t0 := time.Now()
		code, err := tc.run()
		elapsed := time.Since(t0)

		if err != nil {
			errorLine(err)
			failed++
		} else {
			successLine(code, elapsed)
			if code < 400 {
				passed++
			} else {
				failed++
			}
		}

		separator()
		time.Sleep(400 * time.Millisecond)
	}

	summary(passed, failed, time.Since(start))
}
