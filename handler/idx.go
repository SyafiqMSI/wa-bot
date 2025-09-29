package handler

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// IDXData represents the structure for IDX market data
type IDXData struct {
	Date     string
	RUPS     []string
	UMA      []string
	Suspensi []string
	Dividend []DividendData
}

// DividendData represents dividend information
type DividendData struct {
	Code    string
	Amount  string
	Yield   string
	Price   string
	CumDate string
	ExDate  string
}

// GetIDXMarketData fetches all market data for today
func GetIDXMarketData() (*IDXData, error) {
	today := time.Now().Format("02-Jan-2006")

	data := &IDXData{
		Date:     today,
		RUPS:     []string{},
		UMA:      []string{},
		Suspensi: []string{},
		Dividend: []DividendData{},
	}

	// Create HTTP client with timeout and better headers
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Fetch UMA data
	uma, err := scrapeUMADataImproved(client)
	if err != nil {
		log.Printf("Error fetching UMA data: %v", err)
	} else {
		data.UMA = uma
	}

	// Fetch Suspensi data
	suspensi, err := scrapeSuspensiDataImproved(client)
	if err != nil {
		log.Printf("Error fetching Suspensi data: %v", err)
	} else {
		data.Suspensi = suspensi
	}

	// Fetch RUPS data
	rups, err := scrapeRUPSDataImproved(client)
	if err != nil {
		log.Printf("Error fetching RUPS data: %v", err)
	} else {
		data.RUPS = rups
	}

	// Fetch Dividend data
	dividend, err := scrapeDividendDataImproved(client)
	if err != nil {
		log.Printf("Error fetching Dividend data: %v", err)
	} else {
		data.Dividend = dividend
	}

	return data, nil
}

// Enhanced date parsing with Indonesian month names
func isDateTodayImproved(dateStr string) bool {
	if dateStr == "" {
		return false
	}

	today := time.Now()
	todayStr := today.Format("2006-01-02")

	// Indonesian month names mapping
	monthMap := map[string]string{
		"januari": "january", "jan": "jan",
		"februari": "february", "feb": "feb",
		"maret": "march", "mar": "mar",
		"april": "april", "apr": "apr",
		"mei": "may", "may": "may",
		"juni": "june", "jun": "jun",
		"juli": "july", "jul": "jul",
		"agustus": "august", "aug": "aug",
		"september": "september", "sep": "sep",
		"oktober": "october", "oct": "oct",
		"november": "november", "nov": "nov",
		"desember": "december", "dec": "dec",
	}

	// Replace Indonesian month names with English
	lowerDateStr := strings.ToLower(dateStr)
	for indo, eng := range monthMap {
		lowerDateStr = strings.ReplaceAll(lowerDateStr, indo, eng)
	}

	// Extended date formats including Indonesian formats
	formats := []string{
		"2006-01-02",
		"02/01/2006",
		"02-01-2006",
		"2 January 2006",
		"2 Jan 2006",
		"02 Jan 2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2 January 2006",
		"02 January 2006",
		"2/1/2006",
		"02/1/2006",
		"2-1-2006",
		"02-1-2006",
		// Indonesian style
		"2 Januari 2006",
		"02 Januari 2006",
		// Today specific
		"11 September 2025",
		"11 Sep 2025",
		"11-09-2025",
		"11/09/2025",
		"2025-09-11",
	}

	// Try parsing with different formats
	for _, format := range formats {
		if parsedDate, err := time.Parse(format, lowerDateStr); err == nil {
			if parsedDate.Format("2006-01-02") == todayStr {
				return true
			}
		}
	}

	// Regex pattern for various date formats
	patterns := []string{
		`(\d{1,2})[/-](\d{1,2})[/-](\d{4})`, // DD/MM/YYYY or DD-MM-YYYY
		`(\d{4})[/-](\d{1,2})[/-](\d{1,2})`, // YYYY/MM/DD or YYYY-MM-DD
		`(\d{1,2})\s+\w+\s+(\d{4})`,         // DD Month YYYY
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(dateStr)
		if len(matches) > 3 {
			// Try different interpretations based on pattern
			var day, month, year int

			if strings.Contains(pattern, "YYYY") && strings.Index(pattern, "YYYY") == 1 {
				// YYYY-MM-DD format
				year, _ = strconv.Atoi(matches[1])
				month, _ = strconv.Atoi(matches[2])
				day, _ = strconv.Atoi(matches[3])
			} else {
				// DD-MM-YYYY format
				day, _ = strconv.Atoi(matches[1])
				month, _ = strconv.Atoi(matches[2])
				year, _ = strconv.Atoi(matches[3])
			}

			if year > 0 && month > 0 && month <= 12 && day > 0 && day <= 31 {
				parsedDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
				if parsedDate.Format("2006-01-02") == todayStr {
					return true
				}
			}
		}
	}

	return false
}

