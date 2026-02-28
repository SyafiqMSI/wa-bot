package idx

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

	"whatsmeow-api/domain"

	"github.com/PuerkitoBio/goquery"
)

func GetIDXMarketData() (*domain.IDXData, error) {
	today := time.Now().Format("02-Jan-2006")

	data := &domain.IDXData{
		Date:     today,
		RUPS:     []string{},
		UMA:      []string{},
		Suspensi: []string{},
		Dividend: []domain.DividendData{},
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	uma, err := scrapeUMADataImproved(client)
	if err != nil {
		log.Printf("Error fetching UMA data: %v", err)
	} else {
		data.UMA = uma
	}

	suspensi, err := scrapeSuspensiDataImproved(client)
	if err != nil {
		log.Printf("Error fetching Suspensi data: %v", err)
	} else {
		data.Suspensi = suspensi
	}

	rups, err := scrapeRUPSDataImproved(client)
	if err != nil {
		log.Printf("Error fetching RUPS data: %v", err)
	} else {
		data.RUPS = rups
	}

	dividend, err := scrapeDividendDataImproved(client)
	if err != nil {
		log.Printf("Error fetching Dividend data: %v", err)
	} else {
		data.Dividend = dividend
	}

	return data, nil
}

func isDateTodayImproved(dateStr string) bool {
	if dateStr == "" {
		return false
	}

	today := time.Now()
	todayStr := today.Format("2006-01-02")

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

	lowerDateStr := strings.ToLower(dateStr)
	for indo, eng := range monthMap {
		lowerDateStr = strings.ReplaceAll(lowerDateStr, indo, eng)
	}

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

		"2 Januari 2006",
		"02 Januari 2006",

		"11 September 2025",
		"11 Sep 2025",
		"11-09-2025",
		"11/09/2025",
		"2025-09-11",
	}

	for _, format := range formats {
		if parsedDate, err := time.Parse(format, lowerDateStr); err == nil {
			if parsedDate.Format("2006-01-02") == todayStr {
				return true
			}
		}
	}

	patterns := []string{
		`(\d{1,2})[/-](\d{1,2})[/-](\d{4})`,
		`(\d{4})[/-](\d{1,2})[/-](\d{1,2})`,
		`(\d{1,2})\s+\w+\s+(\d{4})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(dateStr)
		if len(matches) > 3 {

			var day, month, year int

			if strings.Contains(pattern, "YYYY") && strings.Index(pattern, "YYYY") == 1 {

				year, _ = strconv.Atoi(matches[1])
				month, _ = strconv.Atoi(matches[2])
				day, _ = strconv.Atoi(matches[3])
			} else {

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

func isDateTodayOrUpcoming(dateStr string) bool {
	if dateStr == "" {
		return false
	}

	today := time.Now()
	thirtyDaysFromNow := today.AddDate(0, 0, 30)

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

func scrapeUMADataImproved(client *http.Client) ([]string, error) {
	url := "https://www.idx.co.id/en/news/unusual-market-activity-uma"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

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
				return
			}

			cells := row.Find("td, th")
			if cells.Length() >= 2 {
				dateText := strings.TrimSpace(cells.Eq(0).Text())
				stockCode := strings.TrimSpace(cells.Eq(1).Text())

				log.Printf("UMA Row %d: Date=%s, Code=%s", i, dateText, stockCode)

				if isDateTodayImproved(dateText) && stockCode != "" && len(stockCode) <= 6 {

					if matched, _ := regexp.MatchString("^[A-Z]{2,6}$", strings.ToUpper(stockCode)); matched {
						umaData = append(umaData, strings.ToUpper(stockCode))
						log.Printf("Added UMA: %s", stockCode)
					}
				}
			}
		})
	}

	if len(umaData) == 0 {
		log.Println("No UMA data found with table selectors, trying alternative approaches...")

		doc.Find("*").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if strings.Contains(strings.ToLower(text), "september") && strings.Contains(text, "2025") {
				log.Printf("Found potential UMA text: %s", text)

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

	selectors := []string{
		"table tbody tr",
		"table tr",
		".table tr",
		"[class*='table'] tr",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, row *goquery.Selection) {
			if i == 0 {
				return
			}

			cells := row.Find("td")
			if cells.Length() >= 3 {
				dateText := strings.TrimSpace(cells.Eq(0).Text())
				stockCode := strings.TrimSpace(cells.Eq(1).Text())
				status := strings.TrimSpace(cells.Eq(2).Text())

				log.Printf("Suspensi Row %d: Date=%s, Code=%s, Status=%s", i, dateText, stockCode, status)

				if isDateTodayImproved(dateText) && stockCode != "" {
					statusLower := strings.ToLower(status)

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

func scrapeRUPSDataImproved(client *http.Client) ([]string, error) {
	url := "https://www.new.sahamidx.com/?/rups"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")

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

	htmlContent, _ := doc.Html()
	log.Printf("RUPS HTML length: %d characters", len(htmlContent))

	title := doc.Find("title").Text()
	log.Printf("RUPS Page title: %s", title)

	bodyText := doc.Find("body").Text()
	if strings.Contains(strings.ToUpper(bodyText), "RUPS") {
		log.Printf("Found RUPS-related text in body")
	} else {
		log.Printf("No RUPS-related text found in body")
	}

	scriptCount := doc.Find("script").Length()
	log.Printf("RUPS Found %d script tags", scriptCount)

	var rupsData []string

	tableCount := doc.Find("table").Length()
	log.Printf("Found %d tables on RUPS page", tableCount)

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

					companyName := strings.TrimSpace(cells.Eq(0).Text())
					stockCode := strings.TrimSpace(cells.Eq(1).Text())
					rupsDate := strings.TrimSpace(cells.Eq(2).Text())
					rupsTime := strings.TrimSpace(cells.Eq(3).Text())
					place := strings.TrimSpace(cells.Eq(4).Text())
					recordingDate := strings.TrimSpace(cells.Eq(5).Text())

					log.Printf("RUPS Row %d: Company=%s, Code=%s, Date=%s, Time=%s, Place=%s, RecDate=%s",
						i, companyName, stockCode, rupsDate, rupsTime, place, recordingDate)

					if stockCode != "" && rupsDate != "" {

						if matched, _ := regexp.MatchString("^[A-Z]{2,6}$", strings.ToUpper(stockCode)); matched {
							rupsData = append(rupsData, strings.ToUpper(stockCode))
							log.Printf("Added RUPS: %s - %s on %s", stockCode, companyName, rupsDate)
						}
					}
				}
			})

			if len(rupsData) > 0 {
				break
			}
		}
	}

	log.Printf("Total RUPS data found: %d", len(rupsData))
	return rupsData, nil
}

func extractStockCodeFromDescriptionImproved(description string) string {

	patterns := []string{
		`(PT\.?\s+[A-Z\s]{3,50}\s+TBK)`,
		`(PT\.?\s+[A-Z\s]{3,50})(?:\s+\()`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(description)
		if len(matches) > 1 {
			companyName := strings.TrimSpace(matches[1])

			companyName = strings.ReplaceAll(companyName, "PT.", "PT")
			companyName = regexp.MustCompile(`\s+`).ReplaceAllString(companyName, " ")
			return companyName
		}
	}

	return strings.TrimSpace(description)
}

func scrapeDividendDataImproved(client *http.Client) ([]domain.DividendData, error) {

	client.Timeout = 60 * time.Second

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

	log.Printf("No dividend data found from any URL")
	return []domain.DividendData{}, nil
}

func scrapeDividendFromURL(client *http.Client, url string) ([]domain.DividendData, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")

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

	htmlContent, _ := doc.Html()
	log.Printf("HTML length: %d characters", len(htmlContent))

	title := doc.Find("title").Text()
	log.Printf("Page title: %s", title)

	bodyText := doc.Find("body").Text()
	if strings.Contains(strings.ToLower(bodyText), "dividen") || strings.Contains(strings.ToLower(bodyText), "dividend") {
		log.Printf("Found dividend-related text in body")
	} else {
		log.Printf("No dividend-related text found in body")

		if len(bodyText) > 500 {
			log.Printf("Body preview: %s...", bodyText[:500])
		} else {
			log.Printf("Full body: %s", bodyText)
		}

	}

	var dividendData []domain.DividendData

	tableCount := doc.Find("table").Length()
	log.Printf("Found %d tables on dividend page", tableCount)

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

					stockCode := strings.TrimSpace(cells.Eq(0).Text())
					amount := strings.TrimSpace(cells.Eq(1).Text())
					cumDate := strings.TrimSpace(cells.Eq(2).Text())
					exDate := strings.TrimSpace(cells.Eq(3).Text())
					_ = strings.TrimSpace(cells.Eq(4).Text())
					_ = strings.TrimSpace(cells.Eq(5).Text())

					log.Printf("Dividend Row %d: Code=%s, Amount=%s, CumDate=%s, ExDate=%s",
						i, stockCode, amount, cumDate, exDate)

					if stockCode != "" && amount != "" && stockCode != "Deviden Saham" {
						dividend := domain.DividendData{
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

			if len(dividendData) > 0 {
				break
			}
		}
	}

	if len(dividendData) == 0 {
		log.Printf("No data found with table selectors, trying aggressive parsing...")

		doc.Find("tr").Each(func(i int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() >= 6 {
				stockCode := strings.TrimSpace(cells.Eq(0).Text())
				amount := strings.TrimSpace(cells.Eq(1).Text())
				cumDate := strings.TrimSpace(cells.Eq(2).Text())
				exDate := strings.TrimSpace(cells.Eq(3).Text())
				_ = strings.TrimSpace(cells.Eq(4).Text())
				_ = strings.TrimSpace(cells.Eq(5).Text())

				log.Printf("Aggressive parse Row %d: Code='%s', Amount='%s', CumDate='%s', ExDate='%s'",
					i, stockCode, amount, cumDate, exDate)

				if len(stockCode) >= 2 && len(stockCode) <= 6 &&
					stockCode != "Deviden Saham" && stockCode != "Nama" &&
					amount != "" && amount != "Amount" {

					if matched, _ := regexp.MatchString(`^[\d.,]+$`, amount); matched {
						dividend := domain.DividendData{
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
					dividend := domain.DividendData{
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

func extractDividendInfoImproved(description string) (string, string) {
	companyName := ""

	patterns := []string{
		`(PT\.?\s+[A-Z\s]{3,50})(?:\s+\([A-Z]+\)|$)`,
		`(PT\.?\s+[A-Z\s]{3,50}\s+TBK)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(description)
		if len(matches) > 1 {
			companyName = strings.TrimSpace(matches[1])

			companyName = strings.ReplaceAll(companyName, "PT.", "PT")
			companyName = regexp.MustCompile(`\s+`).ReplaceAllString(companyName, " ")
			break
		}
	}

	if companyName == "" {

		if strings.Contains(strings.ToUpper(description), "UNTUNG TUMBUH BERSAMA") {
			companyName = "PT UNTUNG TUMBUH BERSAMA"
		} else if strings.Contains(strings.ToUpper(description), "MNC DIGITAL") {
			companyName = "PT MNC DIGITAL ENTERTAINMENT Tbk"
		} else {

			companyName = strings.TrimSpace(description)
		}
	}

	dividendAmount := "N/A"
	amountPatterns := []string{
		`Rp\s*([\d,\.]+(?:\.\d{2})?)`,
		`IDR\s*([\d,\.]+(?:\.\d{2})?)`,
		`(\d+(?:\.\d+)?)\s*rupiah`,
		`dividend.*?(\d+(?:\.\d+)?)`,
		`sebesar\s*Rp\s*([\d,\.]+)`,
		`amount.*?(\d+(?:\.\d+)?)`,
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

func FormatIDXResponse(data *domain.IDXData) string {
	var response strings.Builder

	response.WriteString("[IDX Market Data for " + data.Date + "]\n\n")

	response.WriteString("[RUPS]\n")
	if len(data.RUPS) > 0 {
		for _, code := range data.RUPS {
			response.WriteString(code + "\n")
		}
	} else {
		response.WriteString("-\n")
	}
	response.WriteString("\n")

	response.WriteString("[UMA]\n")
	if len(data.UMA) > 0 {
		for _, code := range data.UMA {
			response.WriteString(code + "\n")
		}
	} else {
		response.WriteString("-\n")
	}
	response.WriteString("\n")

	response.WriteString("[Unsuspensi]\n")
	response.WriteString("-\n")
	response.WriteString("\n")

	response.WriteString("[Suspensi]\n")
	if len(data.Suspensi) > 0 {
		for _, code := range data.Suspensi {
			response.WriteString(code + "\n")
		}
	} else {
		response.WriteString("-\n")
	}
	response.WriteString("\n")

	response.WriteString("[DIVIDEND]\n")
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

func TestIDXScrapingDetailed() {
	log.Println("[IDX] Starting detailed IDX scraping test...")

	client := &http.Client{Timeout: 30 * time.Second}
	today := time.Now().Format("2006-01-02")

	log.Printf("[IDX] Today's date: %s", today)
	log.Printf("[IDX] Current time: %s", time.Now().Format("15:04:05"))

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
		log.Printf("[Error]: %v", err)
		return
	}

	response := FormatIDXResponse(data)
	log.Println("[IDX] Final Response:")
	log.Println(response)
}

func testSingleEndpoint(client *http.Client, url, name string) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[Error] %s - Request error: %v", name, err)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Error] %s - Connection error: %v", name, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("[IDX] %s - Status: %d %s", name, resp.StatusCode, resp.Status)

	if resp.StatusCode != 200 {
		log.Printf("[Warning] %s - Non-200 status code", name)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("[Error] %s - Parse error: %v", name, err)
		return
	}

	tablesCount := doc.Find("table").Length()
	log.Printf("[IDX] %s - Found %d tables", name, tablesCount)

	if tablesCount == 0 {
		log.Printf("[Warning] %s - No tables found, checking alternative structures", name)
		log.Printf("[IDX] %s - Divs with 'table' class: %d", name, doc.Find("div[class*='table']").Length())
		log.Printf("[IDX] %s - Elements with 'data' class: %d", name, doc.Find("[class*='data']").Length())

		title := doc.Find("title").Text()
		log.Printf("[IDX] %s - Page title: %s", name, title)

		bodyText := doc.Find("body").Text()
		if len(bodyText) < 100 {
			log.Printf("[Warning] %s - Very little content found, possible JS-rendered page", name)
		}
	}

	if tablesCount > 0 {
		firstTable := doc.Find("table").First()
		rows := firstTable.Find("tr")
		log.Printf("[IDX] %s - First table has %d rows", name, rows.Length())

		if rows.Length() > 0 {
			headerCells := rows.First().Find("th, td")
			log.Printf("[IDX] %s - Header has %d columns", name, headerCells.Length())

			headerCells.Each(func(i int, cell *goquery.Selection) {
				text := strings.TrimSpace(cell.Text())
				log.Printf("  [IDX] %s - Column %d: %s", name, i+1, text)
			})
		}
	}
}
