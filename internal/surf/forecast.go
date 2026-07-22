package surf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	forecastResponseLimit = 4 << 20
	// surflineForecastDays covers today and the following nine days shown in
	// Surfline's spot forecast.
	surflineForecastDays = 10
)

const surfistaUserAgent = "surfista/0.1"

var ErrEmptySpotID = errors.New("spot ID is empty")

// SurflineForecastProvider reads Surfline's anonymous hourly forecast feeds.
// These are undocumented website services, so their response details remain
// private to this package and the base URL is injectable for deterministic
// tests.
type SurflineForecastProvider struct {
	baseURL *url.URL
	client  *http.Client
}

func NewSurflineForecastProvider(baseURL string, client *http.Client) (*SurflineForecastProvider, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultSurflineBaseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Surfline base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("Surfline base URL must include a scheme and host")
	}
	if client == nil {
		client = http.DefaultClient
	}

	return &SurflineForecastProvider{baseURL: parsed, client: client}, nil
}

func (p *SurflineForecastProvider) Forecast(ctx context.Context, spotID string) (Forecast, error) {
	spotID = strings.TrimSpace(spotID)
	if spotID == "" {
		return Forecast{}, ErrEmptySpotID
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type waveResult struct {
		response waveForecastResponse
		err      error
	}
	type ratingResult struct {
		response ratingForecastResponse
		err      error
	}

	waves := make(chan waveResult, 1)
	ratings := make(chan ratingResult, 1)
	go func() {
		var response waveForecastResponse
		err := p.getForecast(ctx, "wave", spotID, &response)
		waves <- waveResult{response: response, err: err}
	}()
	go func() {
		var response ratingForecastResponse
		err := p.getForecast(ctx, "rating", spotID, &response)
		ratings <- ratingResult{response: response, err: err}
	}()

	wave := <-waves
	if wave.err != nil {
		cancel()
	}
	rating := <-ratings
	if wave.err != nil {
		return Forecast{}, wave.err
	}
	if rating.err != nil {
		return Forecast{}, rating.err
	}

	ratingsByTimestamp := make(map[int64]string, len(rating.response.Data.Rating))
	for _, point := range rating.response.Data.Rating {
		if label := point.Rating.label(); label != "" {
			ratingsByTimestamp[point.Timestamp] = label
		}
	}

	slots := make([]ForecastSlot, 0, len(wave.response.Data.Wave))
	for _, point := range wave.response.Data.Wave {
		label, ok := ratingsByTimestamp[point.Timestamp]
		if !ok {
			continue
		}

		minHeight, maxHeight := point.Surf.Min, point.Surf.Max
		if point.Surf.Raw != nil {
			if minHeight == 0 {
				minHeight = point.Surf.Raw.Min
			}
			if maxHeight == 0 {
				maxHeight = point.Surf.Raw.Max
			}
		}
		slots = append(slots, ForecastSlot{
			Timestamp: time.Unix(point.Timestamp, 0).UTC(),
			Rating:    label,
			SurfHeight: SurfHeight{
				Min:           minHeight,
				Max:           maxHeight,
				Plus:          point.Surf.Plus,
				HumanRelation: point.Surf.HumanRelation,
			},
			Swells: mapSurflineSwells(point.Swells),
		})
	}

	if len(slots) == 0 {
		return Forecast{}, errors.New("Surfline returned no matching wave and rating forecast slots")
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Timestamp.Before(slots[j].Timestamp)
	})

	offsetHours := wave.response.Associated.UTCOffset
	if offsetHours == 0 {
		offsetHours = rating.response.Associated.UTCOffset
	}

	return Forecast{
		SpotID:    spotID,
		UTCOffset: time.Duration(offsetHours * float64(time.Hour)),
		Slots:     slots,
	}, nil
}

