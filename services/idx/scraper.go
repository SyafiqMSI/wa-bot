package idx

import (
	"compress/gzip"
	"context"
	"encoding/json"
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
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// idxNuxtItem is a single announcement row from IDX __NUXT__ state
type idxNuxtItem struct {
	Text string `json:"text"`
	Date string `json:"date"`
}

// GetIDXMarketData is the main entry point to fetch all market data for a target date
func GetIDXMarketData(targetDate time.Time) (*domain.IDXData, error) {
	if targetDate.IsZero() {
		targetDate = time.Now()
	}
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	targetDate = targetDate.In(loc)
	todayStr := targetDate.Format("02-Jan-2006")

	data := &domain.IDXData{
		Date:       todayStr,
		RUPS:       []string{},
		UMA:        []string{},
		Suspensi:   []string{},
		Unsuspensi: []string{},
		Dividend:   []domain.DividendData{},
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Fetch everything in sequence
	if uma, err := scrapeUMAData(targetDate); err == nil {
		data.UMA = uma
	}
	if susp, unsusp, err := scrapeSuspensiData(targetDate); err == nil {
		data.Suspensi = susp
		data.Unsuspensi = unsusp
	}
	if rups, err := scrapeRUPSData(client, targetDate); err == nil {
		data.RUPS = rups
	}
	if dividend, err := scrapeDividendData(client, targetDate); err == nil {
		data.Dividend = dividend
	}

	return data, nil
}

// --- Scraper Implementations ---

func scrapeUMAData(targetDate time.Time) ([]string, error) {
	items, err := scrapeIDXWithChromedp("https://www.idx.co.id/id/berita/unusual-market-activity-uma", "", "")
	if err != nil {
		return nil, err
	}

	parenRe := regexp.MustCompile(`\(([A-Z]{2,6})\)`)
	var results []string
	for _, item := range items {
		if isTargetDateImproved(item.Date, targetDate) && item.Text != "" {
			if m := parenRe.FindStringSubmatch(item.Text); len(m) > 1 {
				results = append(results, m[1])
			}
		}
	}
	return results, nil
}

func scrapeSuspensiData(targetDate time.Time) ([]string, []string, error) {
	items, err := scrapeIDXWithChromedp("https://www.idx.co.id/id/berita/suspensi", "", "")
	if err != nil {
		return nil, nil, err
	}

	parenRe := regexp.MustCompile(`\(([A-Z]{2,6})\)`)
	var suspensi, unsuspensi []string

	for _, item := range items {
		if !isTargetDateImproved(item.Date, targetDate) || item.Text == "" {
			continue
		}

		low := strings.ToLower(item.Text)
		isS := strings.Contains(low, "penghentian sementara") || strings.Contains(low, "suspensi")
		isU := strings.Contains(low, "pembukaan kembali") || strings.Contains(low, "pencabutan") || strings.Contains(low, "dibuka")

		if !isS && !isU {
			continue
		}

		if m := parenRe.FindStringSubmatch(item.Text); len(m) > 1 {
			code := m[1]
			if isU {
				unsuspensi = append(unsuspensi, code)
			} else {
				suspensi = append(suspensi, code)
			}
		}
	}
	return suspensi, unsuspensi, nil
}

func scrapeRUPSData(client *http.Client, targetDate time.Time) ([]string, error) {
	var results []string
	seen := make(map[string]bool)

	// Fetch up to 10 pages to ensure we catch the target date (pagination uses /page/X)
	for p := 1; p <= 10; p++ {
		url := "https://www.new.sahamidx.com/?/rups"
		if p > 1 {
			url = fmt.Sprintf("https://www.new.sahamidx.com/?/rups/page/%d", p)
		}

		doc, err := fetchGoQuery(client, url)
		if err != nil {
			log.Printf("[RUPS] Error fetching page %d: %v", p, err)
			continue
		}

		foundOnPage := false
		doc.Find("table tbody tr").Each(func(i int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() >= 6 {
				code := strings.TrimSpace(cells.Eq(1).Text())
				date := strings.TrimSpace(cells.Eq(2).Text())
				if code != "" && isTargetDateImproved(date, targetDate) {
					uCode := strings.ToUpper(code)
					if !seen[uCode] {
						results = append(results, uCode)
						seen[uCode] = true
						foundOnPage = true
					}
				}
			}
		})

		// If we haven't found anything yet across several pages, we keep looking
		// But if we FOUND something and now we don't, we might have passed the date block
		if p > 5 && !foundOnPage && len(results) > 0 {
			break
		}
	}
	return results, nil
}

func scrapeDividendData(client *http.Client, targetDate time.Time) ([]domain.DividendData, error) {
	var results []domain.DividendData
	seen := make(map[string]bool)

	for p := 1; p <= 10; p++ {
		url := "https://www.new.sahamidx.com/?/deviden"
		if p > 1 {
			url = fmt.Sprintf("https://www.new.sahamidx.com/?/deviden/page/%d", p)
		}

		doc, err := fetchGoQuery(client, url)
		if err != nil {
			log.Printf("[Dividend] Error fetching page %d: %v", p, err)
			continue
		}

		doc.Find("table tbody tr").Each(func(i int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() >= 6 {
				code := strings.TrimSpace(cells.Eq(0).Text())
				amt := strings.TrimSpace(cells.Eq(1).Text())
				cum := strings.TrimSpace(cells.Eq(2).Text())
				ex := strings.TrimSpace(cells.Eq(3).Text())

				if code != "" && code != "Deviden Saham" {
					if isTargetDateImproved(cum, targetDate) || isTargetDateImproved(ex, targetDate) {
						uCode := strings.ToUpper(code)
						if !seen[uCode] {
							results = append(results, domain.DividendData{
								Code: uCode, Amount: amt, CumDate: cum, ExDate: ex,
								Yield: "N/A", Price: "N/A",
							})
							seen[uCode] = true
						}
					}
				}
			}
		})
	}
	return results, nil
}

// --- Headless Browser Logic ---

func scrapeIDXWithChromedp(pageURL, _, _ string) ([]idxNuxtItem, error) {
	js := `
(function() {
	var best = null; var max = 0;
	function s(o, d) {
		if (d > 12 || !o || typeof o !== 'object') return;
		if (Array.isArray(o)) {
			if (o.length > 2 && o[0] && typeof o[0] === 'object') {
				var keys = Object.keys(o[0]).join(',');
				if (keys.includes('Date') || keys.includes('Judul') || keys.includes('Pengumuman')) {
					if (o.length > max) { best = o; max = o.length; }
				}
			}
			o.forEach(x => s(x, d + 1));
		} else {
			for (var k in o) { try { s(o[k], d + 1); } catch(e) {} }
		}
	}
	var srcs = [window.__NUXT__, window.__NUXT_DATA__, window.__nuxt__];
	srcs.forEach(x => { if(x) s(x, 0); });
	if (!best) return "[]";
	return JSON.stringify(best.map(x => ({
		text: x.Judul || x.Pengumuman || x.JudulEn || x.PengumumanEn || x.Text || x.Title || "",
		date: x.Date || x.PublishDate || x.CreatedDate || x.SuspensiDate || x.UMADate || x.date || ""
	})));
})()`

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, tcancel := context.WithTimeout(ctx, 50*time.Second)
	defer tcancel()

	// Hide webdriver attribute
	chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument("Object.defineProperty(navigator, 'webdriver', {get: () => undefined})").Do(ctx)
		return err
	}))

	var res string
	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.Sleep(5*time.Second),
		chromedp.Evaluate(js, &res),
	)
	if err != nil {
		return nil, err
	}

	var items []idxNuxtItem
	if err := json.Unmarshal([]byte(res), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func fetchGoQuery(client *http.Client, url string) (*goquery.Document, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var r io.Reader = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, _ := gzip.NewReader(resp.Body)
		defer gz.Close()
		r = gz
	}
	return goquery.NewDocumentFromReader(r)
}

