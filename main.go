package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"sync"

	// "net/url"

	// "regexp"
	"log"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/joho/godotenv"
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
	center := LatLng{Latitude: 34.0549, Longitude: -118.2426}
	radius := 500.0
	maxCount := 10

	// Get places
	places, err := GetNearbyPlaces(apiKey, center, radius, maxCount)
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

		if (p.WebsiteURI != "") {
			urls = append(urls, p.WebsiteURI)
		}
	}

	fmt.Printf("Total places: %d\n", len(places))
	fmt.Println()

	
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
    for _, s := range slice {
        if s == item {
            return slice
        }
    }
    return append(slice, item)
}


// Goroutine crawls a restaurant website concurrently and returns record to results channel
func crawlSite(url string, results chan<- RestaurantData, wg *sync.WaitGroup) {
    defer wg.Done()

	// Create collector
    c := colly.NewCollector(
        colly.Async(true),
		colly.MaxDepth(10),
    )

	// Limit concurrency to avoid overloading sites
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 5,
		Delay:       1 * time.Second,
	})

    var record RestaurantData
    record.URL = url

	var mu sync.Mutex

    // Callback func for when html element is encountered
	c.OnHTML("body", func(e *colly.HTMLElement) {
		mu.Lock()
		defer mu.Unlock()

		orderingSet := make(map[string]bool)

		phone := e.DOM.Find("a[href^='tel:']").Text()
		email := e.DOM.Find("a[href^='mailto:']").Text()

		if phone != "" {
			record.PhoneNumbers = appendUnique(record.PhoneNumbers, phone)
		}
		if email != "" {
			record.Emails = appendUnique(record.Emails, email)
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

	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Failed:", r.Request.URL, err)
	})

    // When domain crawl completes
    c.OnScraped(func(_ *colly.Response) {
		mu.Lock()
    	defer mu.Unlock()

		fmt.Println("Done crawling a domain");
        results <- record // Send to writer
    })

	// Visit the website
	err := c.Visit(url)
    if err != nil {
        fmt.Println("Visit failed:", url, err)
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

	// Write the records
	for r := range records {
		if err := w.Write([]string{
			r.URL,
			strings.Join(r.PhoneNumbers, ";"),
			strings.Join(r.Emails, ";"),
			strings.Join(r.OrderingLinks, ";"),
		}); err != nil {
			log.Printf("Error writing record: %v", err)
		}
	}

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
