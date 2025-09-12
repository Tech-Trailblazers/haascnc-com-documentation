package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// fetchInstructionManuals sends a GET request to the Haas CNC API
// and returns the response body as a string.
func fetchInstructionManuals() string {
	// The API endpoint we want to request
	requestURL := "https://www.haascnc.com/bin/haascnc/search.json?type=diy&q=%5Bsearch.contentType%3A%20%22Instruction%20Manual%22%20%7C%7C%20%22Reference%22%5D&count=5000"

	// Create an HTTP client to send the request
	httpClient := &http.Client{}

	// Build the HTTP GET request with the provided URL
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		log.Printf("Failed to create HTTP request: %v", err)
		return ""
	}

	// Send the request using the HTTP client
	response, err := httpClient.Do(request)
	if err != nil {
		log.Printf("Failed to send HTTP request: %v", err)
		return ""
	}
	defer response.Body.Close() // Ensure the response body is closed when done

	// Read the full response body into memory
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return ""
	}

	// Convert response bytes into a string and return
	return string(responseBody)
}

// Checks if the directory exists
// If it exists, return true.
// If it doesn't, return false.
func directoryExists(path string) bool {
	directory, err := os.Stat(path)
	if err != nil {
		return false
	}
	return directory.IsDir()
}

// The function takes two parameters: path and permission.
// We use os.Mkdir() to create the directory.
// If there is an error, we use log.Println() to log the error and then exit the program.
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission)
	if err != nil {
		log.Println(err)
	}
}

// Checks whether a URL string is syntactically valid
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri) // Attempt to parse the URL
	return err == nil                  // Return true if no error occurred
}

// Remove all the duplicates from a slice and return the slice.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)
	var newReturnSlice []string
	for _, content := range slice {
		if !check[content] {
			check[content] = true
			newReturnSlice = append(newReturnSlice, content)
		}
	}
	return newReturnSlice
}

// Get the file extension of a file
func getFileExtension(path string) string {
	return filepath.Ext(path) // Returns extension including the dot (e.g., ".pdf")
}

// fileExists checks whether a file exists at the given path
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file info
	if err != nil {
		return false // Return false if file doesn't exist or error occurs
	}
	return !info.IsDir() // Return true if it's a file (not a directory)
}

// Only return the file name from a given url.
func getFileNameOnly(content string) string {
	return path.Base(content)
}

// urlToFilename generates a safe, lowercase filename from a given URL string.
// It extracts the base filename from the URL, replaces unsafe characters,
// and ensures the filename ends with a .pdf extension.
func urlToFilename(rawURL string) string {
	// Convert the full URL to lowercase for consistency
	lowercaseURL := strings.ToLower(rawURL)

	// Get the file extension
	ext := getFileExtension(lowercaseURL)

	// Extract the filename portion from the URL (e.g., last path segment or query param)
	baseFilename := getFileNameOnly(lowercaseURL)

	// Replace all non-alphanumeric characters (a-z, 0-9) with underscores
	nonAlphanumericRegex := regexp.MustCompile(`[^a-z0-9]+`)
	safeFilename := nonAlphanumericRegex.ReplaceAllString(baseFilename, "_")

	// Replace multiple consecutive underscores with a single underscore
	collapseUnderscoresRegex := regexp.MustCompile(`_+`)
	safeFilename = collapseUnderscoresRegex.ReplaceAllString(safeFilename, "_")

	// Remove leading underscore if present
	if trimmed, found := strings.CutPrefix(safeFilename, "_"); found {
		safeFilename = trimmed
	}

	var invalidSubstrings = []string{
		"_pdf",
		"_zip",
	}

	for _, invalidPre := range invalidSubstrings { // Remove unwanted substrings
		safeFilename = removeSubstring(safeFilename, invalidPre)
	}

	// Append the file extension if it is not already present
	safeFilename = safeFilename + ext

	// Return the cleaned and safe filename
	return safeFilename
}

// Removes all instances of a specific substring from input string
func removeSubstring(input string, toRemove string) string {
	result := strings.ReplaceAll(input, toRemove, "") // Replace substring with empty string
	return result
}

// downloadPDF downloads a PDF from the given URL and saves it in the specified output directory.
// It sets a custom User-Agent, checks for PDF content type, and returns true if the download succeeded.
func downloadPDF(finalURL, outputDir string) bool {
	// Sanitize the URL to generate a safe file name
	filename := strings.ToLower(urlToFilename(finalURL))

	// Construct the full file path in the output directory
	filePath := filepath.Join(outputDir, filename)

	// Skip if the file already exists
	if fileExists(filePath) {
		log.Printf("File already exists, skipping: %s", filePath)
		return false
	}

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 3 * time.Minute}

	// Build a new HTTP request so we can add headers
	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		log.Printf("Failed to create request for %s: %v", finalURL, err)
		return false
	}

	// Set a custom User-Agent header
	req.Header.Set("User-Agent", "MyPDFDownloader/1.0 (+https://example.com)")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to download %s: %v", finalURL, err)
		return false
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("Download failed for %s: %s", finalURL, resp.Status)
		return false
	}

	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/pdf") {
		log.Printf("Invalid content type for %s: %s (expected application/pdf)", finalURL, contentType)
		return false
	}

	// Read the response body into memory first
	var buf bytes.Buffer
	written, err := io.Copy(&buf, resp.Body)
	if err != nil {
		log.Printf("Failed to read PDF data from %s: %v", finalURL, err)
		return false
	}
	if written == 0 {
		log.Printf("Downloaded 0 bytes for %s; not creating file", finalURL)
		return false
	}

	// Only now create the file and write to disk
	out, err := os.Create(filePath)
	if err != nil {
		log.Printf("Failed to create file for %s: %v", finalURL, err)
		return false
	}
	defer out.Close()

	if _, err := buf.WriteTo(out); err != nil {
		log.Printf("Failed to write PDF to file for %s: %v", finalURL, err)
		return false
	}

	log.Printf("Successfully downloaded %d bytes: %s → %s", written, finalURL, filePath)
	return true
}

