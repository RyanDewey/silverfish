package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"
	"net/url"

	"regexp"
	"log"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/joho/godotenv"
	"golang.org/x/net/publicsuffix"
)

type RestaurantData struct {
	URL           string
	PhoneNumbers  []string
	Emails        []string
	OrderingLinks []string
}

// Main function
func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Access the key
	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")

	if apiKey == "" {
		log.Fatal("GOOGLE_MAPS_API_KEY not set in .env file")
	}

	// Set path for exporting file
	filePath := "restaurants.csv"

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}

	// Close the file after all goroutines finish
	defer func() {
		log.Println("Closing the file.")
		if closeErr := file.Close(); closeErr != nil {
			log.Printf("Error closing file: %v", closeErr)
		}
	}()

	// Create channel for restaurant data
	results := make(chan RestaurantData)
	done := make(chan struct{}) // Signal channel to say writing is done
	var crawlWg sync.WaitGroup

	go writeWorker(file, results, done)

	// 1. Get restaurant website URLs from Google Places (nearby) API
	// Set up parameters for func call
	// center := LatLng{Latitude: 34.0549, Longitude: -118.2426}
	// radius := 500.0
	// maxCount := 10

	// Get places
	// places, err := GetNearbyPlaces(apiKey, center, radius, maxCount)
	// if err != nil {
	// 	fmt.Println("Error getting places:", err)
	// 	return
	// }

	places := []Place{
		{
			DisplayName: struct {
				Text string `json:"text"`
			}{Text: "Fat Sal's"},
			WebsiteURI: "https://www.fatsalsdeli.com/",
		},
		{
			DisplayName: struct {
				Text string `json:"text"`
			}{Text: "Barney's Beanery"},
			WebsiteURI: "https://barneysbeanery.com/",
		},
		{
			DisplayName: struct {
				Text string `json:"text"`
			}{Text: "Wolf's Glen"},
			WebsiteURI: "https://wolfsglen.com/",
		},
		{
			DisplayName: struct {
				Text string `json:"text"`
			}{Text: "Gogobop"},
			WebsiteURI: "https://www.gogobop.com/",
		},
	}

	// 2. Store all URLs in a queue
	// Create new slice for urls
	urls := []string{}

	// Loop through places and append urls
	for _, p := range places {
		fmt.Printf("Name: %s - Website URL: %s\n", p.DisplayName.Text, p.WebsiteURI)

		if p.WebsiteURI != "" {
			urls = append(urls, p.WebsiteURI)
		}
	}

	fmt.Printf("Total places: %d\n", len(places))
	fmt.Println()

	// Start crawling sites
	for _, url := range urls {
		crawlWg.Add(1)
		fmt.Println("Calling crawl site!")
		go crawlSite(url, results, &crawlWg)
	}

	crawlWg.Wait()
	close(results)
	<-done

	fmt.Printf("\n\nSilverfish done crawling!\n")
}

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

func writeWorker(file *os.File, records <-chan RestaurantData, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	fmt.Println("Write worker ready for duty!")

	w := csv.NewWriter(file)
	defer w.Flush()

	// Write the header
	header := []string{"URL", "Phones", "Emails", "OrderingLinks"}
	if err := w.Write(header); err != nil {
		log.Printf("Error writing header: %v", err)
		return
	}

	// Dedupe with a seen map
	visitedDomains := make(map[string]bool)

	// Write the records
	for r := range records {

		// Normalize the URL
		normalizedURL := r.URL

		// Write record if not in visited
		if !visitedDomains[normalizedURL] {
			// Add the domain to the visited map
			visitedDomains[normalizedURL] = true

			// Write the record, handle error
			if err := w.Write([]string{
				normalizedURL,
				strings.Join(r.PhoneNumbers, ";"),
				strings.Join(r.Emails, ";"),
				strings.Join(r.OrderingLinks, ";"),
			}); err != nil {
				log.Printf("Error writing record: %v", err)
			}
		} 

	}

}


func NormalizeDomainKey(website string) (string, bool) {
	s := strings.TrimSpace(website)
	if s == "" {
		return "", false
	}

	// Ignore non-web schemes
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "data:") {
		return "", false
	}

	// Handle protocol-relative URLs: //example.com/path
	if strings.HasPrefix(s, "//") {
		s = "https:" + s
	}

	// If missing scheme, prepend https:// so url.Parse can find Host.
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}

	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return "", false
	}

	// Strip www / www2 / www10 ...
	wwwPrefixRe := regexp.MustCompile(`^www\d*\.`)
	host = wwwPrefixRe.ReplaceAllString(host, "")

	// Convert to registrable domain (effective TLD + 1)
	etld1, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		// Fallback: return the host itself if PSL fails
		return host, true
	}

	return etld1, true
}

type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type CircleRestriction struct {
	Center LatLng  `json:"center"`
	Radius float64 `json:"radius"`
}

type LocationRestriction struct {
	Circle *CircleRestriction `json:"circle,omitempty"`
}

type NearbyRequest struct {
	IncludedTypes       []string            `json:"includedTypes,omitempty"`
	MaxResultCount      int                 `json:"maxResultCount,omitempty"`
	LocationRestriction LocationRestriction `json:"locationRestriction"`
}

type Place struct {
	DisplayName struct {
		Text string `json:"text"`
	} `json:"displayName"`
	WebsiteURI string `json:"websiteUri"`
}

type NearbyResponse struct {
	Places []Place `json:"places"`
}

// GetNearbyPlaces queries Google Places Nearby Search API and returns all results (up to 20 per location)
func GetNearbyPlaces(apiKey string, center LatLng, radius float64, maxCount int) ([]Place, error) {
	placesNearbyURL := "https://places.googleapis.com/v1/places:searchNearby"

	// Build request body
	reqBody := NearbyRequest{
		IncludedTypes:  []string{"restaurant"},
		MaxResultCount: maxCount,
		LocationRestriction: LocationRestriction{
			Circle: &CircleRestriction{
				Center: center,
				Radius: radius,
			},
		},
	}

	// Convert the Go struct into JSON format for the API req
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create the http POST request to the API
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", placesNearbyURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	// Use API key via header (or OAuth token, whichever your setup uses)
	req.Header.Set("X-Goog-Api-Key", apiKey)
	// Also you must set a field mask header to tell the API which fields you want back
	req.Header.Set("X-Goog-FieldMask", "places.displayName,places.websiteUri")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response: %d, body: %s", resp.StatusCode, string(respBytes))
	}

	var nr NearbyResponse
	if err := json.Unmarshal(respBytes, &nr); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	return nr.Places, nil
}
