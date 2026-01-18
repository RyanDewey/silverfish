package main

type RestaurantData struct {
	URL           string
	PhoneNumbers  []string
	Emails        []string
	OrderingLinks []string
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