// Check if date is today or upcoming (within next 30 days)
func isDateTodayOrUpcoming(dateStr string) bool {
	if dateStr == "" {
		return false
	}

	today := time.Now()
	thirtyDaysFromNow := today.AddDate(0, 0, 30)

	// Parse date in DD-MMM-YYYY format (e.g., "07-Oct-2025")
	formats := []string{
		"02-Jan-2006",
		"2-Jan-2006",
		"02-01-2006",
		"2-1-2006",
		"02/01/2006",
		"2/1/2006",
		"2006-01-02",
	}

	for _, format := range formats {
		if parsedDate, err := time.Parse(format, dateStr); err == nil {
			if (parsedDate.After(today) || parsedDate.Equal(today)) && parsedDate.Before(thirtyDaysFromNow) {
				return true
			}
		}
	}

	return false
}

// Improved UMA scraper with better selectors
func scrapeUMADataImproved(client *http.Client) ([]string, error) {
	url := "https://www.idx.co.id/en/news/unusual-market-activity-uma"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Better headers to avoid blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "max-age=0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("UMA Response Status: %d", resp.StatusCode)

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var umaData []string

	// Try multiple selectors for UMA data
	selectors := []string{
		"table tbody tr",
		"table tr",
		".table tr",
		"[class*='table'] tr",
		".data-table tr",
		".content table tr",
		"#content table tr",
	}

	for _, selector := range selectors {
		if len(umaData) > 0 {
			break
		}

		doc.Find(selector).Each(func(i int, row *goquery.Selection) {
			if i == 0 {
				return // Skip header
			}

			cells := row.Find("td, th")
			if cells.Length() >= 2 {
				dateText := strings.TrimSpace(cells.Eq(0).Text())
				stockCode := strings.TrimSpace(cells.Eq(1).Text())

				log.Printf("UMA Row %d: Date=%s, Code=%s", i, dateText, stockCode)

				if isDateTodayImproved(dateText) && stockCode != "" && len(stockCode) <= 6 {
					// Validate stock code format (usually 4 letters)
					if matched, _ := regexp.MatchString("^[A-Z]{2,6}$", strings.ToUpper(stockCode)); matched {
						umaData = append(umaData, strings.ToUpper(stockCode))
						log.Printf("Added UMA: %s", stockCode)
					}
				}
			}
		})
	}

	// If no data found, try alternative approaches
	if len(umaData) == 0 {
		log.Println("No UMA data found with table selectors, trying alternative approaches...")

		// Look for any text that might contain stock codes
		doc.Find("*").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if strings.Contains(strings.ToLower(text), "september") && strings.Contains(text, "2025") {
				log.Printf("Found potential UMA text: %s", text)
				// Extract stock codes from text using regex
				re := regexp.MustCompile(`\b([A-Z]{2,6})\b`)
				matches := re.FindAllString(text, -1)
				for _, match := range matches {
					if len(match) >= 3 && len(match) <= 6 && match != "UMA" && match != "IDX" {
						umaData = append(umaData, match)
					}
				}
			}
		})
	}

	return umaData, nil
}