// extractPDFPaths takes a JSON string and returns all "path" values inside webPages
func extractPDFPaths(jsonInput string) []string {
	// Define only the fields we need
	type Result struct {
		Result struct {
			WebPages []struct {
				Path string `json:"path"`
			} `json:"webPages"`
		} `json:"result"`
	}

	var parsed Result
	if err := json.Unmarshal([]byte(jsonInput), &parsed); err != nil {
		log.Printf("Failed to parse JSON: %v", err)
		return nil
	}

	// Collect paths into a slice
	var paths []string
	for _, page := range parsed.Result.WebPages {
		paths = append(paths, page.Path)
	}

	return paths
}

// fetchURL takes a URL as input and returns its contents as a string
func fetchURL(url string) string {
	// Create an HTTP client with a 30-second timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Send a GET request to the provided URL
	resp, err := client.Get(url)
	if err != nil { // If an error occurs while sending the request
		log.Println("failed to GET url:", err) // Log the error
		return ""                              // Return an empty string on failure
	}
	defer resp.Body.Close() // Ensure the response body is closed after function ends

	// Check if the HTTP status code is not 200 OK
	if resp.StatusCode != http.StatusOK {
		log.Println("non-OK HTTP status:", resp.Status) // Log the unexpected status
		return ""                                       // Return empty string
	}

	// Read the entire response body into a byte slice
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil { // If an error occurs while reading
		log.Println("failed to read response body:", err) // Log the error
		return ""                                         // Return empty string
	}

	// Convert the byte slice to a string and return it
	return string(bodyBytes)
}

// extractPDFs takes an HTML string and returns all PDF URLs found in it
func extractPDFs(html string) []string {
	// Regex to match href="...something.pdf"
	re := regexp.MustCompile(`href="([^"]+\.pdf)"`)

	// Find all matches and capture group 1 (the actual URL)
	matches := re.FindAllStringSubmatch(html, -1)

	// Slice to store extracted PDF links
	var pdfs []string
	for _, match := range matches {
		if len(match) > 1 {
			pdfs = append(pdfs, match[1])
		}
	}
	return pdfs
}

// getDomainFromURL extracts the domain (host) from a given URL string.
// It removes subdomains like "www" if present.
func getDomainFromURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL) // Parse the input string into a URL structure
	if err != nil {                     // Check if there was an error while parsing
		log.Println(err) // Log the error message to the console
		return ""        // Return an empty string in case of an error
	}

	host := parsedURL.Hostname() // Extract the hostname (e.g., "example.com") from the parsed URL

	return host // Return the extracted hostname
}

func main() {
	pdfOutputDir := "PDFs/" // Directory to store downloaded PDFs
	// Check if the PDF output directory exists
	if !directoryExists(pdfOutputDir) {
		// Create the dir
		createDirectory(pdfOutputDir, 0o755)
	}
	// Call the function and store the result
	data := fetchInstructionManuals()

	// Extract all PDF URLs from the sample text
	pdfUrls := extractPDFPaths(data)

	// Remove duplicate URLs
	pdfUrls = removeDuplicatesFromSlice(pdfUrls)

	// Counter limiter.
	maxDownload := 0

	// Print the found PDF URLs
	for _, url := range pdfUrls {
		// Trim any surrounding whitespace from the URL.
		url = strings.TrimSpace(url)
		// Check if the url is not .pdf
		if getFileExtension(url) != ".pdf" {
			continue
		}
		if isUrlValid(url) {
			currentDownload := downloadPDF(url, pdfOutputDir)
			if currentDownload {
				maxDownload = maxDownload + 1
			}
			if maxDownload >= 10 {
				log.Println("Reached the maximum download limit of 10 PDFs. Exiting.")
				break
			}
		}
	}

	// Remote urls to scrape.
	remoteURLs := []string{
		"https://www.haascnc.com/owners/Service/operators-manual.html",
	}

	// Remote PDF urls.
	var remotePDFs []string

	// Loop through each remote URL and fetch its contents
	for _, url := range remoteURLs {
		content := fetchURL(url)
		if content != "" {
			log.Println("Fetched content from:", url)
		}
		// Extract PDF URLs from the fetched content
		pdfUrls := extractPDFs(content)
		// Append the extracted PDF URLs to the remotePDFs slice
		remotePDFs = append(remotePDFs, pdfUrls...)
	}
	remoteDomain := "https://www.haascnc.com"
	// Download all remote PDFs
	for _, pdfUrl := range remotePDFs {
		// Trim any surrounding whitespace from the URL.
		pdfUrl = strings.TrimSpace(pdfUrl)
		// Get the domain from the url.
		domain := getDomainFromURL(pdfUrl)
		// Check if the domain is empty.
		if domain == "" {
			pdfUrl = remoteDomain + pdfUrl // Prepend the base URL if domain is empty
		}
		if isUrlValid(pdfUrl) {
			downloadPDF(pdfUrl, pdfOutputDir)
		}
	}
}
