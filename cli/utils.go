package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"dewey-httpclient"
)

// selfIP prints the local address the OS would use to reach host, via
// httpclient.SelfIP — see that function's doc comment for what this
// actually measures and its limits.
func selfIP(host string) {
	ip, err := httpclient.SelfIP(host)
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"could not determine local IP:"+colorReset, err)
		os.Exit(1)
	}
	fmt.Println(colorCyan + ip + colorReset)
}

// parseMetadata turns trailing "field=value" CLI args into the metadata map
// upload attaches to the request, rejecting anything that isn't in
// httpclient.UploadMetaFields so a typo'd field name fails fast instead of
// being silently dropped by the backend's PostForm lookup.
func parseMetadata(args []string) map[string]string {
	valid := make(map[string]struct{}, len(httpclient.UploadMetaFields))
	for _, k := range httpclient.UploadMetaFields {
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
			fmt.Fprintf(os.Stderr, colorRed+"unknown metadata field %q, expected one of: %s\n"+colorReset, key, strings.Join(httpclient.UploadMetaFields, ", "))
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