func (p *SurflineForecastProvider) ForecastDetails(ctx context.Context, spotID string) (ForecastDetails, error) {
	spotID = strings.TrimSpace(spotID)
	if spotID == "" {
		return ForecastDetails{}, ErrEmptySpotID
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type windResult struct {
		response windForecastResponse
		err      error
	}
	type tideResult struct {
		response tideForecastResponse
		err      error
	}
	type weatherResult struct {
		response weatherForecastResponse
		err      error
	}

	winds := make(chan windResult, 1)
	tides := make(chan tideResult, 1)
	weather := make(chan weatherResult, 1)
	go func() {
		var response windForecastResponse
		err := p.getForecast(ctx, "wind", spotID, &response)
		winds <- windResult{response: response, err: err}
	}()
	go func() {
		var response tideForecastResponse
		err := p.getForecast(ctx, "tides", spotID, &response)
		tides <- tideResult{response: response, err: err}
	}()
	go func() {
		var response weatherForecastResponse
		err := p.getForecast(ctx, "weather", spotID, &response)
		weather <- weatherResult{response: response, err: err}
	}()

	wind := <-winds
	tide := <-tides
	conditions := <-weather
	for _, result := range []struct {
		kind string
		err  error
	}{
		{kind: "wind", err: wind.err},
		{kind: "tides", err: tide.err},
		{kind: "weather", err: conditions.err},
	} {
		if result.err != nil {
			cancel()
			return ForecastDetails{}, result.err
		}
	}

	slotsByTimestamp := make(map[int64]ForecastDetailSlot, len(wind.response.Data.Wind)+len(conditions.response.Data.Weather))
	for _, point := range wind.response.Data.Wind {
		slotsByTimestamp[point.Timestamp] = ForecastDetailSlot{
			Timestamp: time.Unix(point.Timestamp, 0).UTC(),
			Wind: Wind{
				Speed:         point.Speed,
				Gust:          point.Gust,
				Direction:     point.Direction,
				DirectionType: point.DirectionType,
			},
		}
	}
	for _, point := range conditions.response.Data.Weather {
		slot := slotsByTimestamp[point.Timestamp]
		slot.Timestamp = time.Unix(point.Timestamp, 0).UTC()
		slot.Temperature = point.Temperature
		slotsByTimestamp[point.Timestamp] = slot
	}

	slots := make([]ForecastDetailSlot, 0, len(slotsByTimestamp))
	for _, slot := range slotsByTimestamp {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Timestamp.Before(slots[j].Timestamp)
	})

	tidePoints := make([]TidePoint, 0, len(tide.response.Data.Tides))
	for _, point := range tide.response.Data.Tides {
		tidePoints = append(tidePoints, TidePoint{
			Timestamp: time.Unix(point.Timestamp, 0).UTC(),
			Type:      strings.ToUpper(point.Type),
			Height:    point.Height,
		})
	}
	sort.Slice(tidePoints, func(i, j int) bool {
		return tidePoints[i].Timestamp.Before(tidePoints[j].Timestamp)
	})

	associated := wind.response.Associated
	for _, candidate := range []forecastAssociated{
		tide.response.Associated,
		conditions.response.Associated,
	} {
		if associated.UTCOffset == 0 && candidate.UTCOffset != 0 {
			associated.UTCOffset = candidate.UTCOffset
		}
		associated.Units.merge(candidate.Units)
	}

	return ForecastDetails{
		SpotID:    spotID,
		UTCOffset: time.Duration(associated.UTCOffset * float64(time.Hour)),
		Units: ForecastUnits{
			WindSpeed:   associated.Units.WindSpeed,
			TideHeight:  associated.Units.TideHeight,
			Temperature: associated.Units.Temperature,
		},
		Slots: slots,
		Tides: tidePoints,
	}, nil
}

func mapSurflineSwells(points []struct {
	Height    float64 `json:"height"`
	Period    float64 `json:"period"`
	Direction float64 `json:"direction"`
}) []Swell {
	swells := make([]Swell, 0, len(points))
	for _, point := range points {
		if point.Height <= 0 {
			continue
		}
		swells = append(swells, Swell{
			Height:    point.Height,
			Period:    point.Period,
			Direction: point.Direction,
		})
	}
	return swells
}

func (p *SurflineForecastProvider) getForecast(ctx context.Context, kind, spotID string, destination any) error {
	endpoint := *p.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/kbyg/spots/forecasts/" + kind
	params := endpoint.Query()
	params.Set("spotId", spotID)
	params.Set("days", fmt.Sprint(surflineForecastDays))
	params.Set("intervalHours", "1")
	if kind == "wave" {
		params.Set("maxHeights", "false")
	}
	if kind == "rating" {
		params.Set("cacheEnabled", "true")
	}
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("create Surfline %s request: %w", kind, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", surfistaUserAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch Surfline %s forecast: %w", kind, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("Surfline %s forecast returned %d: %s", kind, resp.StatusCode, detail)
	}

	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("Surfline %s forecast returned unexpected Content-Type %q", kind, resp.Header.Get("Content-Type"))
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, forecastResponseLimit))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode Surfline %s forecast response: %w", kind, err)
	}
	return nil
}

