package main

// Maybe use structs here

// File system specific constants
const uploadDir = "./cache"         // temp dir for storing files after files post upload and for fast query access
const fileSystemBaseDir = "./store" // base directory where all stored files start from
const backupDir = "./backup"        // directory where backup files are stored
const daemonTickTime = 1            // system tick interval in hours (note don't try and set this to a float as it won't compile)

// Server specific constants
const maxFileSize = 50 << 20 // maximum file that can be uploaded in bytes (50MB)
const portNumber = "8080"    // port for the server to listen on
const appVersion = "0.1.0"   // app release version, bump by hand before cutting a release

// Rate limiter (global, not per-IP — shared across every client hitting this
// server). A single dashboard page load fires off several requests (files
// or machines, metrics, version), so this needs enough headroom for normal
// navigation, not just a single request.
const rateLimitPerSecond = 10 // steady-state requests/sec refill rate
const rateLimitBurst = 20     // burst allowance on top of the refill rate

// Database specific constants
const maxOpenDBConnections = 10         // maximum allowable open connections
const maxIdleDBConnections = 10         // maximum allowable idle connections
const dbConnectionTimeoutMultiplier = 2 // in minutes

// Plugin specific constants
const pluginDir = "./plugins" // Directory where plugins live

// Post-processing (issue #217) — bounds how many idAndSort goroutines
// (barcode scan + Lua filter plugins + disk copy) can run at once. Without
// this, a burst of uploads accepted just under the rate limiter could each
// spin up a full Lua-plugin-running goroutine concurrently with no ceiling,
// letting them pile up faster than a slow filesystem can drain them.
const maxConcurrentPostProcessing = 4
