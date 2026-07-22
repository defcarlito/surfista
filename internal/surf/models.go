package surf

import "time"

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

// Forecast is an hourly surf outlook for a spot. UTCOffset is kept separately
// because the terminal may be running in a different timezone from the surf
// spot. The dashboard can select and sample a local day from the full window.
type Forecast struct {
	SpotID    string
	UTCOffset time.Duration
	Slots     []ForecastSlot
}

// ForecastSlot contains the surf data used by the dashboard for each hour in
// the available forecast window.
type ForecastSlot struct {
	Timestamp  time.Time
	Rating     string
	SurfHeight SurfHeight
	Swells     []Swell
}

type SurfHeight struct {
	Min           float64
	Max           float64
	Plus          bool
	HumanRelation string
}

// ForecastDetails contains the richer wind, weather, tide, and daylight data
// fetched for a location.
type ForecastDetails struct {
	SpotID    string
	UTCOffset time.Duration
	Units     ForecastUnits
	Slots     []ForecastDetailSlot
	Tides     []TidePoint
	Sunlight  []SunlightDay
}

// ForecastCacheEntry is the last successful base and detailed forecast saved
// for a favorite location. Each timestamp belongs to its corresponding data
// because the two requests can succeed independently.
type ForecastCacheEntry struct {
	SpotID            string
	Forecast          Forecast
	ForecastUpdatedAt time.Time
	Details           ForecastDetails
	DetailsUpdatedAt  time.Time
}

type ForecastUnits struct {
	WindSpeed   string
	TideHeight  string
	Temperature string
}

type ForecastDetailSlot struct {
	Timestamp   time.Time
	Wind        Wind
	Temperature *float64
}

type Swell struct {
	Height    float64
	Period    float64
	Direction float64
}

type Wind struct {
	Speed         float64
	Gust          float64
	Direction     float64
	DirectionType string
}

type TidePoint struct {
	Timestamp time.Time
	Type      string
	Height    float64
}

// SunlightDay contains Surfline's daily daylight boundaries. The timestamps
// remain in UTC so callers can consistently convert them using a spot's
// timezone information.
type SunlightDay struct {
	Dawn    time.Time
	Sunrise time.Time
	Sunset  time.Time
	Dusk    time.Time
}
