# File System & Rule Engine Documentation

## Overview
This module provides a robust system for managing file uploads, automated sorting based on configurable rules, and database-backed file retrieval. It is designed to handle file ingestion by matching incoming filenames against a set of regex patterns and routing them to specific target directories while maintaining a record of the transaction in a SQL database.

## Key Features
* **Regex-Based Routing:** Automated file sorting based on filename patterns.
* **Dynamic Configuration:** Rules are loaded from a JSON configuration file.
* **Persistence:** Integrates with a SQL database to track file locations and metadata.
* **Caching Strategy:** Efficiently checks local upload directories before querying the database.

---

## Data Structures

### Config
The root configuration object loaded at runtime.
- **Version**: Schema versioning for the ruleset.
- **Level**: Logging or priority level for the rule engine.
- **Rules**: A slice of `Rule` objects defining sorting logic.

### Rule
Defines how a specific file type should be handled.
- **NameMatch**: A regular expression string matched against the file path.
- **Action**: The operation to perform (e.g., move, copy).
- **TargetDirectory**: The destination path relative to the system base directory.
- **Meta**: Custom tags associated with the rule.

---

## Function Reference

### `fileSystemInit() *Config`
Initializes the environment. It ensures that the `uploadDir` and `fileSystemBaseDir` exist with restricted permissions (`0600`). It then triggers the loading of filter rules.
- **Returns**: A pointer to the loaded `Config`.
- **Panics**: If directory creation fails or config is missing.

### `loadFilters() *Config`
Reads `rules.json` from the rules directory.
- **Strict Parsing**: Uses `DisallowUnknownFields()` to ensure the JSON matches the Go structs exactly.
- **State Management**: Increments the `FiltersLoadings` counter upon success.

### `idAndSort(path string, hash string, filename string)`
The core processing engine for new files.
1.  Iterates through all configured rules.
2.  Compiles the `NameMatch` regex.
3.  If a match is found:
    -   Ensures the target directory exists.
    -   Copies the file from the upload cache to the permanent storage.
    -   Calls `createNewFileRecord` to persist the location and hash in the database.
4.  Exits after the first successful match.

### `searchAndReturn(filename string, pullMeta string) string`
Retrieves a file's location.
- **Cache-First**: Scans the `uploadDir` for immediate hits.
- **Database Fallback**: If not in cache, queries the SQL database using `pullRecordByFilename`.
- **Meta Support**: Can be toggled to return metadata instead of the file path.

### `CopyFile(src, dst string) error`
A utility function that handles low-level I/O operations. It performs a buffered copy, ensures destination directories exist, and calls `Sync()` to flush writes to physical disk storage.

---

## Future Implementations
- **PDF Splitting**: Logic to treat individual PDF pages as unique records.
- **Metadata Management**: `changeMeta()` and `setFileStatus()` are currently stubs for SQL-backed metadata updates and soft-deletion logic.
- **Security Enhancements**: Implement strict path validation to prevent directory traversal attacks during file retrieval.

developer_documentation.md
Displaying developer_documentation.md.
