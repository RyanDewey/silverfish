package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"os"
	"github.com/joho/godotenv"
	"log"
)

// GetNearbyPlaces queries Google Places Nearby Search API and returns all results (up to 20 per location)
func GetNearbyPlaces() ([]Place, error) {
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

	// Set url to query
	placesNearbyURL := "https://places.googleapis.com/v1/places:searchNearby"

	// Set up parameters for api call
	center := LatLng{Latitude: 34.0549, Longitude: -118.2426}
	radius := 500.0
	maxCount := 10

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
