package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andreimerlescu/gematria"
	"github.com/andreimerlescu/textee"
)

// FileAppender opens a file with the specified mode and returns a buffered writer and file handle.
func FileAppender(filename string, mode int) (*bufio.Writer, *os.File, error) {
	file, err := os.OpenFile(filename, mode, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open %s due to err: %v", filename, err)
	}
	writer := bufio.NewWriter(file)
	return writer, file, nil
}

// ProcessOCRFile processes an OCR text file and returns PageData and postings.
func ProcessOCRFile(path string, pageID int) (*PageData, []string, []string, error) {
	relPath := filepath.Dir(path)
	if !strings.HasSuffix(relPath, "pages") {
		return nil, nil, nil, nil // Skip if not in 'pages' directory
	}

	// record.json contains the document identifier
	docDir := filepath.Dir(relPath)
	var dataInRecordJson = make(map[string]interface{})
	recordJsonBytes, readErr := os.ReadFile(filepath.Join(docDir, "record.json"))
	if readErr != nil {
		return nil, nil, nil, readErr
	}
	jsonErr := json.Unmarshal(recordJsonBytes, &dataInRecordJson)
	if jsonErr != nil {
		return nil, nil, nil, jsonErr
	}
	documentIdentifier, ok := dataInRecordJson["identifier"].(string)
	if !ok {
		return nil, nil, nil, errors.New("no such field identifier in record.json")
	}

	// the page number is in the filename of the ocr.######.txt
	var pageNumber int
	_, err := fmt.Sscanf(filepath.Base(path), "ocr.%06d.txt", &pageNumber)
	if err != nil {
		fmt.Printf("Error parsing filename: %v\n", err)
	}

	// page.######.json contains the page identifier
	var dataInPageJson = make(map[string]interface{})
	pageJsonBytes, readErr := os.ReadFile(filepath.Join(relPath, fmt.Sprintf("page.%06d.json", pageNumber)))
	if readErr != nil {
		return nil, nil, nil, readErr
	}
	jsonErr = json.Unmarshal(pageJsonBytes, &dataInPageJson)
	if jsonErr != nil {
		return nil, nil, nil, jsonErr
	}
	pageIdentifier, ok := dataInPageJson["identifier"].(string)
	if !ok {
		return nil, nil, nil, fmt.Errorf("no such field identifier in page.%06d.json", pageNumber)
	}

	// page.000001.json contains the cover page identifier
	var dataInCoverPageJson = make(map[string]interface{})
	coverPageJsonBytes, readErr := os.ReadFile(filepath.Join(relPath, "page.000001.json"))
	if readErr != nil {
		return nil, nil, nil, readErr
	}
	jsonErr = json.Unmarshal(coverPageJsonBytes, &dataInCoverPageJson)
	if jsonErr != nil {
		return nil, nil, nil, jsonErr
	}
	coverPageIdentifier, ok := dataInCoverPageJson["identifier"].(string)
	if !ok {
		return nil, nil, nil, errors.New("missing identifier field in page.000001.json")
	}

	// gather the identifiers
	pageData := &PageData{
		PageIdentifier:      pageIdentifier,
		DocumentIdentifier:  documentIdentifier,
		CoverPageIdentifier: coverPageIdentifier,
	}

	// read the ocr full text file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}

	// calculate textee data for result
	text, err := textee.NewTextee(string(content))
	if err != nil {
		return nil, nil, nil, err
	}
	pageData.Textee = text

	if len(pageData.Textee.Gematrias) == 0 && len(pageData.Textee.Substrings) > 0 {
		for substring, _ := range pageData.Textee.Substrings {
			pageData.Textee.Gematrias[substring] = gematria.FromString(substring)
		}
	}

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
