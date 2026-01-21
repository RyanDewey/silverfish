package main

import ( 
	"sync/atomic"
	"time"
)

type RestaurantData struct {
	URL           string
	PhoneNumbers  []string
	Emails        []string
	OrderingLinks []string
	hasOnlineOrdering bool
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
type Metrics struct {
	Start time.Time

	DomainsStarted   atomic.Int64
	DomainsFinished  atomic.Int64
	DomainsWithEmail atomic.Int64
	DomainsWithPhone atomic.Int64

	RequestsStarted  atomic.Int64
	RequestsOK       atomic.Int64
	RequestsErrored  atomic.Int64

	EmailsFound atomic.Int64
	PhonesFound atomic.Int64
}