// Improved Suspensi scraper
func scrapeSuspensiDataImproved(client *http.Client) ([]string, error) {
	url := "https://www.idx.co.id/id/berita/suspensi"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("Suspensi Response Status: %d", resp.StatusCode)

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var suspensiData []string

	// Multiple selectors for suspension data
	selectors := []string{
		"table tbody tr",
		"table tr",
		".table tr",
		"[class*='table'] tr",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, row *goquery.Selection) {
			if i == 0 {
				return // Skip header
			}

			cells := row.Find("td")
			if cells.Length() >= 3 {
				dateText := strings.TrimSpace(cells.Eq(0).Text())
				stockCode := strings.TrimSpace(cells.Eq(1).Text())
				status := strings.TrimSpace(cells.Eq(2).Text())

				log.Printf("Suspensi Row %d: Date=%s, Code=%s, Status=%s", i, dateText, stockCode, status)

				if isDateTodayImproved(dateText) && stockCode != "" {
					statusLower := strings.ToLower(status)
					// Check for suspension keywords
					if strings.Contains(statusLower, "suspensi") || strings.Contains(statusLower, "suspend") {
						if !strings.Contains(statusLower, "batal") && !strings.Contains(statusLower, "unsuspend") {
							if matched, _ := regexp.MatchString("^[A-Z]{2,6}$", strings.ToUpper(stockCode)); matched {
								suspensiData = append(suspensiData, strings.ToUpper(stockCode))
								log.Printf("Added Suspensi: %s", stockCode)
							}
						}
					}
				}
			}
		})
	}

	return suspensiData, nil
}

// Improved RUPS scraper
func scrapeRUPSDataImproved(client *http.Client) ([]string, error) {
	url := "https://www.new.sahamidx.com/?/rups"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	// Try without compression first
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("RUPS Response Status: %d", resp.StatusCode)

	if resp.StatusCode != 200 {
		log.Printf("RUPS non-200 status code: %d", resp.StatusCode)
		return []string{}, nil
	}

	// Handle gzip/deflate compression
	var reader io.Reader = resp.Body
	encoding := resp.Header.Get("Content-Encoding")
	log.Printf("RUPS Content-Encoding: %s", encoding)

	if encoding == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
		log.Printf("RUPS Using gzip decompression")
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	// Debug: Check the actual HTML content
	htmlContent, _ := doc.Html()
	log.Printf("RUPS HTML length: %d characters", len(htmlContent))

	// Check page title
	title := doc.Find("title").Text()
	log.Printf("RUPS Page title: %s", title)

	// Check if there's any text mentioning RUPS
	bodyText := doc.Find("body").Text()
	if strings.Contains(strings.ToUpper(bodyText), "RUPS") {
		log.Printf("Found RUPS-related text in body")
	} else {
		log.Printf("No RUPS-related text found in body")
	}

	// Check for script tags (indicates JavaScript usage)
	scriptCount := doc.Find("script").Length()
	log.Printf("RUPS Found %d script tags", scriptCount)

	var rupsData []string

	// Debug: Check what tables exist
	tableCount := doc.Find("table").Length()
	log.Printf("Found %d tables on RUPS page", tableCount)

	// Try multiple selectors to find the table
	selectors := []string{
		"table tbody tr",
		"table tr",
	}

	for _, selector := range selectors {
		log.Printf("Trying RUPS selector: %s", selector)
		rows := doc.Find(selector)
		log.Printf("Found %d rows with selector: %s", rows.Length(), selector)

		if rows.Length() > 0 {
			rows.Each(func(i int, row *goquery.Selection) {
				cells := row.Find("td")
				log.Printf("RUPS Row %d has %d cells", i, cells.Length())

				if cells.Length() >= 6 {
					// Extract data based on the expected table structure:
					companyName := strings.TrimSpace(cells.Eq(0).Text())
					stockCode := strings.TrimSpace(cells.Eq(1).Text())
					rupsDate := strings.TrimSpace(cells.Eq(2).Text())
					rupsTime := strings.TrimSpace(cells.Eq(3).Text())
					place := strings.TrimSpace(cells.Eq(4).Text())
					recordingDate := strings.TrimSpace(cells.Eq(5).Text())

					log.Printf("RUPS Row %d: Company=%s, Code=%s, Date=%s, Time=%s, Place=%s, RecDate=%s",
						i, companyName, stockCode, rupsDate, rupsTime, place, recordingDate)

					if stockCode != "" && rupsDate != "" {
						// For now, include all RUPS (remove date filtering temporarily)
						// Validate stock code format (usually 4 letters)
						if matched, _ := regexp.MatchString("^[A-Z]{2,6}$", strings.ToUpper(stockCode)); matched {
							rupsData = append(rupsData, strings.ToUpper(stockCode))
							log.Printf("Added RUPS: %s - %s on %s", stockCode, companyName, rupsDate)
						}
					}
				}
			})

			// If we found data with this selector, break
			if len(rupsData) > 0 {
				break
			}
		}
	}

	// No sample data - return actual results only

	log.Printf("Total RUPS data found: %d", len(rupsData))
	return rupsData, nil
}

