package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// docxFiles maps the required internal paths of a .docx (a zip package)
// to their XML content. This is the minimum set Word/LibreOffice need
// to open the file: content-type registry, root relationship, and body.
var docxFiles = map[string]string{
	"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,

	"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,

	"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r>
        <w:t>Hello, world!</w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
}

func createDocx(filename string) error {
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()

		zw := zip.NewWriter(f)
		for name, content := range docxFiles {
			w, err := zw.Create(name)
			if err != nil {
				return err
			}
			if _, err := w.Write([]byte(content)); err != nil {
				return err
			}
		}
		if err := zw.Close(); err != nil {
			return err
		}
		return nil
	}
	return nil
}

func createExe(filename string) error {
	buf := new(bytes.Buffer)
	w16 := func(v uint16) { binary.Write(buf, binary.LittleEndian, v) }
	w32 := func(v uint32) { binary.Write(buf, binary.LittleEndian, v) }
	w64 := func(v uint64) { binary.Write(buf, binary.LittleEndian, v) }

	// --- DOS header (64 bytes) ---
	// Only e_magic ("MZ") and e_lfanew (offset of PE header) matter here.
	w16(0x5A4D)                 // e_magic = "MZ"
	buf.Write(make([]byte, 58)) // remaining DOS header fields, unused
	w32(0x40)                   // e_lfanew: PE header starts right after this header

	// --- PE signature ---
	buf.WriteString("PE\x00\x00")

	// --- COFF file header (20 bytes) ---
	w16(0x8664) // Machine = AMD64
	w16(1)      // NumberOfSections
	w32(0)      // TimeDateStamp
	w32(0)      // PointerToSymbolTable
	w32(0)      // NumberOfSymbols
	w16(0xF0)   // SizeOfOptionalHeader (240 bytes, PE32+)
	w16(0x0022) // Characteristics: EXECUTABLE_IMAGE | LARGE_ADDRESS_AWARE

	// --- Optional header (PE32+, 240 bytes) ---
	w16(0x20B)       // Magic: PE32+
	buf.WriteByte(0) // MajorLinkerVersion
	buf.WriteByte(0) // MinorLinkerVersion
	w32(0x200)       // SizeOfCode
	w32(0)           // SizeOfInitializedData
	w32(0)           // SizeOfUninitializedData
	w32(0x1000)      // AddressOfEntryPoint (RVA)
	w32(0x1000)      // BaseOfCode
	w64(0x140000000) // ImageBase
	w32(0x1000)      // SectionAlignment
	w32(0x200)       // FileAlignment
	w16(6)           // MajorOSVersion
	w16(0)           // MinorOSVersion
	w16(0)           // MajorImageVersion
	w16(0)           // MinorImageVersion
	w16(6)           // MajorSubsystemVersion
	w16(0)           // MinorSubsystemVersion
	w32(0)           // Win32VersionValue
	w32(0x2000)      // SizeOfImage
	w32(0x200)       // SizeOfHeaders
	w32(0)           // CheckSum
	w16(3)           // Subsystem: WINDOWS_CUI (console)
	w16(0)           // DllCharacteristics
	w64(0x100000)    // SizeOfStackReserve
	w64(0x1000)      // SizeOfStackCommit
	w64(0x100000)    // SizeOfHeapReserve
	w64(0x1000)      // SizeOfHeapCommit
	w32(0)           // LoaderFlags
	w32(16)          // NumberOfRvaAndSizes
	for i := 0; i < 16; i++ {
		w64(0) // 16 empty data directories (no imports/exports/etc.)
	}

	// --- Section header: .text (40 bytes) ---
	buf.WriteString(".text\x00\x00\x00") // Name (8 bytes, padded)
	w32(3)                               // VirtualSize (actual code size)
	w32(0x1000)                          // VirtualAddress (RVA)
	w32(0x200)                           // SizeOfRawData (file-aligned)
	w32(0x200)                           // PointerToRawData
	w32(0)                               // PointerToRelocations
	w32(0)                               // PointerToLinenumbers
	w16(0)                               // NumberOfRelocations
	w16(0)                               // NumberOfLinenumbers
	w32(0x60000020)                      // Characteristics: CODE | EXECUTE | READ

	// Pad headers out to SizeOfHeaders (0x200) before section data starts.
	buf.Write(make([]byte, 0x200-buf.Len()))

	// --- .text section: xor eax,eax ; ret  (exits with code 0) ---
	buf.Write([]byte{0x31, 0xC0, 0xC3})
	buf.Write(make([]byte, 0x200-3)) // pad section to its declared raw size

	if err := os.WriteFile(filename, buf.Bytes(), 0755); err != nil {
		return err
	}
	return nil
}

// buildPDF constructs a minimal valid PDF by hand, with independent toggles:
//
//   - withJS:         adds a JavaScript action, exposed via /Names/JavaScript
//     so it runs on open regardless of /OpenAction.
//   - withOpenAction:  adds a /OpenAction entry to the catalog. If JS is also
//     on, OpenAction points at the JS action (the classic
//     dangerous combo). If JS is off, OpenAction points at a
//     benign /GoTo (jump to page 1) so you can test that an
//     OpenAction alone, with no scripting, isn't flagged.
func createPDF(filename string, withJS, withOpenAction bool) error {
	buf := new(bytes.Buffer)
	var offsets []int // byte offset of each object, needed for the xref table

	buf.WriteString("%PDF-1.7\n")
	buf.Write([]byte{'%', 0xE2, 0xE3, 0xCF, 0xD3, '\n'}) // binary marker per spec

	writeObj := func(num int, body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(buf, "%d 0 obj\n%s\nendobj\n", num, body)
	}

	// Object 5 (if present) is either the JS action or a benign GoTo action —
	// the two cases are mutually exclusive, so the numbering stays simple.
	hasObj5 := withJS || withOpenAction

	catalog := "<< /Type /Catalog /Pages 2 0 R"
	if withJS {
		catalog += " /Names << /JavaScript << /Names [(EmbeddedJS) 5 0 R] >> >>"
	}
	if withOpenAction {
		catalog += " /OpenAction 5 0 R"
	}
	catalog += " >>"

	// Object 1: Catalog
	writeObj(1, catalog)

	// Object 2: Pages tree (one page)
	writeObj(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")

	// Object 3: The page itself
	writeObj(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << >> /Contents 4 0 R >>")

	// Object 4: Blank content stream
	content := "BT ET"
	writeObj(4, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))

	// Object 5: JS action (if withJS) or benign GoTo action (if OpenAction-only)
	if hasObj5 {
		if withJS {
			js := `app.alert('test payload - benign, no exploit');`
			writeObj(5, fmt.Sprintf("<< /Type /Action /S /JavaScript /JS (%s) >>", js))
		} else {
			writeObj(5, "<< /Type /Action /S /GoTo /D [3 0 R /Fit] >>")
		}
	}

	// xref table — one 20-byte entry per object, in object-number order
	xrefStart := buf.Len()
	numObjs := len(offsets) + 1 // +1 for the free-list head (object 0)
	fmt.Fprintf(buf, "xref\n0 %d\n", numObjs)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(buf, "%010d 00000 n \n", off)
	}

	fmt.Fprintf(buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", numObjs)
	fmt.Fprintf(buf, "startxref\n%d\n%%%%EOF", xrefStart)

	if err := os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
		return err
	}
	fmt.Println("wrote", filename)
	return nil
}
