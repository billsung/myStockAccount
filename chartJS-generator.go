package main

import (
	"fmt"
	"math"
	"strconv"

	"golang.org/x/exp/rand"
)

const MA5_SKYBLUE string = "rgba(97, 141, 212, 0.6)"
const MA10_YELLOW string = "rgba(214, 194, 92, 0.6)"
const MA20_PURPLE string = "rgba(245, 158, 255, 0.6)"

type DataPoint struct {
	X string  `json:"x,omitempty"` // Time
	O float64 `json:"o,omitempty"` // Open price
	H float64 `json:"h,omitempty"` // High price
	L float64 `json:"l,omitempty"` // Low price
	C float64 `json:"c,omitempty"` // Close price
	Y float64 `json:"y,omitempty"` // Volume,MA
}

// ChartJS Dataset
type Dataset struct {
	Type               string      `json:"type"`
	Label              string      `json:"label"`
	Data               []DataPoint `json:"data"`
	BackgroundColor    interface{} `json:"backgroundColor,omitempty"` // Dynamic or static color
	BorderColor        interface{} `json:"borderColor,omitempty"`
	BorderWidth        int         `json:"borderWidth,omitempty"`
	YAxisID            string      `json:"yAxisID"`
	BarPercentage      float64     `json:"barPercentage,omitempty"`
	CategoryPercentage float64     `json:"categoryPercentage,omitempty"`
	Fill               bool        `json:"fill,omitempty"`
}

// ChartJS Config
type ChartConfig struct {
	Type string `json:"type"`
	Data struct {
		Labels   []string  `json:"labels"`
		Datasets []Dataset `json:"datasets"`
	} `json:"data"`
	Options map[string]interface{} `json:"options"`
}

type GenericDataset struct {
	Type            string      `json:"type"`
	Label           string      `json:"label"`
	Data            []float64   `json:"data"`
	BackgroundColor interface{} `json:"backgroundColor,omitempty"`
	BorderColor     interface{} `json:"borderColor,omitempty"`
	BorderWidth     int         `json:"borderWidth,omitempty"`
	HoverOffset     int         `json:"hoverOffset,omitempty"`
}

type GenericChartConfig struct {
	Type string `json:"type"`
	Data struct {
		Labels   []string         `json:"labels"`
		Datasets []GenericDataset `json:"datasets"`
	} `json:"data"`
	Options map[string]interface{} `json:"options"`
}

func GenCandleDataPoint(x string, o float64, h float64, l float64, c float64) DataPoint {
	return DataPoint{X: x, O: o, H: h, L: l, C: c}
}

func GenXYDataPoint(x string, y float64) DataPoint {
	return DataPoint{X: x, Y: y}
}

func GenCandleDataset(ticket string, candle []DataPoint) Dataset {
	return Dataset{
		Type:    "candlestick",
		Label:   ticket,
		Data:    candle,
		YAxisID: "price",
	}
}

func GenLineDataset(name string, data []DataPoint, color string) Dataset {
	return Dataset{
		Type:        "line",
		Label:       name,
		Data:        data,
		BorderColor: color,
		Fill:        false,
		YAxisID:     "price",
		BorderWidth: 2,
	}
}

func GenVolumeDataset(volume []DataPoint) Dataset {
	return Dataset{
		Type:               "bar",
		Label:              "Volume",
		Data:               volume,
		YAxisID:            "volume",
		BarPercentage:      0.8,
		CategoryPercentage: 0.8,
		BackgroundColor:    "rgba(75, 192, 192, 0.6)", // Static color
	}
}

func GenGenericDataset(graphType string, graphName string, val []float64, bgColor []string) GenericDataset {
	return GenericDataset{
		Type:            graphType,
		Label:           graphName,
		Data:            val,
		BackgroundColor: bgColor,
		HoverOffset:     2,
	}
}

func GenBGColor() string {
	r := rand.Int63n(256)
	g := rand.Int63n(256)
	b := rand.Int63n(256)
	return fmt.Sprintf("rgba(%s, %s, %s, 0.6)", strconv.FormatInt(r, 10), strconv.FormatInt(g, 10), strconv.FormatInt(b, 10))
}

func GenCandleStickChartConfig(labels []string, datasets []Dataset) ChartConfig {
	config := ChartConfig{Type: "candlestick"}

	config.Data.Labels = labels
	config.Data.Datasets = datasets

	nr := len(datasets[0].Data)
	hh := 0.0
	ll := math.MaxFloat64
	vhh := 0.0
	for i := 0; i < nr; i += 1 {
		hh = math.Max(hh, datasets[0].Data[i].H)
		ll = math.Min(ll, datasets[0].Data[i].L)
		vhh = math.Max(vhh, datasets[1].Data[i].Y)
	}
	hh = hh * 1.05
	ll = ll * 0.95
	height := hh - ll
	ll -= (height * 0.2)

	vhh = vhh * 5

	config.Options = map[string]interface{}{
		"responsive": true,
		"scales": map[string]interface{}{
			"x": map[string]interface{}{
				"type": "category",
				"ticks": map[string]interface{}{
					"autoSkip": false,
				},
			},
			"price": map[string]interface{}{
				"type":        "linear",
				"max":         hh,
				"min":         ll,
				"beginAtZero": false,
				"position":    "left",
				"title": map[string]interface{}{
					"display": false,
					"text":    "Price",
				},
			},
			"volume": map[string]interface{}{
				"type":     "linear",
				"max":      vhh,
				"min":      0,
				"position": "right",
				"title": map[string]interface{}{
					"display": false,
					"text":    "Volume",
				},
				"grid": map[string]interface{}{
					"drawOnChartArea": false,
				},
			},
		},
	}

	return config
}

func GenGenericChartConfig(graphType string, labels []string, datasets []GenericDataset) GenericChartConfig {
	config := GenericChartConfig{Type: graphType}

	config.Data.Labels = labels
	config.Data.Datasets = datasets

	config.Options = map[string]interface{}{
		"responsive": true,
		"plugins": map[string]interface{}{
			"legend": map[string]interface{}{
				"display": false,
			},
		},
	}

	return config
}
