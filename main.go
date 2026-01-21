package main

import (
	"fmt"
	"sync"

	// "regexp"
	"log"
	"os"
)

// Main function
func main() {
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

	// Start worker that 
	go writeWorker(file, results, done)

	// Get places
	// places, err := GetNearbyPlaces()
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
			}{Text: "Hangry Moon's"},
			WebsiteURI: "https://www.hangrymoons.com/",
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