package main

import (
	"fmt"
	"sync"

	// "regexp"
	"log"
	"os"

	"github.com/joho/godotenv"
)


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