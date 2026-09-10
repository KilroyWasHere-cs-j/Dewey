package main

import (
	"os"
	"fmt"
	"os/exec"
)

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


