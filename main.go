package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	// "net/url"

	// "regexp"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

type RestaurantData struct {
	URL           string
	PhoneNumbers  []string
	Emails        []string
	OrderingLinks []string
}

// Main function
func main() {
	// 1. Get restaurant website URLs from Google Places (nearby) API
	// Set up parameters for func call
	const GOOGLE_MAPS_API_KEY string = ""
	center := LatLng{Latitude: 34.0549, Longitude: -118.2426}
	radius := 500.0
	maxCount := 3

	// Get places
	places, err := GetNearbyPlaces(GOOGLE_MAPS_API_KEY, center, radius, maxCount)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// 2. Store all URLs in a queue
	// Create new slice for urls
	urls := []string{}

	// Loop through places and append urls
	for _, p := range places {
		fmt.Printf("Name: %s - Website URL: %s\n", p.DisplayName.Text, p.WebsiteURI)
		urls = append(urls, p.WebsiteURI)
	}

	fmt.Printf("Total places: %d\n", len(places))

	// 3. Set up goroutine for writing
	// Create channel for restaurant data
	results := make(chan RestaurantData)

	go func() {
		file, _ := os.Create("restaurants.csv")
		defer file.Close()
		w := csv.NewWriter(file)
		defer w.Flush()
		w.Write([]string{"URL", "Phones", "Emails", "OrderingLinks"})

		for r := range results {
			w.Write([]string{
				r.URL,
				strings.Join(r.PhoneNumbers, ";"),
				strings.Join(r.Emails, ";"),
				strings.Join(r.OrderingLinks, ";"),
			})
		}
	}()

	// 4. Loop through URLs and call the Crawl function on them concurrently
	// Create collector
	c := colly.NewCollector(
		colly.Async(true),
		colly.MaxDepth(1),
	)

	// Limit concurrency to avoid overloading sites
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 5,
		Delay:       1 * time.Second,
	})

	// Callback func for when html element is encountered
	c.OnHTML("body", func(e *colly.HTMLElement) {
		phone := e.DOM.Find("a[href^='tel:']").Text()
		email := e.DOM.Find("a[href^='mailto:']").Text()
		fmt.Println("URL:", e.Request.URL, "Phone:", phone, "Email:", email)

		// find ordering links
		e.DOM.Find("a").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			if strings.Contains(href, "order") {
				fmt.Println("Ordering link:", e.Request.AbsoluteURL(href))
			}
		})
	})

	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Failed:", r.Request.URL, err)
	})

	// Track visited
	visited := make(map[string]bool)

	// Visit all urls in the slice
	for _, url := range urls {
		if !visited[url] {
			visited[url] = true
			c.Visit(url)
		}
	}

	// Wait until all crawling is done
	c.Wait()
	close(results)

	fmt.Printf("\n\nSilverfish done crawling!")

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
