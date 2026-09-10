package main

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
  get_file <filename>                 GET  /core/files/:filename?meta=false
  get_file_meta <filename>            GET  /core/files/:filename?meta=true
  delete_file <filename>              DELETE /core/files/:filename
  undelete_file <filename>            POST /core/files/undelete/:filename
  refilter_file <filename>            POST /core/files/refilter/:filename (issue #324)
  upload <path> [field=value ...]     POST /core/upload (optional metadata fields, see below)
  self_ip                             locally-determined outbound IP toward the host
  metrics                              live terminal metrics view (issue #348), polls GET /metrics
                                       every 5s, q to quit, arrows/jk/wheel to scroll
  docs <readme|admin>                  terminal markdown viewer (issue #379) for README.md or
                                       frontend/doctooladmin/README.md, q to quit, arrows/jk/wheel to scroll
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
