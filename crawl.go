package main

import (
	"fmt"
	"slices"
	"sync"
	"time"
	"net/url"

	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

func appendUnique(slice []string, item string) []string {
	if slices.Contains(slice, item) {
		return slice
	}
	return append(slice, item)
}

// Goroutine crawls a restaurant website concurrently and returns record to results channel
func crawlSite(Url string, results chan<- RestaurantData, wg *sync.WaitGroup) {
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

	var mu sync.Mutex

	// Callback func for when html element is encountered
	c.OnHTML("body", func(e *colly.HTMLElement) {
		mu.Lock()
		defer mu.Unlock()

		orderingSet := make(map[string]bool)

		// Regex filter for phone numbers
		// re := regexp.MustCompile(`\+?\d[\d\s\-\(\)]{7,}\d`)

		// e.DOM.Find("a[href^='tel:'], .phone, .contact, [href*='call'], span:contains('Call'), div:contains('Call')").Each(func(_ int, s *goquery.Selection) {
		// 	text := strings.TrimSpace(s.Text())
		// 	if re.MatchString(text) {
		// 		record.PhoneNumbers = appendUnique(record.PhoneNumbers, text)
		// 	}
		// })

		phone, _ := e.DOM.Find("a[href^='tel:']").Attr("href")

		email, _ := e.DOM.Find("a[href^='mailto:']").Attr("href")

		if phone != "" {
			record.PhoneNumbers = appendUnique(record.PhoneNumbers, phone[4:])
		}

		if email != "" {
			record.Emails = appendUnique(record.Emails, email[7:])
		}

		// find ordering links
		e.DOM.Find("a").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			abs := e.Request.AbsoluteURL(href)
			if strings.Contains(abs, "order") && !orderingSet[abs] {
				orderingSet[abs] = true
				record.OrderingLinks = append(record.OrderingLinks, abs)
			}
		})
	})

	// Keep track of visited URLs
	visited := make(map[string]bool)
	visitedMu := sync.Mutex{}

	// Keywords to follow
	keywords := []string{
		"contact", "about", "location", "order", "menu",
		"info", "reservations", "reservation", "shop",
		"store", "pickup", "delivery",
	}

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
				if !visited[normalized] {
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
		fmt.Println("Failed:", r.Request.URL, err)
	})

	// When domain crawl completes
	c.OnScraped(func(_ *colly.Response) {
		mu.Lock()
		defer mu.Unlock()

		fmt.Println("Done crawling a domain")
		results <- record // Send to writer
	})

	// Visit the website
	err := c.Visit(Url)
	if err != nil {
		fmt.Println("Visit failed:", Url, err)
		return
	}
	c.Wait()
}