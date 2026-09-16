package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func IsUpHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	body, err := IsUp()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("IsUp requests body:, %s!", body)), nil
}

func VersionHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	body, err := Version()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Version requests body:, %s!", body)), nil
}

func GetPodmanContainersHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	body, err := GetPodmanContainers()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("GetPodmanContainers requests body:, %s!", body)), nil
}

func RestartPodmanContainerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	containerID, err := request.RequireString("container_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	out, err := RestartPodmanContainer(containerID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Command ran with message: %s", out)), nil
}

func GetPodmanContainerLogsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	containerID, err := request.RequireString("container_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	out, err := GetPodmanContainerLogs(containerID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Command ran with message: %s", out)), nil
}

func GetPodmanHealthHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	out, err := GetPodmanHealth()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Command ran with message: %s", out)), nil
}

func CreateFileHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	contents, err := request.RequireString("contents")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := CreateFile(contents, path); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText("File created successfully"), nil
}

func ReadFileHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	contents, err := ReadFile(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(contents), nil
}

func MoveFileHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src, err := request.RequireString("src")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	dst, err := request.RequireString("dst")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := MoveFile(src, dst); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText("File moved successfully"), nil
}

func DeleteFileHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := DeleteFile(path); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText("File deleted successfully"), nil
}