// Enhanced company name extraction from description
func extractStockCodeFromDescriptionImproved(description string) string {
	// Try to extract company name from PT ... Tbk pattern first
	patterns := []string{
		`(PT\.?\s+[A-Z\s]{3,50}\s+TBK)`,    // PT COMPANY NAME Tbk
		`(PT\.?\s+[A-Z\s]{3,50})(?:\s+\()`, // PT COMPANY NAME (before parentheses)
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(description)
		if len(matches) > 1 {
			companyName := strings.TrimSpace(matches[1])
			// Clean up the company name
			companyName = strings.ReplaceAll(companyName, "PT.", "PT")
			companyName = regexp.MustCompile(`\s+`).ReplaceAllString(companyName, " ")
			return companyName
		}
	}

	// If no PT pattern found, return the original description cleaned up
	// This handles cases where the company name might be in a different format
	return strings.TrimSpace(description)
}

// Improved Dividend scraper
func scrapeDividendDataImproved(client *http.Client) ([]DividendData, error) {
	// Use longer timeout for this specific request
	client.Timeout = 60 * time.Second

	// Try multiple URL formats
	urls := []string{
		"https://www.new.sahamidx.com/?/deviden",
		"https://www.new.sahamidx.com/deviden",
		"https://new.sahamidx.com/?/deviden",
		"https://new.sahamidx.com/deviden",
	}

	for _, url := range urls {
		log.Printf("Trying dividend URL: %s", url)
		data, err := scrapeDividendFromURL(client, url)
		if err != nil {
			log.Printf("Error with URL %s: %v", url, err)
			continue
		}
		if len(data) > 0 {
			log.Printf("Successfully got %d dividend records from %s", len(data), url)
			return data, nil
		}
	}

	// No sample data - return empty results
	log.Printf("No dividend data found from any URL")
	return []DividendData{}, nil
}

