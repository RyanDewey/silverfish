package main

import (
	"encoding/csv"

	"fmt"
	"log"
	"os"
	"strings"
	"net/url"
	"regexp"

	"golang.org/x/net/publicsuffix"
)

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
		normalizedURL, normalizedSucceeded := NormalizeDomainKey(r.URL)
		if !normalizedSucceeded {
			log.Printf("Error normalizing domain")
			normalizedURL = r.URL
		}

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

// Function takes website URL and normalizes it
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