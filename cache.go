package main

import (
	"bufio"
	"encoding/json"
	"github.com/RoaringBitmap/roaring"
	"github.com/andreimerlescu/textee"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func buildCache(dir string) (err error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Open files for writing (create mode)
	cacheWriter, cacheFile, err := FileAppender(cacheFile, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	idxWriter, idxFile, err := FileAppender(cacheIndexFile, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer idxFile.Close()

	wordWriter, wordFile, err := FileAppender("word_postings.txt", os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer wordFile.Close()

	gemWriter, gemFile, err := FileAppender("gematria_postings.txt", os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer gemFile.Close()

	pageID := 0
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasPrefix(info.Name(), "ocr.") || !strings.HasSuffix(info.Name(), ".txt") {
			return nil
		}

		pageData, wordPostings, gemPostings, err := ProcessOCRFile(path, dir, pageID)
		if err != nil {
			return err
		}
		if pageData == nil {
			return nil // Skip if not in 'pages'
		}

		// Append to cache and index
		if err := AppendToCache(cacheWriter, idxWriter, pageData, pageID, cacheFile); err != nil {
			return err
		}

		// Write postings
		for _, posting := range wordPostings {
			_, err = wordWriter.WriteString(posting + "\n")
			if err != nil {
				return err
			}
		}
		for _, posting := range gemPostings {
			_, err = gemWriter.WriteString(posting + "\n")
			if err != nil {
				return err
			}
		}

		pageID++
		return nil
	})
	if err != nil {
		return err
	}

	// Flush writers
	if err = cacheWriter.Flush(); err != nil {
		return err
	}
	if err = idxWriter.Flush(); err != nil {
		return err
	}
	if err = wordWriter.Flush(); err != nil {
		return err
	}
	if err = gemWriter.Flush(); err != nil {
		return err
	}

	// Build indexes
	if err = buildIndex("word_postings.txt", wordIndexFile); err != nil {
		return err
	}
	if err = buildIndex("gematria_postings.txt", gemIndexFile); err != nil {
		return err
	}
	return nil
}

func buildIndex(postingsFile, indexFile string) error {
	// Sort postings (simplified; use external sort for large files)
	lines, _ := os.ReadFile(postingsFile)
	sorted := strings.Split(string(lines), "\n")
	sort.Strings(sorted)

	// Build bitmaps
	out, _ := os.Create(indexFile)
	defer out.Close()
	writer := bufio.NewWriter(out)

	currentKey := ""
	bitmap := roaring.New()
	header := make(map[string][2]int64) // key -> [offset, length]
	_, _ = out.Seek(0, io.SeekCurrent)

	for _, line := range sorted {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		key, id := parts[0], parts[1]
		idInt, _ := strconv.Atoi(id)

		if key != currentKey && currentKey != "" {
			// Write previous bitmap
			offset, _ := out.Seek(0, io.SeekCurrent)
			data, _ := bitmap.MarshalBinary()
			_, err := writer.Write(data)
			if err != nil {
				return err
			}
			header[currentKey] = [2]int64{offset, int64(len(data))}
			bitmap.Clear()
		}
		currentKey = key
		bitmap.Add(uint32(idInt))
	}
	if currentKey != "" {
		offset, _ := out.Seek(0, io.SeekCurrent)
		data, _ := bitmap.MarshalBinary()
		_, err := writer.Write(data)
		if err != nil {
			return err
		}
		header[currentKey] = [2]int64{offset, int64(len(data))}
	}

	// Write header
	err := writer.Flush()
	if err != nil {
		return err
	}
	_, err = out.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = json.NewEncoder(out).Encode(header)
	if err != nil {
		return err
	}
	return nil
}

// FileAppender opens a file with the specified mode and returns a buffered writer and file handle.
func FileAppender(filename string, mode int) (*bufio.Writer, *os.File, error) {
	file, err := os.OpenFile(filename, mode, 0644)
	if err != nil {
		return nil, nil, err
	}
	writer := bufio.NewWriter(file)
	return writer, file, nil
}

// ProcessOCRFile processes an OCR text file and returns PageData and postings.
func ProcessOCRFile(path, baseDir string, pageID int) (*PageData, []string, []string, error) {
	relPath, err := filepath.Rel(baseDir, path)
	if err != nil || !strings.HasPrefix(relPath, "pages") {
		return nil, nil, nil, nil // Skip if not in 'pages' directory
	}

	docDir := filepath.Dir(filepath.Dir(relPath))
	pageData := &PageData{
		PageIdentifier:      relPath,
		DocumentIdentifier:  docDir,
		CoverPageIdentifier: filepath.Join(docDir, "pages", "page.000001.json"),
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}
	text, err := textee.NewTextee(string(content))
	if err != nil {
		return nil, nil, nil, err
	}
	pageData.Textee = text

	wordPostings := generateWordPostings(text, pageID)
	gemPostings := generateGematriaPostings(text, pageID)

	return pageData, wordPostings, gemPostings, nil
}

// AppendToCache appends PageData to the cache file and updates the index.
func AppendToCache(cacheWriter *bufio.Writer, idxWriter *bufio.Writer, pageData *PageData, pageID int, cacheFile *os.File) error {
	offset, err := cacheFile.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}
	data, err := json.Marshal(pageData)
	if err != nil {
		return err
	}
	_, err = cacheWriter.Write(data)
	if err != nil {
		return err
	}
	_, err = cacheWriter.WriteString("\n")
	if err != nil {
		return err
	}
	length := int64(len(data))
	_, err = idxWriter.WriteString(strconv.Itoa(pageID) + " " + strconv.FormatInt(offset, 10) + " " + strconv.FormatInt(length, 10) + "\n")
	return err
}

// generateWordPostings generates word postings for a given Textee and page ID.
func generateWordPostings(text *textee.Textee, pageID int) []string {
	var postings []string
	for word := range text.Gematrias {
		postings = append(postings, word+" "+strconv.Itoa(pageID))
	}
	return postings
}

// generateGematriaPostings generates gematria postings for a given Textee and page ID.
func generateGematriaPostings(text *textee.Textee, pageID int) []string {
	var postings []string
	for _, g := range text.Gematrias {
		postings = append(postings,
			"english_"+strconv.FormatUint(g.English, 10)+" "+strconv.Itoa(pageID),
			"simple_"+strconv.FormatUint(g.Simple, 10)+" "+strconv.Itoa(pageID),
			"jewish_"+strconv.FormatUint(g.Jewish, 10)+" "+strconv.Itoa(pageID),
			"mystery_"+strconv.FormatUint(g.Mystery, 10)+" "+strconv.Itoa(pageID),
			"majestic_"+strconv.FormatUint(g.Majestic, 10)+" "+strconv.Itoa(pageID),
			"eights_"+strconv.FormatUint(g.Eights, 10)+" "+strconv.Itoa(pageID),
		)
	}
	return postings
}