// --- Utilities ---

func isTargetDateImproved(dateStr string, targetDate time.Time) bool {
	if dateStr == "" {
		return false
	}
	loc := time.FixedZone("WIB", 7*3600)
	targetStr := targetDate.In(loc).Format("2006-01-02")

	val := strings.ToLower(strings.TrimSpace(dateStr))
	monthMap := map[string]string{
		"januari": "January", "jan": "Jan", "februari": "February", "feb": "Feb", "pebruari": "February", "peb": "Feb",
		"maret": "March", "mar": "Mar", "april": "April", "apr": "Apr",
		"mei": "May", "may": "May", "juni": "June", "jun": "Jun",
		"juli": "July", "jul": "Jul", "agustus": "August", "agu": "Aug", "agt": "Aug",
		"september": "September", "sep": "Sep", "oktober": "October", "okt": "Oct", "oct": "Oct",
		"november": "November", "nov": "Nov", "desember": "December", "des": "Dec", "dec": "Dec",
	}
	for k, v := range monthMap {
		val = strings.ReplaceAll(val, k, v)
	}

	formats := []string{
		"2006-01-02T15:04:05", "2006-01-02", "02-Jan-2006", "2-Jan-2006", "2-Jan-06",
		"02/01/2006", "02-01-2006", "2 January 2006", "2 Jan 2006", "02 January 2006", "02 Jan 2006",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, strings.TrimSpace(dateStr)); err == nil {
			if t.Format("2006-01-02") == targetStr {
				return true
			}
		}
		if t, err := time.Parse(f, val); err == nil {
			if t.Format("2006-01-02") == targetStr {
				return true
			}
		}
	}

	// Day Month Year (ID full)
	parts := strings.Fields(val)
	if len(parts) >= 3 {
		day, _ := strconv.Atoi(parts[0])
		year, _ := strconv.Atoi(parts[len(parts)-1])
		if year > 0 && day > 0 {
			for k, v := range monthMap {
				if strings.Contains(val, k) {
					if t, err := time.Parse("January 2, 2006", fmt.Sprintf("%s %d, %d", v, day, year)); err == nil {
						if t.Format("2006-01-02") == targetStr {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

func FormatIDXResponse(data *domain.IDXData) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[IDX Market Data for %s]\n\n", data.Date))

	writeSec := func(title string, items []string) {
		sb.WriteString("[" + title + "]\n")
		if len(items) == 0 {
			sb.WriteString("-\n")
		}
		for _, v := range items {
			sb.WriteString(v + "\n")
		}
		sb.WriteString("\n")
	}

	writeSec("RUPS", data.RUPS)
	writeSec("UMA", data.UMA)
	writeSec("Unsuspensi", data.Unsuspensi)
	writeSec("Suspensi", data.Suspensi)

	sb.WriteString("[DIVIDEND]\n")
	if len(data.Dividend) == 0 {
		sb.WriteString("-\n")
	} else {
		for _, d := range data.Dividend {
			sb.WriteString(fmt.Sprintf("%s (Div. Rp %s)\n", d.Code, d.Amount))
			if d.CumDate != "" && d.CumDate != "N/A" {
				sb.WriteString("Cum: " + d.CumDate + "\n")
			}
			if d.ExDate != "" && d.ExDate != "N/A" {
				sb.WriteString("Ex: " + d.ExDate + "\n")
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
