package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
  dump_cache                          GET  /core/admin/dumpCache
  list_machines                       GET  /core/machines
  add_machine <ip> <label>            POST /core/machines
  delete_machine <ip>                 DELETE /core/machines/:ip
  list_files                          GET  /core/files
  get_file <filename>                 GET  /core/files/:filename/false
  get_file_meta <filename>            GET  /core/files/:filename/true
  delete_file <filename>              DELETE /core/files/:filename
  upload <path> [field=value ...]     POST /core/upload (optional metadata fields, see below)
  self_ip                             locally-determined outbound IP toward the host
  soak_test [sim_days] [users] [seconds_per_sim_day]
                                       runs backend/testing_tooling/soak_test.sh (issue #311) inside
                                       the backend container via podman exec — long-running
                                       multi-user traffic simulation, Ctrl-C stops it early

env:
  DEWEY_HOST                          backend base URL, default http://localhost:8080
  DEWEY_FILES_PASSWORD                password for file management routes (upload/list/get/delete)
  DEWEY_MACHINES_PASSWORD             password for machine management routes (list/add/delete)

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
	filesPassword := os.Getenv("DEWEY_FILES_PASSWORD")
	machinesPassword := os.Getenv("DEWEY_MACHINES_PASSWORD")

	switch os.Args[1] {
	case "health":
		get(host+"/", "", nil)
	case "version":
		get(host+"/version", "", nil)
	case "dump_cache":
		get(host+"/core/admin/dumpCache", filesPassword, nil)
	case "list_machines":
		get(host+"/core/machines", machinesPassword, printMachinesTable)
	case "add_machine":
		requireArgs(4, "add_machine <ip> <label>")
		postJSON(host+"/core/machines", machinesPassword, map[string]string{"ip": os.Args[2], "label": os.Args[3]}, nil)
	case "delete_machine":
		requireArgs(3, "delete_machine <ip>")
		del(host+"/core/machines/"+os.Args[2], machinesPassword, nil)
	case "list_files":
		get(host+"/core/files", filesPassword, printFilesTable)
	case "get_file":
		requireArgs(3, "get_file <filename>")
		get(host+"/core/files/"+os.Args[2]+"/false", filesPassword, nil)
	case "get_file_meta":
		requireArgs(3, "get_file_meta <filename>")
		get(host+"/core/files/"+os.Args[2]+"/true", filesPassword, nil)
	case "delete_file":
		requireArgs(3, "delete_file <filename>")
		del(host+"/core/files/"+os.Args[2], filesPassword, nil)
	case "upload":
		requireArgs(3, "upload <path> [field=value ...]")
		meta := parseMetadata(os.Args[3:])
		upload(host+"/core/upload", filesPassword, os.Args[2], meta)
	case "self_ip":
		selfIP(host)
	case "soak_test":
		soakTest(os.Args[2:])
	case "create_docx":
		requireArgs(2, "create_docx <filename>")
		fmt.Println(createDocx(os.Args[2]))
	case "create_exe":
		requireArgs(2, "create_exe <filename>")
		fmt.Println(createExe(os.Args[2]))
	case "create_pdf":
		requireArgs(5, "create_pdf filename withJS withOpenAction")
		withJS, err := strconv.ParseBool(os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"invalid withJS: "+err.Error()+colorReset)
			os.Exit(1)
		}
		withOpenAction, err := strconv.ParseBool(os.Args[4])
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"invalid withOpenAction: "+err.Error()+colorReset)
			os.Exit(1)
		}
		fmt.Println(createPDF(os.Args[2], withJS, withOpenAction))
	case "reset_db":
		if askYesNo("You are about to reset the database. This is high risk action. Are you sure you want to reset the database? There is no rollback...") {
			reset_db()
		} else {
			fmt.Println("Cool cool cool, not resetting.")
		}
	case "clean_slate":
		if askYesNo("You are about to reset the system to a clean slate. This is high risk action. Are you sure you want to reset to a clean slate? There is no rollback...") {
			clean_slate()
		} else {
			fmt.Println("Grovy, not cleaning.")
		}
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

// YesNoPrompt asks yes/no questions using the label.
func YesNoPrompt(label string, def bool) bool {
	choices := "Y/n"
	if !def {
		choices = "y/N"
	}

	r := bufio.NewReader(os.Stdin)
	var s string

	for {
		fmt.Fprintf(os.Stderr, "%s (%s) ", label, choices)
		s, _ = r.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		s = strings.ToLower(s)
		if s == "y" || s == "yes" {
			return true
		}
		if s == "n" || s == "no" {
			return false
		}
	}
}

