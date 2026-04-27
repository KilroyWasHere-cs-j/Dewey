package main

// Maybe use structs here

// File system specific constants
const uploadDir = "./cache" // Temp dir for storing files after files post upload and for fast query access
const fileSystemBaseDir = "./store" // The base directory at which all the stored files live
const rulesDir = "./rules" // Where the rules in JSON format live
const htmlTemplatesDir = "./templates" // Contains the HTML files that are used to render the admin pages
const daemonTickTime = 1 // In hours, if you need this to fire less than an hour you'll need to do the math  

// Server specific constants
const maxFileSize = 50 << 2 // Controls the max size that uploaded can be
const portNumber = "8080" // Port to spawn on
const allowedIP = "127.0.0.1" // Only IP allowed to connect to the server (no worky)
const loopback = "::1" // Allows for loopback to work (no worky)

// Database specific constants
const dbConnectionString = ""

// Prometheus server
const prometheusServer = ":8081"
