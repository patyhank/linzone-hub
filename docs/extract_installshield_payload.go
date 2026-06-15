//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var oleSignature = []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}

type section struct {
	name     string
	rva      uint32
	virtSize uint32
	raw      uint32
	rawSize  uint32
}

type peImage struct {
	data        []byte
	sections    []section
	resourceRVA uint32
}

type resourceBlob struct {
	offset int
	size   int
	path   string
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: go run ./extract_installshield_payload.go <INZONEHub_Setup.exe> <output-dir> [file ...]\n")
	fmt.Fprintf(os.Stderr, "Default files: INZONEHub.dll APP_NOTIFY_ICON.png\n")
}

func main() {
	if len(os.Args) < 3 {
		usage()
		os.Exit(2)
	}

	input := os.Args[1]
	outputDir := os.Args[2]
	targets := os.Args[3:]
	if len(targets) == 0 {
		targets = []string{"INZONEHub.dll", "APP_NOTIFY_ICON.png"}
	}

	if _, err := exec.LookPath("7z"); err != nil {
		fatal(errors.New("7z not found in PATH; native MSI/CAB extraction requires 7z"))
	}

	data, err := os.ReadFile(input)
	if err != nil {
		fatal(err)
	}

	inner, innerOffset, err := findInnerPE(data)
	if err != nil {
		fatal(err)
	}

	blob, err := findMSIResource(inner)
	if err != nil {
		fatal(err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatal(err)
	}

	tmp, err := os.MkdirTemp("", "inzone-msi-*")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(tmp)

	msiPath := filepath.Join(tmp, "inzone-resource.msi")
	if err := os.WriteFile(msiPath, inner.data[blob.offset:blob.offset+blob.size], 0o644); err != nil {
		fatal(err)
	}

	streamDir := filepath.Join(tmp, "streams")
	cabDir := filepath.Join(tmp, "cab")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(cabDir, 0o755); err != nil {
		fatal(err)
	}

	fmt.Printf("[*] Found inner PE at 0x%x\n", innerOffset)
	fmt.Printf("[*] Found MSI resource %s at inner offset 0x%x size 0x%x\n", blob.path, blob.offset, blob.size)

	run("7z", "x", "-y", "-o"+streamDir, msiPath, "Data1.cab")
	cabPath := filepath.Join(streamDir, "Data1.cab")
	if _, err := os.Stat(cabPath); err != nil {
		fatal(fmt.Errorf("Data1.cab stream was not extracted from MSI resource: %w", err))
	}

	args := []string{"x", "-y", "-o" + cabDir, cabPath}
	for _, target := range targets {
		args = append(args, strings.ToLower(filepath.Base(target)))
	}
	run("7z", args...)

	for _, target := range targets {
		path, err := findByBase(cabDir, target)
		if err != nil {
			fatal(err)
		}

		name := filepath.Base(path)
		if strings.EqualFold(name, "APP_NOTIFY_ICON.png") {
			resourcesDir := filepath.Join(outputDir, "resources")
			if err := os.MkdirAll(resourcesDir, 0o755); err != nil {
				fatal(err)
			}
			copyFile(path, filepath.Join(resourcesDir, "APP_NOTIFY_ICON.png"))
		}

		dst := filepath.Join(outputDir, name)
		copyFile(path, dst)
		fmt.Printf("[+] extracted %s\n", dst)
	}
}

func findInnerPE(data []byte) (*peImage, int, error) {
	for start := 0; ; start++ {
		i := bytes.Index(data[start:], []byte("MZ"))
		if i < 0 {
			return nil, 0, errors.New("inner PE not found")
		}
		start += i

		pe, err := parsePE(data[start:])
		if err != nil {
			continue
		}
		if pe.resourceRVA == 0 || !hasSection(pe.sections, ".rsrc") {
			continue
		}
		if _, err := findMSIResource(pe); err != nil {
			continue
		}

		return pe, start, nil
	}
}

func parsePE(data []byte) (*peImage, error) {
	if len(data) < 0x40 || string(data[:2]) != "MZ" {
		return nil, errors.New("not MZ")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3c:0x40]))
	if peOff < 0 || peOff+24 > len(data) || string(data[peOff:peOff+4]) != "PE\x00\x00" {
		return nil, errors.New("not PE")
	}

	nsec := int(binary.LittleEndian.Uint16(data[peOff+6 : peOff+8]))
	optsize := int(binary.LittleEndian.Uint16(data[peOff+20 : peOff+22]))
	opt := peOff + 24
	if opt+optsize > len(data) || optsize < 136 {
		return nil, errors.New("bad optional header")
	}
	magic := binary.LittleEndian.Uint16(data[opt : opt+2])
	if magic != 0x20b {
		return nil, fmt.Errorf("unsupported optional header magic 0x%x", magic)
	}
	numDirs := binary.LittleEndian.Uint32(data[opt+108 : opt+112])
	if numDirs <= 2 {
		return nil, errors.New("PE has no resource data directory")
	}
	resourceDir := opt + 112 + 2*8
	resourceRVA := binary.LittleEndian.Uint32(data[resourceDir : resourceDir+4])

	secOff := opt + optsize
	if secOff+nsec*40 > len(data) {
		return nil, errors.New("truncated section table")
	}
	sections := make([]section, 0, nsec)
	for i := 0; i < nsec; i++ {
		off := secOff + i*40
		sections = append(sections, section{
			name:     strings.TrimRight(string(data[off:off+8]), "\x00"),
			virtSize: binary.LittleEndian.Uint32(data[off+8 : off+12]),
			rva:      binary.LittleEndian.Uint32(data[off+12 : off+16]),
			rawSize:  binary.LittleEndian.Uint32(data[off+16 : off+20]),
			raw:      binary.LittleEndian.Uint32(data[off+20 : off+24]),
		})
	}

	return &peImage{data: data, sections: sections, resourceRVA: resourceRVA}, nil
}