func reset_db() {
	ok := YesNoPrompt("You are about to reset the database. This is high risk action. Are you sure you want to reset the database? There is no rollback...", true)
	if ok {
		cmd := exec.Command("bash", "-c", "podman volume inspect mysql-data &>/dev/null && podman volume rm mysql-data")
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"reset_db failed:"+colorReset, err)
		}

	} else {
		fmt.Println("Cool cool cool, not resetting.")
	}
}

func clean_slate() {
	ok := YesNoPrompt("Your to nuke all stored data. This is high risk action. Are you sure you want to reset the database? There is no rollback...", true)
	if ok {
		cmd := exec.Command("bash", "-c", `for vol in dewey-store dewey-cache dewey-backup dewey-logs dewey-plugin-scratch; do
  podman volume inspect "$vol" &>/dev/null && podman volume rm "$vol"
done`)
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"reset_db failed:"+colorReset, err)
		}

	} else {
		fmt.Println("Cool cool cool, not resetting.")
	}
}

// soakTest shells out to `podman exec` to run soak_test.sh inside the
// backend container, same pattern as reset_db/clean_slate below. It has to
// run there rather than being reimplemented in Go: soak_test.sh simulates
// multiple distinct users by binding requests to loopback aliases
// (127.0.0.2, 127.0.0.3, ...), which only produces distinct source IPs
// known_machines will see from inside the backend's own network namespace.
//
// BASE is always the container's own localhost — args are whatever the
// caller passed after "soak_test" (sim_days, users, seconds_per_sim_day),
// forwarded positionally to match soak_test.sh's own argument order.
// Output streams live since this runs for minutes to hours, not a single
// request/response like the rest of this CLI.
func soakTest(args []string) {
	cmdArgs := append([]string{"exec", "cross-doc-tool-dev", "./testing_tooling/soak_test.sh", "http://localhost:8080"}, args...)
	cmd := exec.Command("podman", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"soak_test failed:"+colorReset, err)
		os.Exit(1)
	}
}

// doRequest fires req and prints the response body. On success (status <
// 400) it's handed to formatter for display, falling back to indented JSON
// if formatter is nil; error bodies always print as indented JSON in red.
// It's shared by every request-shaped command so output stays consistent
// across GET/POST/DELETE.
func doRequest(req *http.Request, formatter func([]byte) string) {
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

	if resp.StatusCode >= 400 {
		fmt.Println(colorRed + prettyJSON(body) + colorReset)
		return
	}

	if formatter != nil {
		fmt.Println(formatter(body))
		return
	}
	fmt.Println(colorGreen + prettyJSON(body) + colorReset)
}

// setAuthHeader attaches the X-Dewey-Password header when a password was
// configured (issue #332) — commands against the base group (health,
// version) pass "" and skip it, since those routes aren't gated.
func setAuthHeader(req *http.Request, password string) {
	if password != "" {
		req.Header.Set("X-Dewey-Password", password)
	}
}

func get(url string, password string, formatter func([]byte) string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	setAuthHeader(req, password)
	doRequest(req, formatter)
}

func del(url string, password string, formatter func([]byte) string) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"bad request:"+colorReset, err)
		os.Exit(1)
	}
	setAuthHeader(req, password)
	doRequest(req, formatter)
}

func postJSON(url string, password string, payload any, formatter func([]byte) string) {
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
	setAuthHeader(req, password)
	doRequest(req, formatter)
}

// upload sends filePath to the backend's /upload route as multipart/form-data
// under the "file" field, matching what uploadFile (backend/routes.go) reads.
// meta entries are attached as additional form fields (claim_number, etc.) —
// omitted entries are simply absent from the request, same as leaving a form
// field blank in the admin portal's upload dialog.
func upload(url, password, filePath string, meta map[string]string) {
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
	setAuthHeader(req, password)
	doRequest(req, nil)
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

// askYesNo prompts the user and loops until they give a valid y/n answer.
func askYesNo(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/n]: ", prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			os.Exit(1)
		}

		switch strings.ToLower(strings.TrimSpace(input)) {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Println("Please answer y or n.")
		}
	}
}