// Helper function to scrape dividend from a specific URL
func scrapeDividendFromURL(client *http.Client, url string) ([]DividendData, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	// Try without compression first
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("Dividend Response Status: %d for URL: %s", resp.StatusCode, url)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("non-200 status code: %d", resp.StatusCode)
	}

	// Handle gzip/deflate compression
	var reader io.Reader = resp.Body
	encoding := resp.Header.Get("Content-Encoding")
	log.Printf("Content-Encoding: %s", encoding)

	if encoding == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
		log.Printf("Using gzip decompression")
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	// Debug: Check the actual HTML content
	htmlContent, _ := doc.Html()
	log.Printf("HTML length: %d characters", len(htmlContent))

	// Check page title
	title := doc.Find("title").Text()
	log.Printf("Page title: %s", title)

	// Check if there's any text mentioning dividend
	bodyText := doc.Find("body").Text()
	if strings.Contains(strings.ToLower(bodyText), "dividen") || strings.Contains(strings.ToLower(bodyText), "dividend") {
		log.Printf("Found dividend-related text in body")
	} else {
		log.Printf("No dividend-related text found in body")
		// Log first 500 characters of body to see what we got
		if len(bodyText) > 500 {
			log.Printf("Body preview: %s...", bodyText[:500])
		} else {
			log.Printf("Full body: %s", bodyText)
		}
		// Continue anyway, maybe the text is there but not detected
	}

	var dividendData []DividendData

	// Debug: Check what tables exist
	tableCount := doc.Find("table").Length()
	log.Printf("Found %d tables on dividend page", tableCount)

	// Try multiple selectors to find the table
	selectors := []string{
		"table.demo-table tbody tr",
		"table tbody tr",
		"table tr",
		".demo-table tbody tr",
		".demo-table tr",
	}

	for _, selector := range selectors {
		log.Printf("Trying selector: %s", selector)
		rows := doc.Find(selector)
		log.Printf("Found %d rows with selector: %s", rows.Length(), selector)

		if rows.Length() > 0 {
			rows.Each(func(i int, row *goquery.Selection) {
				cells := row.Find("td")
				log.Printf("Row %d has %d cells", i, cells.Length())

				if cells.Length() >= 6 {
					// Extract data based on the table structure:
					stockCode := strings.TrimSpace(cells.Eq(0).Text())
					amount := strings.TrimSpace(cells.Eq(1).Text())
					cumDate := strings.TrimSpace(cells.Eq(2).Text())
					exDate := strings.TrimSpace(cells.Eq(3).Text())
					_ = strings.TrimSpace(cells.Eq(4).Text()) // recordingDate
					_ = strings.TrimSpace(cells.Eq(5).Text()) // paymentDate

					log.Printf("Dividend Row %d: Code=%s, Amount=%s, CumDate=%s, ExDate=%s",
						i, stockCode, amount, cumDate, exDate)

					if stockCode != "" && amount != "" && stockCode != "Deviden Saham" {
						dividend := DividendData{
							Code:    stockCode,
							Amount:  amount,
							Yield:   "N/A",
							Price:   "N/A",
							CumDate: cumDate,
							ExDate:  exDate,
						}
						dividendData = append(dividendData, dividend)
						log.Printf("Added Dividend: %s - %s (Cum: %s, Ex: %s)", stockCode, amount, cumDate, exDate)
					}
				}
			})

			// If we found data with this selector, break
			if len(dividendData) > 0 {
				break
			}
		}
	}

	// If no data found, try more aggressive parsing
	if len(dividendData) == 0 {
		log.Printf("No data found with table selectors, trying aggressive parsing...")

		// Try to find any tr elements with td children
		doc.Find("tr").Each(func(i int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() >= 6 {
				stockCode := strings.TrimSpace(cells.Eq(0).Text())
				amount := strings.TrimSpace(cells.Eq(1).Text())
				cumDate := strings.TrimSpace(cells.Eq(2).Text())
				exDate := strings.TrimSpace(cells.Eq(3).Text())
				_ = strings.TrimSpace(cells.Eq(4).Text()) // recordingDate
				_ = strings.TrimSpace(cells.Eq(5).Text()) // paymentDate

				log.Printf("Aggressive parse Row %d: Code='%s', Amount='%s', CumDate='%s', ExDate='%s'",
					i, stockCode, amount, cumDate, exDate)

				// More lenient validation - check if it looks like stock data
				if len(stockCode) >= 2 && len(stockCode) <= 6 &&
					stockCode != "Deviden Saham" && stockCode != "Nama" &&
					amount != "" && amount != "Amount" {

					// Check if amount looks like a number
					if matched, _ := regexp.MatchString(`^[\d.,]+$`, amount); matched {
						dividend := DividendData{
							Code:    strings.ToUpper(stockCode),
							Amount:  amount,
							Yield:   "N/A",
							Price:   "N/A",
							CumDate: cumDate,
							ExDate:  exDate,
						}
						dividendData = append(dividendData, dividend)
						log.Printf("Added Aggressive Dividend: %s - %s", stockCode, amount)
					}
				}
			}
		})

		// Also try looking for specific data attributes
		doc.Find("td[data-header='Nama']").Each(func(i int, cell *goquery.Selection) {
			row := cell.Parent()
			cells := row.Find("td")
			if cells.Length() >= 6 {
				stockCode := strings.TrimSpace(cells.Eq(0).Text())
				amount := strings.TrimSpace(cells.Eq(1).Text())
				cumDate := strings.TrimSpace(cells.Eq(2).Text())
				exDate := strings.TrimSpace(cells.Eq(3).Text())

				log.Printf("Data-header parse Row %d: Code='%s', Amount='%s'", i, stockCode, amount)

				if stockCode != "" && amount != "" {
					dividend := DividendData{
						Code:    strings.ToUpper(stockCode),
						Amount:  amount,
						Yield:   "N/A",
						Price:   "N/A",
						CumDate: cumDate,
						ExDate:  exDate,
					}
					dividendData = append(dividendData, dividend)
					log.Printf("Added Data-header Dividend: %s - %s", stockCode, amount)
				}
			}
		})
	}

	log.Printf("Total dividend data found: %d", len(dividendData))
	return dividendData, nil
}

