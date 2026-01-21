package main

import (
	"fmt"
	"slices"
	"sync"
	"time"
	"net/url"
	"strings"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

func appendUnique(slice []string, item string) []string {
	if slices.Contains(slice, item) {
		return slice
	}
	return append(slice, item)
}

var blockedKeywords = []string{
	"ubereats",
	"uber",
	"doordash",
	"postmates",
	"grubhub",
	"toast",
	"toasttab",
	"chownow",
	"caviar",
	"delivery",
}

func isBlockedLink(link string) bool {
	l := strings.ToLower(link)
	for _, kw := range blockedKeywords {
		if strings.Contains(l, "://"+kw) || strings.Contains(l, "."+kw+".") {
			return true
		}
	}
	return false
}

var digitsRe = regexp.MustCompile(`\d+`)

// Normalize phone numbers from regex into real numbers
func normalizePhone(s string) (string, bool) {
	// Keep digits only
	digits := strings.Join(digitsRe.FindAllString(s, -1), "")
	if digits == "" {
		return "", false
	}

	// US-centric normalization:
	// Allow 11 digits starting with 1
	if len(digits) == 11 && digits[0] == '1' {
		digits = digits[1:]
	}
	if len(digits) != 10 {
		return "", false
	}

	// Reject obvious garbage
	if digits == "0000000000" {
		return "", false
	}

	// Format consistently (helps dedupe)
	return digits[:3] + "-" + digits[3:6] + "-" + digits[6:], true
}


// Goroutine crawls a restaurant website concurrently and returns record to results channel
func crawlSite(Url string, results chan<- RestaurantData, wg *sync.WaitGroup, m *Metrics) {

	// Start timer for domain
	m.DomainsStarted.Add(1)
	domainStart := time.Now()

	defer wg.Done()

	// Create collector
	c := colly.NewCollector(
		colly.Async(true),
		colly.MaxDepth(3),
	)

	// Limit concurrency to avoid overloading sites
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 5,
		Delay:       1 * time.Second,
	})

	var record RestaurantData
	record.URL = Url

	var (
		mu sync.Mutex
		pending int // Keep track of the current number of pages being scraped
		emitted bool
	)

	// Sends record to writer if all pages scraped
	finalizeIfDone := func() {
		if pending != 0 || emitted {
			return
		}
		emitted = true

		recCopy := record // copy while locked if record is a struct
		// Set hasOnlineOrdering to true if ordering links
		if len(recCopy.OrderingLinks) > 0 {
			recCopy.hasOnlineOrdering = true
		}
		mu.Unlock()       // unlock before sending (avoid blocking while locked)
		results <- recCopy
		mu.Lock()
		
		m.DomainsFinished.Add(1)

		if len(record.Emails) > 0 { m.DomainsWithEmail.Add(1) }
		if len(record.PhoneNumbers) > 0 { m.DomainsWithPhone.Add(1) }

		m.EmailsFound.Add(int64(len(record.Emails)))
		m.PhonesFound.Add(int64(len(record.PhoneNumbers)))

		// optional: keep domainStart->elapsed for latency stats
		_ = time.Since(domainStart)

	}

	// Callback func for when html element is encountered
	c.OnHTML("body", func(e *colly.HTMLElement) {
		mu.Lock()
		defer mu.Unlock()

		orderingSet := make(map[string]bool)

		// Regex filter for phone numbers
		phoneRe := regexp.MustCompile(`(?i)(?:\+?1[\s\-.]?)?(?:\(\s*\d{3}\s*\)|\d{3})[\s\-.]?\d{3}[\s\-.]?\d{4}(?:\s*(?:x|ext\.?)\s*\d{1,6})?`)

		// First check for links with the tel: attribute for high accuracy numbers
		e.DOM.Find("a[href^='tel:']").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			// href like "tel:+1-949-555-1212"
			raw := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(href), "tel:"))
			if raw != "" {
				if norm, ok := normalizePhone(raw); ok {
					record.PhoneNumbers = appendUnique(record.PhoneNumbers, norm)
				}
			}
		})

		// Then scan the whole page for matches to the phone regex
		e.DOM.Find("script, style, noscript").Remove() // Remove noise
		phonePageText := strings.Join(strings.Fields(e.DOM.Text()), " ") // collapse whitespace

		// Add regex phone matches to the record
		phoneMatches := phoneRe.FindAllString(phonePageText, -1)
		for _, m := range phoneMatches {
			if norm, ok := normalizePhone(m); ok {
				record.PhoneNumbers = appendUnique(record.PhoneNumbers, norm)
			}
		}

		// Check for mailto: links for high accuracy emails
		email, _ := e.DOM.Find("a[href^='mailto:']").Attr("href")
		if email != "" {
			record.Emails = appendUnique(record.Emails, email[7:])
		}

		// find ordering links
		e.DOM.Find("a").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			abs := e.Request.AbsoluteURL(href)
			if strings.Contains(abs, "order") && !orderingSet[abs] {
				orderingSet[abs] = true
				if len(record.OrderingLinks) < 2 {
					record.OrderingLinks = appendUnique(record.OrderingLinks, abs)
				}
			}
		})
	})

	// Keep track of visited URLs
	visited := make(map[string]bool)
	visitedMu := sync.Mutex{}

	// Keywords to follow
	keywords := []string{
		"contact", "about", "location", "order",
		"info", "store", "pickup", "delivery",
	}

	// When encountering links, follow if valid
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		if link == "" {
			return
		}

		// Normalize the link to prevent duplicates
		normalized := strings.TrimSuffix(link, "/")
		u, err := url.Parse(normalized)
		if err == nil {
			u.RawQuery = "" // remove query params for deduping
			normalized = u.String()
		}

		// Only follow if keyword is in URL
		for _, k := range keywords {
			if strings.Contains(strings.ToLower(normalized), k) {
				visitedMu.Lock()
				// Set online ordering flag if link gets blocked
				if isBlockedLink(normalized) {
					record.hasOnlineOrdering = true
				}
				// Check if the link hasnt been visited and isnt blocked to queue next
				if !visited[normalized] && !isBlockedLink(normalized) {
					visited[normalized] = true
					visitedMu.Unlock()
					e.Request.Visit(normalized)
				} else {
					visitedMu.Unlock()
				}
				break
			}
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		urlStr := ""
		status := 0
		if r != nil {
			urlStr = r.Request.URL.String()
			status = r.StatusCode
		}
		fmt.Printf("ERROR url=%s status=%d err=%v\n", urlStr, status, err)

		mu.Lock()
		pending--
		m.RequestsErrored.Add(1)     
		finalizeIfDone()   // emit record even if pages failed
		mu.Unlock()
	})

	// Each new page, increment the counter
	c.OnRequest(func(_ *colly.Request) {
		mu.Lock()
		pending++
		m.RequestsStarted.Add(1)
		mu.Unlock()
	})

	// Decrement counter when the page is scraped, send record if all pages have been scraped
	c.OnScraped(func(_ *colly.Response) {
		mu.Lock()
		pending--
		m.RequestsOK.Add(1)
		finalizeIfDone()
		mu.Unlock()
		fmt.Println("Done crawling a page")
	})

	// Visit the website
	err := c.Visit(Url)
	if err != nil {
		fmt.Println("Visit failed:", Url, err)
		return
	}
	c.Wait()
}