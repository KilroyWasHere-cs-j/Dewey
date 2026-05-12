# Binary Validation Module Documentation

## Overview
This module provides logic for identifying executable binary formats by analyzing their internal structures and "magic numbers." It specifically supports Windows Portable Executable (PE) and Linux Executable and Linkable Format (ELF) files. These validations are critical for security screening and system-specific routing.

## Global State
- **PECount**: Integer tracking the total number of successfully validated Windows PE files.
- **ELFCount**: Integer tracking the total number of successfully validated Linux ELF files.

---

## Function Reference

### `IsPEFile(f io.ReadSeeker) (bool, error)`
Determines if the provided reader contains a valid Windows PE binary. The function performs a two-stage validation:
1. **DOS Header Check**: Reads the first 64 bytes to locate the "MZ" signature at the start of the file.
2. **PE Offset Identification**: Extracts the `e_lfanew` value (located at offset `0x3C`) to find the location of the PE header.
3. **PE Signature Check**: Seeks to the calculated offset and verifies the presence of the `PE  ` signature.
- **Returns**: `true` if both signatures are present and correctly positioned.
- **Performance**: Requires two seek operations and two read operations.

### `IsELFFile(f io.ReadSeeker) (bool, error)`
Determines if the provided reader contains a valid Linux ELF binary. 
- **Mechanism**: Reads the first 4 bytes and compares them against the standard ELF magic number: `0x7F`, `E`, `L`, `F`.
- **Returns**: `true` if the header matches.
- **Performance**: Highly efficient; requires one seek to start and one 4-byte read.

### `zeroize(b []byte)`
A utility function for memory safety and sensitive data handling.
- **Mechanism**: Iterates through the provided byte slice and overwrites every element with `0`.
- **Note**: This provides a "best-effort" cleanup. In high-security contexts, be aware that Go's runtime or compiler optimizations may occasionally affect the timing or persistence of these operations.

---

## Technical Details: Binary Signatures

| Format | Header Type | Offset | Expected Value |
| :--- | :--- | :--- | :--- |
| **PE (Windows)** | DOS Signature | 0x00 | `MZ` |
| **PE (Windows)** | PE Signature | Variable (`e_lfanew`) | `PE  ` |
| **ELF (Linux)** | Magic Number | 0x00 | `0x7F 45 4C 46` |

---

## Usage Considerations
- **Seekable Input**: Both validation functions require an `io.ReadSeeker`. If providing an `os.File`, ensure the file is open and readable.
- **Counter Persistance**: The `PECount` and `ELFCount` are global to the package. In concurrent environments, consider using atomic operations if thread safety is required.
