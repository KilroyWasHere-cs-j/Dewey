package main

import (
	"encoding/json"
	"os"
)

// configFile is the path load() reads at startup — relative to the
// binary's working directory, same as every other path in this file
// (./cache, ./store, etc.).
const configFile = "./config.json"

// dateLayout is Go's reference-time layout string for "YYYY-MM-DD" —
// time.Parse/time.Format take this exact reference date
// ("Mon Jan 2 15:04:05 MST 2006"), not strftime-style tokens.
const dateLayout = "2006-01-02"

// Config mirrors config.json's shape. Field names match the const names
// this file used to declare (issue #256) so callers throughout the
// codebase keep referencing the same package-level identifiers below —
// only this file and the handful of call sites that needed a type change
// for the switch from untyped consts to typed vars were touched.
type Config struct {
	FileSystem FileSystemConfig `json:"file_system"`
	Server     ServerConfig     `json:"server"`
	RateLimit  RateLimitConfig  `json:"rate_limit"`
	Database   DatabaseConfig   `json:"database"`
	Plugin     PluginConfig     `json:"plugin"`
}

// FileSystem specific config
type FileSystemConfig struct {
	AppDir            string  `json:"app_dir"`                  // root directory the app and its data live under on the server
	UploadDir         string  `json:"upload_dir"`               // temp dir for storing files after files post upload and for fast query access
	FileSystemBaseDir string  `json:"file_system_base_dir"`     // base directory where all stored files start from
	BackupDir         string  `json:"backup_dir"`               // directory where backup files are stored
	LogsDir           string  `json:"logs_dir"`                 // directory where log files are stored
	DaemonTickTime    int     `json:"daemon_tick_time_minutes"` // system tick interval in minutes
	Alpha             float64 `json:"alpha"`                    // extra seconds added per active user in the tick-scaling formula (issue #305)
	Beta              float64 `json:"beta"`                     // extra seconds added per unit of smoothed upload rate in the tick-scaling formula (issue #305)
	TBase             float64 `json:"t_base"`                   // tick times base value for the tick-scaling formulua
	TickMax           int     `json:"tick_max"`                 // maximum number of space between each tick
	TickMin           int     `json:"tick_min"`                 // minimum number of space between each tick
	// BackupIntervalMinutes is the fixed period between full backups
	// (mysqldump + store/ zip), independent of the fast adaptive daemon
	// ticker above — a full backup is too expensive to ride the same
	// load-adaptive clock as cheap housekeeping like cache-clearing
	// (issue #393).
	BackupIntervalMinutes int `json:"backup_interval_minutes"`
}

// Server specific config
type ServerConfig struct {
	MaxFileSize int64  `json:"max_file_size_bytes"` // maximum file that can be uploaded in bytes
	PortNumber  string `json:"port_number"`         // port for the server to listen on
	AppVersion  string `json:"app_version"`         // app release version, bump by hand before cutting a release
}

// Rate limiter config (global, not per-IP — shared across every client
// hitting this server). A single dashboard page load fires off several
// requests (files or machines, metrics, version), so this needs enough
// headroom for normal navigation, not just a single request.
type RateLimitConfig struct {
	RequestsPerSecond float64 `json:"requests_per_second"` // steady-state requests/sec refill rate
	Burst             int     `json:"burst"`               // burst allowance on top of the refill rate
}

// Database specific config
type DatabaseConfig struct {
	MaxOpenConnections          int `json:"max_open_connections"`                  // maximum allowable open connections
	MaxIdleConnections          int `json:"max_idle_connections"`                  // maximum allowable idle connections
	ConnectionTimeoutMultiplier int `json:"connection_timeout_multiplier_minutes"` // in minutes
}