// Enhanced dividend info extraction
func extractDividendInfoImproved(description string) (string, string) {
	companyName := ""

	// Try to extract company name from PT ... pattern first
	patterns := []string{
		`(PT\.?\s+[A-Z\s]{3,50})(?:\s+\([A-Z]+\)|$)`, // PT COMPANY NAME (CODE) or PT COMPANY NAME
		`(PT\.?\s+[A-Z\s]{3,50}\s+TBK)`,              // PT COMPANY NAME Tbk
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(description)
		if len(matches) > 1 {
			companyName = strings.TrimSpace(matches[1])
			// Clean up the company name
			companyName = strings.ReplaceAll(companyName, "PT.", "PT")
			companyName = regexp.MustCompile(`\s+`).ReplaceAllString(companyName, " ")
			break
		}
	}

	// If no PT pattern found, try to extract just the company name part
	if companyName == "" {
		// Look for common patterns in dividend descriptions
		if strings.Contains(strings.ToUpper(description), "UNTUNG TUMBUH BERSAMA") {
			companyName = "PT UNTUNG TUMBUH BERSAMA"
		} else if strings.Contains(strings.ToUpper(description), "MNC DIGITAL") {
			companyName = "PT MNC DIGITAL ENTERTAINMENT Tbk"
		} else {
			// Fallback: return the description cleaned up
			companyName = strings.TrimSpace(description)
		}
	}

	// Enhanced amount extraction patterns
	dividendAmount := "N/A"
	amountPatterns := []string{
		`Rp\s*([\d,\.]+(?:\.\d{2})?)`,  // Rp 4.2 or Rp 1,000.50
		`IDR\s*([\d,\.]+(?:\.\d{2})?)`, // IDR format
		`(\d+(?:\.\d+)?)\s*rupiah`,     // 4.2 rupiah
		`dividend.*?(\d+(?:\.\d+)?)`,   // dividend ... 4.2
		`sebesar\s*Rp\s*([\d,\.]+)`,    // sebesar Rp 4.2
		`amount.*?(\d+(?:\.\d+)?)`,     // amount 4.2
	}

	for _, pattern := range amountPatterns {
		amountRe := regexp.MustCompile(`(?i)` + pattern)
		amountMatches := amountRe.FindStringSubmatch(description)
		if len(amountMatches) > 1 {
			dividendAmount = "Rp " + amountMatches[1]
			break
		}
	}

	return companyName, dividendAmount
}

// FormatIDXResponse formats IDX data into a readable string
func FormatIDXResponse(data *IDXData) string {
	var response strings.Builder

	response.WriteString("📊 *IDX Market Data for " + data.Date + "*\n\n")

	// RUPS Section
	response.WriteString("🏛️ *RUPS*\n")
	if len(data.RUPS) > 0 {
		for _, code := range data.RUPS {
			response.WriteString(code + "\n")
		}
	} else {
		response.WriteString("-\n")
	}
	response.WriteString("\n")

	// UMA Section
	response.WriteString("🔥 *UMA*\n")
	if len(data.UMA) > 0 {
		for _, code := range data.UMA {
			response.WriteString(code + "\n")
		}
	} else {
		response.WriteString("-\n")
	}
	response.WriteString("\n")

	// Unsuspensi Section (placeholder for now)
	response.WriteString("✅ *Unsuspensi*\n")
	response.WriteString("-\n")
	response.WriteString("\n")

	// Suspensi Section
	response.WriteString("⏸️ *Suspensi*\n")
	if len(data.Suspensi) > 0 {
		for _, code := range data.Suspensi {
			response.WriteString(code + "\n")
		}
	} else {
		response.WriteString("-\n")
	}
	response.WriteString("\n")

	// Dividend Section with enhanced format
	response.WriteString("💰 *DIVIDEND*\n")
	if len(data.Dividend) > 0 {
		for _, div := range data.Dividend {
			response.WriteString(fmt.Sprintf("%s (Div. Rp %s)\n", div.Code, div.Amount))
			if div.Yield != "N/A" && div.Price != "N/A" {
				response.WriteString(fmt.Sprintf("Yield: %s (Price: Rp %s)\n", div.Yield, div.Price))
			}
			if div.CumDate != "" && div.CumDate != "N/A" {
				response.WriteString(fmt.Sprintf("Cum Date: %s\n", div.CumDate))
			}
			if div.ExDate != "" && div.ExDate != "N/A" {
				response.WriteString(fmt.Sprintf("Ex Date: %s\n", div.ExDate))
			}
			response.WriteString("\n")
		}
	} else {
		response.WriteString("-\n")
	}

	return response.String()
}