func findMSIResource(pe *peImage) (resourceBlob, error) {
	rsrcOff, ok := pe.rvaToOff(pe.resourceRVA)
	if !ok {
		return resourceBlob{}, errors.New("resource directory RVA is not mapped")
	}

	blobs := make([]resourceBlob, 0)
	walkResources(pe, int(rsrcOff), int(rsrcOff), "", &blobs, 0)
	for _, blob := range blobs {
		data := pe.data[blob.offset : blob.offset+blob.size]
		if bytes.HasPrefix(data, oleSignature) && bytes.Contains(data, []byte("MSCF")) {
			return blob, nil
		}
	}
	return resourceBlob{}, errors.New("MSI/OLE resource containing embedded CAB was not found")
}

func walkResources(pe *peImage, base, dir int, path string, out *[]resourceBlob, depth int) {
	if depth > 8 || dir+16 > len(pe.data) {
		return
	}
	named := int(binary.LittleEndian.Uint16(pe.data[dir+12 : dir+14]))
	ids := int(binary.LittleEndian.Uint16(pe.data[dir+14 : dir+16]))
	for i := 0; i < named+ids; i++ {
		entry := dir + 16 + i*8
		if entry+8 > len(pe.data) {
			return
		}
		nameRaw := binary.LittleEndian.Uint32(pe.data[entry : entry+4])
		childRaw := binary.LittleEndian.Uint32(pe.data[entry+4 : entry+8])
		name := resourceName(pe.data, base, nameRaw)
		childPath := name
		if path != "" {
			childPath = path + "/" + name
		}

		child := base + int(childRaw&0x7fffffff)
		if childRaw&0x80000000 != 0 {
			walkResources(pe, base, child, childPath, out, depth+1)
			continue
		}
		if child+16 > len(pe.data) {
			continue
		}
		rva := binary.LittleEndian.Uint32(pe.data[child : child+4])
		size := binary.LittleEndian.Uint32(pe.data[child+4 : child+8])
		off, ok := pe.rvaToOff(rva)
		if !ok || uint64(off)+uint64(size) > uint64(len(pe.data)) {
			continue
		}
		*out = append(*out, resourceBlob{offset: int(off), size: int(size), path: childPath})
	}
}

func (pe *peImage) rvaToOff(rva uint32) (uint32, bool) {
	for _, sec := range pe.sections {
		size := sec.virtSize
		if sec.rawSize > size {
			size = sec.rawSize
		}
		if rva >= sec.rva && rva < sec.rva+size {
			return sec.raw + (rva - sec.rva), true
		}
	}
	return 0, false
}

func resourceName(data []byte, base int, raw uint32) string {
	if raw&0x80000000 == 0 {
		return fmt.Sprintf("#%d", raw)
	}
	off := base + int(raw&0x7fffffff)
	if off+2 > len(data) {
		return "<bad-name>"
	}
	n := int(binary.LittleEndian.Uint16(data[off : off+2]))
	runes := make([]rune, 0, n)
	for i := 0; i < n && off+4+i*2 <= len(data); i++ {
		runes = append(runes, rune(binary.LittleEndian.Uint16(data[off+2+i*2:off+4+i*2])))
	}
	return string(runes)
}

func hasSection(sections []section, name string) bool {
	for _, sec := range sections {
		if sec.name == name {
			return true
		}
	}
	return false
}

func findByBase(root, target string) (string, error) {
	want := strings.ToLower(filepath.Base(target))
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.ToLower(filepath.Base(path)) == want {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s was not extracted from Data1.cab", target)
	}
	return found, nil
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		fatal(err)
	}
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal(fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err))
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
