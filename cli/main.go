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
  upload <path>                       POST /upload
  self_ip                             locally-determined outbound IP toward the host`

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
		requireArgs(3, "upload <path>")
		upload(host+"/upload", os.Args[2])
	case "self_ip":
		selfIP(host)
	default:
		fmt.Fprintf(os.Stderr, colorRed+"unknown command: %s\n"+colorReset, os.Args[1])
		fmt.Fprintln(os.Stderr, colorYellow+usage+colorReset)
		os.Exit(1)
	}
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
func upload(url, filePath string) {
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"could not open file:"+colorReset, err)
		os.Exit(1)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
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
