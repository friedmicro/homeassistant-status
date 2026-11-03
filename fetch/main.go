package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

func parseEntityList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func fetchOne(haURL, token, entity string) (entityState, error) {
	var e entityState
	url := fmt.Sprintf("%s/api/states/%s", haURL, entity)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return e, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return e, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return e, fmt.Errorf("ha returned %d for %s: %s", resp.StatusCode, entity, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return e, err
	}
	if e.EntityID == "" {
		e.EntityID = entity
	}
	return e, nil
}

func fetchAll(haURL, token string, entities []string) []entityState {
	out := make([]entityState, 0, len(entities))
	for _, id := range entities {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		e, err := fetchOne(haURL, token, id)
		if err != nil {
			out = append(out, entityState{EntityID: id, State: "unknown",
				Attributes: map[string]interface{}{"fetch_error": err.Error()}})
			log.Printf("error fetching %s: %v", id, err)
			continue
		}
		out = append(out, e)
	}
	return out
}

func main() {
	port := getenv("PORT", "8080")
	haURL := strings.TrimRight(getenv("HA_URL", "http://serpent.friedmicro-lab.org:8123"), "/")
	haToken := os.Getenv("HA_TOKEN")
	entities := parseEntityList(os.Getenv("ENTITIES"))

	if haToken == "" {
		log.Fatal("HA_TOKEN is required")
	}
	if len(entities) == 0 {
		log.Fatal("ENTITIES list is empty; refusing to start with no entities configured")
	}
	log.Printf("configured %d entities to fetch from %s", len(entities), haURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("/entities", func(w http.ResponseWriter, r *http.Request) {
		states := fetchAll(haURL, haToken, entities)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(states); err != nil {
			log.Printf("error encoding response: %v", err)
		}
	})

	log.Printf("homeassistant-status-fetch listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