// Plugin specific config
type PluginConfig struct {
	PluginDir string `json:"plugin_dir"` // Directory where plugins live
	// PluginScratchDir is where files.read/files.write (issue #284) confine
	// a plugin's file access — kept separate from store/cache/backup so a
	// plugin can never reach documents a user actually uploaded.
	PluginScratchDir string `json:"plugin_scratch_dir"`
	// HTTPTimeoutMultiplier bounds how long a plugin's http.get/http.post
	// call (issue #404) is allowed to run before it's cancelled.
	HTTPTimeoutMultiplier int `json:"http_timeout_multiplier_seconds"` // in seconds
	// HookTimeoutMultiplier bounds an entire hook invocation (issue #404) —
	// larger than HTTPTimeoutMultiplier since one hook call can legitimately
	// make several sequential http.get/http.post calls, each already bounded
	// on its own; this is the outer ceiling on the whole call, including any
	// pure-Lua work (loops, string processing) that isn't an HTTP call at all.
	HookTimeoutMultiplier int `json:"hook_timeout_multiplier_seconds"` // in seconds
	// MaxPluginFileSize caps how large a file a plugin can write via
	// files.write (issue #448).
	MaxPluginFileSize int64 `json:"max_plugin_file_size_bytes"` // maximum file size a plugin can write, in bytes
	// MaxPluginDownloadSize caps how much a plugin's http.get/http.post
	// (issue #448) can pull down in a single response.
	MaxPluginDownloadSize int64 `json:"max_plugin_download_size_bytes"` // maximum response size a plugin can download, in bytes
}

// Package-level vars populated by load() — same identifiers every other
// file in this package already references, so this is the only file that
// needed to know config.json exists.
var (
	appDir                string
	uploadDir             string
	fileSystemBaseDir     string
	backupDir             string
	logsDir               string
	daemonTickTime        int
	alpha                 float64
	beta                  float64
	tBase                 float64
	tickMax               int
	tickMin               int
	backupIntervalMinutes int

	maxFileSize int64
	portNumber  string
	appVersion  string

	rateLimitPerSecond float64
	rateLimitBurst     int

	maxOpenDBConnections          int
	maxIdleDBConnections          int
	dbConnectionTimeoutMultiplier int

	pluginDir                   string
	pluginScratchDir            string
	pluginHTTPTimeoutMultiplier int
	pluginHookTimeoutMultiplier int
	maxPluginFileSize           int64
	maxPluginDownloadSize       int64
)

// load reads configFile and populates every package-level config var
// above. Called first thing in main(), before anything (plugins, the
// rate limiter, the DB pool) that depends on these values. Fails fast on
// a missing or malformed config file rather than silently falling back
// to defaults — same reasoning as DB_DSN's required env var (issue
// #200): a config error should stop startup loudly, not get masked by a
// fallback that happens to still work most of the time.
func load() {
	data, err := os.ReadFile(configFile)
	if err != nil {
		Fatal("Failed to read " + configFile + ": " + err.Error())
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		Fatal("Failed to parse " + configFile + ": " + err.Error())
	}

	appDir = cfg.FileSystem.AppDir
	uploadDir = cfg.FileSystem.UploadDir
	fileSystemBaseDir = cfg.FileSystem.FileSystemBaseDir
	backupDir = cfg.FileSystem.BackupDir
	logsDir = cfg.FileSystem.LogsDir
	daemonTickTime = cfg.FileSystem.DaemonTickTime
	alpha = cfg.FileSystem.Alpha
	beta = cfg.FileSystem.Beta
	tBase = cfg.FileSystem.TBase
	tickMax = cfg.FileSystem.TickMax
	tickMin = cfg.FileSystem.TickMin
	backupIntervalMinutes = cfg.FileSystem.BackupIntervalMinutes

	maxFileSize = cfg.Server.MaxFileSize
	portNumber = cfg.Server.PortNumber
	appVersion = cfg.Server.AppVersion

	rateLimitPerSecond = cfg.RateLimit.RequestsPerSecond
	rateLimitBurst = cfg.RateLimit.Burst

	maxOpenDBConnections = cfg.Database.MaxOpenConnections
	maxIdleDBConnections = cfg.Database.MaxIdleConnections
	dbConnectionTimeoutMultiplier = cfg.Database.ConnectionTimeoutMultiplier

	pluginDir = cfg.Plugin.PluginDir
	pluginScratchDir = cfg.Plugin.PluginScratchDir
	pluginHTTPTimeoutMultiplier = cfg.Plugin.HTTPTimeoutMultiplier
	pluginHookTimeoutMultiplier = cfg.Plugin.HookTimeoutMultiplier
	maxPluginFileSize = cfg.Plugin.MaxPluginFileSize
	maxPluginDownloadSize = cfg.Plugin.MaxPluginDownloadSize
}
