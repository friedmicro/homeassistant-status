package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type entityState struct {
	EntityID   string                 `json:"entity_id"`
	State      string                 `json:"state"`
	Attributes map[string]interface{} `json:"attributes"`
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fetchRaw(fetchURL string) ([]entityState, error) {
	resp, err := http.Get(fetchURL + "/entities")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var states []entityState
	if err := json.Unmarshal(body, &states); err != nil {
		return nil, err
	}
	return states, nil
}

func stateOf(ents map[string]entityState, id string) (string, bool) {
	e, ok := ents[id]
	if !ok {
		return "", false
	}
	return e.State, true
}

func baseStationOffline(ents map[string]entityState) bool {
	for _, n := range []string{"1", "2", "3", "4"} {
		st, ok := stateOf(ents, "sensor.lucia_base_station_"+n+"_power_state")
		if ok && st == "unknown" {
			return true
		}
	}
	return false
}

func luciaPhoneBatteryLow(ents map[string]entityState) bool {
	level, ok1 := stateOf(ents, "sensor.pixel_11_pro_battery_level")
	chg, ok2 := stateOf(ents, "sensor.pixel_11_pro_battery_state")
	if !ok1 || !ok2 {
		return false
	}
	lv, err := strconv.Atoi(level)
	if err != nil {
		return false
	}
	return lv <= 20 && chg == "discharging"
}

var sunOrder = []string{"dawn", "rising", "noon", "setting", "dusk"}
var sunSensors = map[string]string{
	"dawn":    "sensor.sun_next_dawn",
	"rising":  "sensor.sun_next_rising",
	"noon":    "sensor.sun_next_noon",
	"setting": "sensor.sun_next_setting",
	"dusk":    "sensor.sun_next_dusk",
}

func parseEpoch(v string) int64 {
	s := strings.TrimSpace(v)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix()
	}
	return 0
}

func sunInfo(ents map[string]entityState) map[string]interface{} {
	now := time.Now().Unix()
	next := ""
	var nextTs int64 = 1<<62 - 1
	for name, sensor := range sunSensors {
		st, ok := stateOf(ents, sensor)
		if !ok {
			continue
		}
		ts := parseEpoch(st)
		if ts > now && ts < nextTs {
			nextTs = ts
			next = name
		}
	}
	if next == "" {
		return map[string]interface{}{"state": "unknown", "next_state": "unknown", "next_event_in_hours": nil}
	}
	cur := ""
	for i, n := range sunOrder {
		if n == next {
			cur = sunOrder[(i+len(sunOrder)-1)%len(sunOrder)]
			break
		}
	}
	hours := float64(nextTs-now) / 3600.0
	hours = float64(int(hours*10)) / 10
	return map[string]interface{}{
		"state":               cur,
		"next_state":          next,
		"next_event_in_hours": hours,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func denTemp(ents map[string]entityState) (float64, bool) {
	st, ok := stateOf(ents, "sensor.apollo_msr_2_414570_dps310_temperature")
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(st, 64)
	if err != nil {
		return 0, false
	}
	return round2(v), true
}

func denTempUnit(ents map[string]entityState) string {
	if e, ok := ents["sensor.apollo_msr_2_414570_dps310_temperature"]; ok {
		if u, ok := e.Attributes["unit_of_measurement"].(string); ok {
			return u
		}
	}
	return ""
}

func anyBaseStationOn(ents map[string]entityState) bool {
	for _, n := range []string{"1", "2", "3", "4"} {
		if st, ok := stateOf(ents, "sensor.lucia_base_station_"+n+"_power_state"); ok && st != "Sleep" {
			return true
		}
	}
	return false
}

func switchOn(ents map[string]entityState, id string) bool {
	return mustState(ents, id) == "on"
}

func buildResponse(ents map[string]entityState) map[string]interface{} {
	raw := make(map[string]entityState, len(ents))
	for _, e := range ents {
		raw[e.EntityID] = e
	}

	computed := map[string]interface{}{
		"den_temperature":      mustState(ents, "sensor.apollo_msr_2_414570_dps310_temperature"),
		"den_temperature_unit": denTempUnit(ents),
		"base_stations_on":     anyBaseStationOn(ents),
		"kettle_on":            switchOn(ents, "switch.kettle"),
		"offloaded_compute_on": switchOn(ents, "switch.lucia_game_pc"),
		"lucia_phone": map[string]interface{}{
			"battery_level": mustState(ents, "sensor.pixel_11_pro_battery_level"),
			"charge_state":  mustState(ents, "sensor.pixel_11_pro_battery_state"),
		},
		"flags": map[string]bool{
			"base_station_service_offline": baseStationOffline(ents),
			"lucia_phone_battery_low":      luciaPhoneBatteryLow(ents),
		},
		"sun": sunInfo(ents),
	}
	if v, ok := denTemp(ents); ok {
		computed["den_temperature"] = v
	}

	return map[string]interface{}{
		"raw":      raw,
		"computed": computed,
	}
}

func mustState(ents map[string]entityState, id string) string {
	if st, ok := stateOf(ents, id); ok {
		return st
	}
	return "unknown"
}

func main() {
	port := getenv("PORT", "8080")
	fetchURL := strings.TrimRight(getenv("FETCH_URL", "http://homeassistant-status-fetch:8080"), "/")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		states, err := fetchRaw(fetchURL)
		if err != nil {
			log.Printf("error calling fetch service: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		ents := make(map[string]entityState, len(states))
		for _, e := range states {
			ents[e.EntityID] = e
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(buildResponse(ents)); err != nil {
			log.Printf("error encoding response: %v", err)
		}
	})

	log.Printf("homeassistant-status listening on :%s (fetch from %s)", port, fetchURL)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
