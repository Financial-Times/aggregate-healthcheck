package main

import (
	"encoding/json"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"log"
	"net/http"
)

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

func (c controller) combineHealthResults() fthealth.HealthResult {
	var allChecksFromResults []fthealth.Check
	log.Printf("DEBUG - Combining health results.", )
	for _, mService := range c.registry.measuredServices {
		healthResult := <-mService.cachedHealth.toReadFromCache
		log.Printf("DEBUG - Health result for service [%v] is: [%v].", mService.service.Name, healthResult.Ok)
		checkFromResult := NewCheckFromSingularHealthResult(healthResult)
		allChecksFromResults = append(allChecksFromResults, checkFromResult)
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
	health := c.combineHealthResults()
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(health)
	if err != nil {
		panic("Couldn't encode health results to ResponseWriter.")
	}
}

func (c controller) htmlHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	health := c.combineHealthResults()
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
