package main

import (
	"os"
	"fmt"
	"strings"
	"net"
	"net/url"
	"bufio"
)

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

