package main

import (	
	"fmt"
	"os"
	"strconv"
)



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
		get(host+"/core/files/"+os.Args[2]+"?meta=false", filesPassword, nil)
	case "get_file_meta":
		requireArgs(3, "get_file_meta <filename>")
		get(host+"/core/files/"+os.Args[2]+"?meta=true", filesPassword, nil)
	case "delete_file":
		requireArgs(3, "delete_file <filename>")
		del(host+"/core/files/"+os.Args[2], filesPassword, nil)
	case "undelete_file":
		requireArgs(3, "undelete_file <filename>")
		postJSON(host+"/core/files/undelete/"+os.Args[2], filesPassword, nil, nil)
	case "refilter_file":
		requireArgs(3, "refilter_file <filename>")
		postJSON(host+"/core/files/refilter/"+os.Args[2], filesPassword, nil, nil)
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
		requireArgs(2, "pull_file <podfilepath> <hostfilepath>")
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
		requireArgs(2, "view_log <logfile>")
		viewLog(os.Args[2], host)
	default:
		fmt.Fprintf(os.Stderr, colorRed+"unknown command: %s\n"+colorReset, os.Args[1])
		fmt.Fprintln(os.Stderr, colorYellow+usage+colorReset)
		os.Exit(0)
	}
}

