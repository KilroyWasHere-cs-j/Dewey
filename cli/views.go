package main

import (
	"fmt"
	"os"

	tui "github.com/grindlemire/go-tui"
)

// runMetricsDashboard launches the live terminal metrics view from
// metrics_dashboard.gsx (issue #348) — polls host's /metrics every
// pollInterval and renders it. WithMouse is needed for the dashboard's
// scroll-wheel support.
func runMetricsDashboard(host string) {
	app, err := tui.NewApp(tui.WithRootComponent(MetricsDashboard(host)), tui.WithMouse())
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to start metrics view:"+colorReset, err)
		os.Exit(1)
	}
	defer app.Close()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"metrics view error:"+colorReset, err)
		os.Exit(1)
	}
}

// showDoc launches the terminal doc viewer for the named doc ("readme" or
// "admin", per docPaths in docs_viewer.go).
func showDoc(name string) {
	app, err := tui.NewApp(tui.WithRootComponent(DocsDashboard(name)), tui.WithMouse())
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"failed to start docs view:"+colorReset, err)
		os.Exit(1)
	}
	defer app.Close()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"docs view error:"+colorReset, err)
		os.Exit(1)
	}
}