type forecastAssociated struct {
	UTCOffset float64       `json:"utcOffset"`
	Units     forecastUnits `json:"units"`
}

type forecastUnits struct {
	WaveHeight  string `json:"waveHeight"`
	WindSpeed   string `json:"windSpeed"`
	TideHeight  string `json:"tideHeight"`
	Temperature string `json:"temperature"`
}

func (u *forecastUnits) merge(other forecastUnits) {
	if u.WaveHeight == "" {
		u.WaveHeight = other.WaveHeight
	}
	if u.WindSpeed == "" {
		u.WindSpeed = other.WindSpeed
	}
	if u.TideHeight == "" {
		u.TideHeight = other.TideHeight
	}
	if u.Temperature == "" {
		u.Temperature = other.Temperature
	}
}

type waveForecastResponse struct {
	Associated forecastAssociated `json:"associated"`
	Data       struct {
		Wave []struct {
			Timestamp int64 `json:"timestamp"`
			Surf      struct {
				Min           float64 `json:"min"`
				Max           float64 `json:"max"`
				Plus          bool    `json:"plus"`
				HumanRelation string  `json:"humanRelation"`
				Raw           *struct {
					Min float64 `json:"min"`
					Max float64 `json:"max"`
				} `json:"raw"`
			} `json:"surf"`
			Swells []struct {
				Height    float64 `json:"height"`
				Period    float64 `json:"period"`
				Direction float64 `json:"direction"`
			} `json:"swells"`
		} `json:"wave"`
	} `json:"data"`
}

type windForecastResponse struct {
	Associated forecastAssociated `json:"associated"`
	Data       struct {
		Wind []windForecastPoint `json:"wind"`
	} `json:"data"`
}

type windForecastPoint struct {
	Timestamp     int64   `json:"timestamp"`
	Speed         float64 `json:"speed"`
	Gust          float64 `json:"gust"`
	Direction     float64 `json:"direction"`
	DirectionType string  `json:"directionType"`
}

type tideForecastResponse struct {
	Associated forecastAssociated `json:"associated"`
	Data       struct {
		Tides []struct {
			Timestamp int64   `json:"timestamp"`
			Type      string  `json:"type"`
			Height    float64 `json:"height"`
		} `json:"tides"`
	} `json:"data"`
}

type weatherForecastResponse struct {
	Associated forecastAssociated `json:"associated"`
	Data       struct {
		Weather []weatherForecastPoint `json:"weather"`
	} `json:"data"`
}

type weatherForecastPoint struct {
	Timestamp   int64    `json:"timestamp"`
	Temperature *float64 `json:"temperature"`
}

type ratingForecastResponse struct {
	Associated forecastAssociated `json:"associated"`
	Data       struct {
		Rating []struct {
			Timestamp int64          `json:"timestamp"`
			Rating    surflineRating `json:"rating"`
		} `json:"rating"`
	} `json:"data"`
}

type surflineRating struct {
	Key   string `json:"key"`
	Value *int   `json:"value"`
}

func (r surflineRating) label() string {
	if r.Key != "" {
		if label, ok := map[string]string{
			"VERY_POOR":    "Very Poor",
			"POOR":         "Poor",
			"POOR_TO_FAIR": "Poor to Fair",
			"FAIR":         "Fair",
			"FAIR_TO_GOOD": "Fair to Good",
			"GOOD":         "Good",
			"VERY_GOOD":    "Very Good",
			"EPIC":         "Epic",
		}[r.Key]; ok {
			return label
		}
		return strings.ReplaceAll(r.Key, "_", " ")
	}
	if r.Value == nil {
		return ""
	}
	labels := []string{"Poor", "Poor to Fair", "Fair", "Fair to Good", "Good", "Very Good", "Epic"}
	if *r.Value < 0 || *r.Value >= len(labels) {
		return ""
	}
	return labels[*r.Value]
}
