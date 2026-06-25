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

// Database specific constants
const maxOpenDBConnections = 10         // maximum allowable open connections
const maxIdleDBConnections = 10         // maximum allowable idle connections
const dbConnectionTimeoutMultiplier = 2 // in minutes

// Prometheus server
const prometheusServer = ":8081"

// Plugin specific constants
const pluginDir = "./plugins" // Directory where plugins live
