package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/tabwriter"
)

// filesResponse mirrors the JSON shape of GET /files (backend/routes.go listFiles).
type filesResponse struct {
	Files []string `json:"files"`
	Count int      `json:"count"`
}

// machine mirrors one entry of GET /machines (backend/db.go getKnownMachines).
type machine struct {
	IP       string  `json:"ip"`
	Label    string  `json:"label"`
	AddedAt  string  `json:"added_at"`
	LastSeen *string `json:"last_seen_at"`
}

type machinesResponse struct {
	Machines []machine `json:"machines"`
}

// prettyJSON indents body for display, falling back to the raw body
// unchanged if it isn't valid JSON (e.g. get_file returning file bytes).
func prettyJSON(body []byte) string {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return string(body)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(body)
	}
	return string(out)
}

// printFilesTable renders a GET /files response as an aligned table.
func printFilesTable(body []byte) string {
	var r filesResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return prettyJSON(body)
	}
	if len(r.Files) == 0 {
		return "no files"
	}

	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "FILENAME")
	for _, f := range r.Files {
		fmt.Fprintln(tw, f)
	}
	tw.Flush()

	return fmt.Sprintf("%s\n%d file(s)", buf.String(), r.Count)
}

// printMachinesTable renders a GET /machines response as an aligned table.
func printMachinesTable(body []byte) string {
	var r machinesResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return prettyJSON(body)
	}
	if len(r.Machines) == 0 {
		return "no machines"
	}

	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "IP\tLABEL\tADDED_AT\tLAST_SEEN")
	for _, m := range r.Machines {
		lastSeen := "-"
		if m.LastSeen != nil {
			lastSeen = *m.LastSeen
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", m.IP, m.Label, m.AddedAt, lastSeen)
	}
	tw.Flush()

	return buf.String()
}
