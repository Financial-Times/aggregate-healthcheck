package main

import (
	"encoding/json"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sort"
)

var defaultCategories = []string { "default" }

type controller struct {
	registry       *ServiceRegistry
}

func NewController(registry *ServiceRegistry) *controller {
	return &controller{registry}

	// set up channels for buffering data to be sent to Graphite
	//bufferGraphite := make(chan *TimedHealth, 10)

	// start checking health and activate handlers to respond on read signals
	//graphiteTicker := time.NewTicker(79 * time.Second)
	//go graphiteFeeder.maintainGraphiteFeed(bufferGraphite, graphiteTicker)
}

func (c controller) combineHealthResults(categories []string) fthealth.HealthResult {
	var allChecksFromResults []fthealth.Check
	log.Printf("DEBUG - Combining health results.", )
	for _, mService := range c.registry.measuredServices {
		if containsAtLeastOneFrom(mService.service.Categories, categories) {
			healthResult := <-mService.cachedHealth.toReadFromCache
			log.Printf("DEBUG - Health result for service [%v] is: [%v].", mService.service.Name, healthResult.Ok)
			checkFromResult := NewCheckFromSingularHealthResult(healthResult)
			allChecksFromResults = append(allChecksFromResults, checkFromResult)
		}
	}
	return fthealth.RunCheck("Cluster health", "Checks the health of the whole cluster", true, allChecksFromResults...)
}

func (c controller) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		c.jsonHandler(w, r)
	} else {
		c.htmlHandler(w, r)
	}
}

func (c controller) jsonHandler(w http.ResponseWriter, r *http.Request) {
	categories := parseCategories(r.URL)
	health := c.combineHealthResults(categories)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(health)
	if err != nil {
		panic("Couldn't encode health results to ResponseWriter.")
	}
}

func (c controller) htmlHandler(w http.ResponseWriter, r *http.Request) {
	categories := parseCategories(r.URL)
	w.Header().Add("Content-Type", "text/html")
	health := c.combineHealthResults(categories)
	htmlTemplate := "<!DOCTYPE html>" +
		"<head>" +
		"<title>CoCo Aggregate Healthcheck</title>" +
		"</head>" +
		"<body>" +
		"<h1>CoCo Aggregate Healthcheck</h1>" +
		"<table>" +
		"%s" +
		"</table>" +
		"</body>" +
		"</html>"
	serviceTrTemplate := "<tr><td><a href=\"%s\">%s</a></td><td>%s</td><td>%v</td></tr>\n"
	serviceUrlTemplate := "/health/%s/__health"
	servicesHtml := ""
	sort.Sort(ByName(health.Checks))
	for _, check := range health.Checks {
		serviceHealthUrl := fmt.Sprintf(serviceUrlTemplate, check.Name)
		status := "CRITICAL"
		if check.Ok {
			status = "OK"
		}
		servicesHtml += fmt.Sprintf(serviceTrTemplate, serviceHealthUrl, check.Name, status, check.LastUpdated)
	}
	fmt.Fprintf(w, htmlTemplate, servicesHtml)
}

func parseCategories(theUrl *url.URL) []string {
	u, err := url.Parse(theUrl.String())
	if err != nil {
		log.Printf("INFO - Error parsing HTTP URL: %v", theUrl)
		return defaultCategories
	}
	q, _ := url.ParseQuery(u.RawQuery)
	fmt.Printf("DEBUG - q[\"categories\"] = %v\n", q["categories"])
	if len(q["categories"]) < 1 {
		return defaultCategories
	}
	categories := strings.Split(q["categories"][0], ",")
	fmt.Printf("DEBUG - %v\n", len(categories))
	return categories
}

func containsAtLeastOneFrom(s []string, e []string) bool {
	for _, a := range s {
		for _, b := range e {
			if a == b {
				return true
			}
		}
	}
	return false
}

type ByName []fthealth.CheckResult

func (s ByName) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s ByName) Len() int {
	return len(s)
}
func (s ByName) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