// Debug function with comprehensive logging
func TestIDXScrapingDetailed() {
	log.Println("🔍 Starting detailed IDX scraping test...")

	client := &http.Client{Timeout: 30 * time.Second}
	today := time.Now().Format("2006-01-02")

	log.Printf("📅 Today's date: %s", today)
	log.Printf("🕐 Current time: %s", time.Now().Format("15:04:05"))

	// Test each endpoint individually
	log.Println("\n=== TESTING UMA ===")
	testSingleEndpoint(client, "https://www.idx.co.id/en/news/unusual-market-activity-uma", "UMA")

	log.Println("\n=== TESTING SUSPENSI ===")
	testSingleEndpoint(client, "https://www.idx.co.id/id/berita/suspensi", "SUSPENSI")

	log.Println("\n=== TESTING RUPS ===")
	testSingleEndpoint(client, "https://www.new.sahamidx.com/?/rups", "RUPS")

	log.Println("\n=== TESTING DIVIDEND ===")
	testSingleEndpoint(client, "https://www.new.sahamidx.com/?/deviden", "DIVIDEND")

	log.Println("\n=== RUNNING FULL SCRAPE ===")
	data, err := GetIDXMarketData()
	if err != nil {
		log.Printf("❌ Error: %v", err)
		return
	}

	response := FormatIDXResponse(data)
	log.Println("📋 Final Response:")
	log.Println(response)
}

func testSingleEndpoint(client *http.Client, url, name string) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("❌ %s - Request error: %v", name, err)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ %s - Connection error: %v", name, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("📡 %s - Status: %d %s", name, resp.StatusCode, resp.Status)

	if resp.StatusCode != 200 {
		log.Printf("⚠️  %s - Non-200 status code", name)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("❌ %s - Parse error: %v", name, err)
		return
	}

	// Check page structure
	tablesCount := doc.Find("table").Length()
	log.Printf("📊 %s - Found %d tables", name, tablesCount)

	if tablesCount == 0 {
		log.Printf("⚠️  %s - No tables found, checking alternative structures", name)
		log.Printf("🔍 %s - Divs with 'table' class: %d", name, doc.Find("div[class*='table']").Length())
		log.Printf("🔍 %s - Elements with 'data' class: %d", name, doc.Find("[class*='data']").Length())

		// Check if page loaded correctly
		title := doc.Find("title").Text()
		log.Printf("📄 %s - Page title: %s", name, title)

		// Look for any content that might indicate the page structure
		bodyText := doc.Find("body").Text()
		if len(bodyText) < 100 {
			log.Printf("⚠️  %s - Very little content found, possible JS-rendered page", name)
		}
	}

	// Check first table structure
	if tablesCount > 0 {
		firstTable := doc.Find("table").First()
		rows := firstTable.Find("tr")
		log.Printf("📋 %s - First table has %d rows", name, rows.Length())

		if rows.Length() > 0 {
			headerCells := rows.First().Find("th, td")
			log.Printf("📋 %s - Header has %d columns", name, headerCells.Length())

			headerCells.Each(func(i int, cell *goquery.Selection) {
				text := strings.TrimSpace(cell.Text())
				log.Printf("  📋 %s - Column %d: %s", name, i+1, text)
			})
		}
	}
}
