package main

import (
	"encoding/json"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"log"
	"net/http"
	"time"
	"github.com/coreos/etcd/client"
)

type hchandlers struct {
	registry       ServiceRegistry
	checker        HealthChecker
	latestResult   <-chan fthealth.HealthResult
	graphiteFeeder *GraphiteFeeder
	kapi           client.KeysAPI
	hcPeriod       chan time.Duration
}

const (



)

func NewHCHandlers(registry ServiceRegistry, checker HealthChecker, graphiteFeeder *GraphiteFeeder, kapi client.KeysAPI) *hchandlers {
	// set up channels for reading health statuses over HTTP

	hcPeriod := make(chan time.Duration)
	hch := &hchandlers{registry, checker,
		graphiteFeeder, kapi, hcPeriod}

	// set up channels for buffering data to be sent to Graphite
	bufferGraphite := make(chan *HealthTimed, 10)

	// start checking health and activate handlers to respond on read signals
	graphiteTicker := time.NewTicker(79 * time.Second)

	cache := NewCache()

	go hch.loop(cache.latestWrite, bufferGraphite)

	go graphiteFeeder.maintainGraphiteFeed(bufferGraphite, graphiteTicker)

	return hch
}

func (hch *hchandlers) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		hch.jsonHandler(w, r)
	} else {
		hch.htmlHandler(w, r)
	}
}




func (hch *hchandlers) jsonHandler(w http.ResponseWriter, r *http.Request) {
	health := <-hch.latestResult

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(health)
	if err != nil {
		panic("write this bit")
	}
}

func (hch *hchandlers) htmlHandler(w http.ResponseWriter, r *http.Request) {
	resp := "<!DOCTYPE html>" +
		"<head>" +
		"<title>Coco</title>" +
		"</head>" +
		"<body>" +
		"<h2>%s</h2>" +
		"<dl>" +
		"<dt>Services:</dt>" +
		"%s" +
		"</dl>" +
		"</body>" +
		"</html>"

	serviceHtmlTemplate := "<dd>- <a href=\"%s\">%s</a>  : %s</dd>"
	servicesHtml := ""

	checkResult := <-hch.latestResult
	for _, check := range checkResult.Checks {
		serviceHealthUrl := fmt.Sprintf("/health/%s/__health", check.Name)
		if !check.Ok {
			servicesHtml += fmt.Sprintf(serviceHtmlTemplate, serviceHealthUrl, check.Name, "CRITICAL")
		} else {
			servicesHtml += fmt.Sprintf(serviceHtmlTemplate, serviceHealthUrl, check.Name, "OK")
		}

	}

	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintf(w, resp, hch.name, servicesHtml)
}
