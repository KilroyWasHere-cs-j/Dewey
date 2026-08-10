package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const defaultHost = "http://localhost:8080"

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

const usage = `usage: dewey-cli <command> [args]

commands:
  health                              GET  /
  version                             GET  /version
  dump_cache                          GET  /admin/dumpCache
  list_machines                       GET  /machines
  add_machine <ip> <label>            POST /machines
  delete_machine <ip>                 DELETE /machines/:ip
  list_files                          GET  /files
  get_file <filename>                 GET  /files/:filename/false
  get_file_meta <filename>            GET  /files/:filename/true
  delete_file <filename>              DELETE /files/:filename
  upload <path> [field=value ...]     POST /upload (optional metadata fields, see below)
  self_ip                             locally-determined outbound IP toward the host

env:
  DEWEY_HOST                          backend base URL, default http://localhost:8080

upload metadata fields (all optional, order doesn't matter):
  claim_number claimant_name date_of_injury employer adjuster
  support claim_type jurisdiction policy_number acts_id

  example: dewey-cli upload report.pdf claim_number=CL-1 acts_id=A-1`

// uploadMetaFields lists the multipart form fields uploadFile
// (backend/routes.go) reads into MetaData, in the same order the backend
// struct declares them — kept here so the CLI can validate field=value args
// instead of silently dropping a typo'd key.
var uploadMetaFields = []string{
	"claim_number", "claimant_name", "date_of_injury", "employer", "adjuster",
	"support", "claim_type", "jurisdiction", "policy_number", "acts_id",
}

func main() {
	fmt.Print(colorGreen + "Welcome to Dewey CLI\n" + colorReset)

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, colorYellow+usage+colorReset)
		os.Exit(1)
	}

	host := os.Getenv("DEWEY_HOST")
	if host == "" {
		host = defaultHost
	}

	switch os.Args[1] {
	case "health":
		get(host + "/")
	case "version":
		get(host + "/version")
	case "dump_cache":
		get(host + "/admin/dumpCache")
	case "list_machines":
		get(host + "/machines")
	case "add_machine":
		requireArgs(4, "add_machine <ip> <label>")
		postJSON(host+"/machines", map[string]string{"ip": os.Args[2], "label": os.Args[3]})
	case "delete_machine":
		requireArgs(3, "delete_machine <ip>")
		del(host + "/machines/" + os.Args[2])
	case "list_files":
		get(host + "/files")
	case "get_file":
		requireArgs(3, "get_file <filename>")
		get(host + "/files/" + os.Args[2] + "/false")
	case "get_file_meta":
		requireArgs(3, "get_file_meta <filename>")
		get(host + "/files/" + os.Args[2] + "/true")
	case "delete_file":
		requireArgs(3, "delete_file <filename>")
		del(host + "/files/" + os.Args[2])
	case "upload":
		requireArgs(3, "upload <path> [field=value ...]")
		meta := parseMetadata(os.Args[3:])
		upload(host+"/upload", os.Args[2], meta)
	case "self_ip":
		selfIP(host)
	default:
		fmt.Fprintf(os.Stderr, colorRed+"unknown command: %s\n"+colorReset, os.Args[1])
		fmt.Fprintln(os.Stderr, colorYellow+usage+colorReset)
		os.Exit(1)
	}
}

// parseMetadata turns trailing "field=value" CLI args into the metadata map
// upload attaches to the request, rejecting anything that isn't in
// uploadMetaFields so a typo'd field name fails fast instead of being
// silently dropped by the backend's PostForm lookup.
func parseMetadata(args []string) map[string]string {
	valid := make(map[string]struct{}, len(uploadMetaFields))
	for _, k := range uploadMetaFields {
		valid[k] = struct{}{}
	}

	meta := make(map[string]string, len(args))
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, colorRed+"invalid metadata arg %q, expected field=value\n"+colorReset, arg)
			os.Exit(1)
		}
		if _, ok := valid[key]; !ok {
			fmt.Fprintf(os.Stderr, colorRed+"unknown metadata field %q, expected one of: %s\n"+colorReset, key, strings.Join(uploadMetaFields, ", "))
			os.Exit(1)
		}
		meta[key] = value
	}
	return meta
}

func requireArgs(n int, cmdUsage string) {
	if len(os.Args) < n {
		fmt.Fprintln(os.Stderr, colorYellow+"usage: dewey-cli "+cmdUsage+colorReset)
		os.Exit(1)
	}
}

// doRequest fires req and prints the response body, colored green for a
// success status and red otherwise. It's shared by every request-shaped
// command so status coloring stays consistent across GET/POST/DELETE.
func doRequest(req *http.Request) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"request failed:"+colorReset, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"reading response failed:"+colorReset, err)
		os.Exit(1)
	}

	color := colorGreen
	if resp.StatusCode >= 400 {
		color = colorRed
	}
	fmt.Println(color + string(body) + colorReset)
}

func get(url string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	doRequest(req)
}

func del(url string) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	doRequest(req)
}

func postJSON(url string, payload any) {
	buf, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to encode request body:"+colorReset, err)
		os.Exit(1)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	doRequest(req)
}

// upload sends filePath to the backend's /upload route as multipart/form-data
// under the "file" field, matching what uploadFile (backend/routes.go) reads.
// meta entries are attached as additional form fields (claim_number, etc.) —
// omitted entries are simply absent from the request, same as leaving a form
// field blank in the admin portal's upload dialog.
func upload(url, filePath string, meta map[string]string) {
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"could not open file:"+colorReset, err)
		os.Exit(1)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, key := range uploadMetaFields {
		if value, ok := meta[key]; ok {
			if err := w.WriteField(key, value); err != nil {
				fmt.Fprintln(os.Stderr, colorRed+"failed to build upload:"+colorReset, err)
				os.Exit(1)
			}
		}
	}

	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to build upload:"+colorReset, err)
		os.Exit(1)
	}
	if _, err := io.Copy(part, f); err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to read file:"+colorReset, err)
		os.Exit(1)
	}
	if err := w.Close(); err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to build upload:"+colorReset, err)
		os.Exit(1)
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	doRequest(req)
}

// selfIP reports the local address the OS would use to reach host. A UDP
// "connect" never sends a packet - it just asks the OS to resolve the route -
// so this is the closest client-side guess at "what IP am I calling from".
// It can still differ from what the server actually sees (e.g. Podman's
// rootless port-forwarding NAT rewrites the source address in transit), so
// treat this as a starting point when deciding what to register in the
// backend's known_machines allowlist, not a guarantee.
func selfIP(host string) {
	u, err := url.Parse(host)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"invalid host:"+colorReset, err)
		os.Exit(1)
	}

	target := u.Host
	if !strings.Contains(target, ":") {
		target += ":80"
	}

	conn, err := net.Dial("udp", target)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"could not determine local IP:"+colorReset, err)
		os.Exit(1)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	fmt.Println(colorCyan + localAddr.IP.String() + colorReset)
}
