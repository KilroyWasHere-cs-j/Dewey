package main

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"Dewey MCP",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Add tool
	//
	// Annotation hints (read-only/destructive/idempotent/open-world) are set
	// explicitly on every tool below — mcp-go's own defaults mark every tool
	// destructive regardless of what it actually does, which would make a
	// client treat is_up/version/every get_* tool exactly as riskily as
	// restart_podman_container. None of these tools touch an unpredictable
	// external system (openWorldHint), since they only ever talk to Dewey's
	// own backend or its own pod.
	isUpTool := mcp.NewTool("is_up",
		mcp.WithDescription("Checks whether the Dewey backend is reachable and responding, via GET /"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	versionTool := mcp.NewTool("version",
		mcp.WithDescription("Returns the running Dewey backend's release version and the git branch it was built from"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	getPodmanHealthTool := mcp.NewTool("get_podman_health",
		mcp.WithDescription("Returns the status of the dewey-pod Podman pod (podman pod ps) — whether it's running and how many containers it has, not any single container's individual health"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	getPodmanContainersTool := mcp.NewTool("get_podman_containers",
		mcp.WithDescription("Lists every Podman container currently running (podman ps) — use this to find a container's name/ID before calling get_podman_container_logs or restart_podman_container"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	getPodmanContainerLogsTool := mcp.NewTool("get_podman_container_logs",
		mcp.WithDescription("Returns the recent log output of a single Podman container by name or ID"),
		mcp.WithString("container_id",
			mcp.Required(),
			mcp.Description("The name or ID of the container to fetch logs for, e.g. \"cross-doc-tool-dev\" or \"dewey-mysql\" — see get_podman_containers for valid values"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	restartPodmanContainerTool := mcp.NewTool("restart_podman_container",
		mcp.WithDescription("Restarts a single Podman container by name or ID. This interrupts whatever that container is currently serving — for the backend or frontend that's a brief outage, for the database it drops in-flight connections. Does not affect other containers in the pod."),
		mcp.WithString("container_id",
			mcp.Required(),
			mcp.Description("The name or ID of the container to restart, e.g. \"cross-doc-tool-dev\" or \"dewey-mysql\" — see get_podman_containers for valid values"),
		),
		// The only tool here that's genuinely destructive/non-idempotent —
		// it interrupts whatever that container was doing, and calling it
		// again does another full restart rather than converging to a
		// no-op like an idempotent operation would.
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	// Add tool handler
	s.AddTool(isUpTool, IsUpHandler)
	s.AddTool(versionTool, VersionHandler)
	s.AddTool(getPodmanHealthTool, GetPodmanHealthHandler)
	s.AddTool(getPodmanContainersTool, GetPodmanContainersHandler)
	s.AddTool(getPodmanContainerLogsTool, GetPodmanContainerLogsHandler)
	s.AddTool(restartPodmanContainerTool, RestartPodmanContainerHandler)

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
