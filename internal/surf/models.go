package surf

// Spot is the provider-neutral location data Surfista needs now and for
// fetching a spot forecast later. ID is Surfline's stable spot identifier.
type Spot struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	URL       string  `json:"url,omitempty"`
	Region    string  `json:"region,omitempty"`
	Country   string  `json:"country,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
