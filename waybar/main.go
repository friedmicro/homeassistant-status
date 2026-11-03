package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultAPIURL = "http://api.serpents.friedmicro-lab.org/status"
const configFile = "has-waybar.conf"

type Output struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip,omitempty"`
	Class   string `json:"class,omitempty"`
}

type statusResponse struct {
	Computed struct {
		DenTemperature     float64 `json:"den_temperature"`
		DenTemperatureUnit string  `json:"den_temperature_unit"`
		BaseStationsOn     bool    `json:"base_stations_on"`
		KettleOn           bool    `json:"kettle_on"`
		OffloadedComputeOn bool    `json:"offloaded_compute_on"`
		Sun                struct {
			NextEventInHours float64 `json:"next_event_in_hours"`
			NextState        string  `json:"next_state"`
			State            string  `json:"state"`
		}
		Flags struct {
			BaseStationServiceOffline bool `json:"base_station_service_offline"`
			LuciaPhoneBatteryLow      bool `json:"lucia_phone_battery_low"`
		} `json:"flags"`
	} `json:"computed"`
}

func apiURL() string {
	if v := os.Getenv("HAS_WAYBAR_URL"); v != "" {
		return v
	}
	if exe, err := os.Executable(); err == nil {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(exe), configFile))
		if err == nil {
			sc := bufio.NewScanner(strings.NewReader(string(b)))
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if strings.HasPrefix(line, "apiurl=") {
					if v := strings.TrimSpace(strings.TrimPrefix(line, "apiurl=")); v != "" {
						return v
					}
				}
			}
		}
	}
	return defaultAPIURL
}

func fetchStatus(url string) (*statusResponse, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var s statusResponse
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func renderDayTime(status *statusResponse) string {
	var dayTime string
	switch status.Computed.Sun.State {
	case "dawn":
		dayTime += "🌿"
	case "rising":
		dayTime += "🌄"
	case "noon":
		dayTime += "🌞"
	case "setting":
		dayTime += "🌇"
	case "dusk":
		dayTime += "🌆"
	}
	return dayTime + strconv.FormatFloat(status.Computed.Sun.NextEventInHours, 'f', 2, 64)
}

func formatTemp(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func render(s *statusResponse) Output {
	var parts []string
	if s.Computed.Flags.LuciaPhoneBatteryLow {
		parts = append(parts, "📱")
	}
	if s.Computed.BaseStationsOn {
		parts = append(parts, "🛙")
	}
	if s.Computed.KettleOn {
		parts = append(parts, "🫖")
	}
	if s.Computed.OffloadedComputeOn {
		parts = append(parts, "⚡")
	}
	parts = append(parts, renderDayTime(s))
	parts = append(parts, "🌡️"+formatTemp(s.Computed.DenTemperature))

	tooltip := fmt.Sprintf("den: %s %s", formatTemp(s.Computed.DenTemperature), strings.TrimSpace(s.Computed.DenTemperatureUnit))
	tooltip += "\nkettle: " + boolWord(s.Computed.KettleOn)
	tooltip += "\nbase stations: " + boolWord(s.Computed.BaseStationsOn)
	tooltip += "\noffloaded compute: " + boolWord(s.Computed.OffloadedComputeOn)
	tooltip += "\nphone battery low: " + boolWord(s.Computed.Flags.LuciaPhoneBatteryLow)

	class := "ok"
	switch {
	case s.Computed.Flags.LuciaPhoneBatteryLow:
		class = "low-battery"
	case s.Computed.Flags.BaseStationServiceOffline:
		class = "service-offline"
	}

	return Output{
		Text:    strings.Join(parts, " "),
		Tooltip: tooltip,
		Class:   class,
	}
}

func boolWord(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func main() {
	s, err := fetchStatus(apiURL())
	if err != nil {
		out, _ := json.Marshal(Output{Text: "⚠️", Class: "unhealthy", Tooltip: err.Error()})
		fmt.Println(string(out))
		os.Exit(1)
	}
	out, _ := json.Marshal(render(s))
	fmt.Println(string(out))
}
