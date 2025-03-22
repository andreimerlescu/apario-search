package main

import (
	"bufio"
	"encoding/json"
	"github.com/andreimerlescu/textee"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
	offset, err := cacheFile.Seek(0, io.SeekCurrent)
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
