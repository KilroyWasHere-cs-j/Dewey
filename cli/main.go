package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

func main() {
	fmt.Print(colorGreen + "Welcome to Dewey CLI\n" + colorReset)

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, colorYellow+usage+colorReset)
		os.Exit(4)
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
		safePath, err := buildSafePath(host, "core", "machines", os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"invalid host: "+err.Error()+colorReset)
			os.Exit(1)
		}
		del(safePath, machinesPassword, nil)
	case "list_files":
		get(host+"/core/files", filesPassword, printFilesTable)
	case "get_file":
		requireArgs(3, "get_file <filename>")
		safePath, err := buildSafePath(host, "core", "files", os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"invalid host: "+err.Error()+colorReset)
			os.Exit(1)
		}
		get(safePath+"?meta=false", filesPassword, nil)
	case "get_file_meta":
		requireArgs(3, "get_file_meta <filename>")
		safePath, err := buildSafePath(host, "core", "files", os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"invalid host: "+err.Error()+colorReset)
			os.Exit(1)
		}
		get(safePath+"?meta=true", filesPassword, nil)
	case "delete_file":
		requireArgs(3, "delete_file <filename>")
		safePath, err := buildSafePath(host, "core", "files", os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"invalid host: "+err.Error()+colorReset)
			os.Exit(1)
		}
		del(safePath, filesPassword, nil)
	case "undelete_file":
		requireArgs(3, "undelete_file <filename>")
		safePath, err := buildSafePath(host, "core", "files", "undelete", os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"invalid host: "+err.Error()+colorReset)
			os.Exit(1)
		}
		postJSON(safePath, filesPassword, nil, nil)
	case "refilter_file":
		requireArgs(3, "refilter_file <filename>")
		safePath, err := buildSafePath(host, "core", "files", "refilter", os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+"invalid host: "+err.Error()+colorReset)
			os.Exit(1)
		}
		postJSON(safePath, filesPassword, nil, nil)
	case "upload":
		requireArgs(3, "upload <path> [field=value ...]")
		meta := parseMetadata(os.Args[3:])
		upload(host+"/core/upload", filesPassword, os.Args[2], meta)
	case "self_ip":
		selfIP(host)
	case "metrics":
		runMetricsDashboard(host)
	case "soak_test":
		soakTest(os.Args[2:])
	case "create_docx":
		requireArgs(3, "create_docx <filename>")
		fmt.Println(createDocx(os.Args[2]))
	case "create_exe":
		requireArgs(3, "create_exe <filename>")
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
		if YesNoPrompt("You are about to reset the database. This is high risk action. Are you sure you want to reset the database? There is no rollback...", false) {
			reset_db()
		} else {
			fmt.Println("Cool cool cool, not resetting.")
		}
	case "clean_slate":
		if YesNoPrompt("You are about to reset the system to a clean slate. This is high risk action. Are you sure you want to reset to a clean slate? There is no rollback...", false) {
			clean_slate()
		} else {
			fmt.Println("Grovy, not cleaning.")
		}
	case "docs":
		requireArgs(3, "docs <readme|admin>")
		showDoc(os.Args[2])
	case "pull_file":
		requireArgs(4, "pull_file <podfilepath> <hostfilepath>")
		stdout, err := pullFile(os.Args[2], os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+err.Error()+colorReset)
			os.Exit(1)
		}
		fmt.Println(stdout)
	case "list_files_on_disk":
		stdout, err := listFilesOnDisk()
		if err != nil {
			fmt.Fprintln(os.Stderr, colorRed+err.Error()+colorReset)
			os.Exit(1)
		}
		fmt.Println(stdout)
	case "view_log_list":
		viewLogList(host)
	case "view_log":
		requireArgs(3, "view_log <logfile>")
		viewLog(os.Args[2], host)
	default:
		fmt.Fprintf(os.Stderr, colorRed+"unknown command: %s\n"+colorReset, os.Args[1])
		fmt.Fprintln(os.Stderr, colorYellow+usage+colorReset)
		os.Exit(0)
	}
}

// buildSafePath joins host with segments, escaping each one individually
// before joining — so "/", "..", or "?" inside a user-supplied segment
// (e.g. a filename or IP passed as an argument) can't be reinterpreted as
// extra path segments or query syntax. Escaping a plain static segment
// like "core" is a no-op, so it's safe to run every segment through this
// the same way regardless of whether it's static or user-supplied.
func buildSafePath(host string, segments ...string) (string, error) {
	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = url.PathEscape(s)
	}
	return url.JoinPath(host, escaped...)
}
