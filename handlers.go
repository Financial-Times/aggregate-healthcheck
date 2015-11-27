package main

import (
	"encoding/json"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"log"
	"net/http"
	"time"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"strconv"
)

type hchandlers struct {
	registry       ServiceRegistry
	checker        ServiceHealthChecker
	name           string
	description    string
	latestResult   <-chan fthealth.HealthResult
	graphiteFeeder *GraphiteFeeder
	kapi           client.KeysAPI
	hcPeriod       chan time.Duration
}

const (
	defaultDuration = time.Duration(60 * time.Second)
	keyName         = "/ft/config/aggregate_healthcheck_period_seconds"
)

func NewHCHandlers(registry ServiceRegistry, checker ServiceHealthChecker, graphiteFeeder *GraphiteFeeder, kapi client.KeysAPI) *hchandlers {
	// set up channels for reading health statuses over HTTP
	latestRead := make(chan fthealth.HealthResult)
	latestWrite := make(chan fthealth.HealthResult)
	hcPeriod := make(chan time.Duration)
	hch := &hchandlers{registry, checker, "Coco Aggregate Healthcheck", "Checks the health of all deployed services",
		latestRead, graphiteFeeder, kapi, hcPeriod}

	// set up channels for buffering data to be sent to Graphite
	latestGraphiteWrite := make(chan *HealthTimed)
	latestGraphiteRead := make(chan *HealthTimed, 10)
	ring := NewRingBuffer(latestGraphiteWrite, latestGraphiteRead)
	go ring.Run()

	// start checking health and activate handlers to respond on read signals
	graphiteTicker := time.NewTicker(79 * time.Second)

	go hch.loop(latestWrite, latestGraphiteWrite)
	go hch.maintainDelayPeriod()
	go graphiteFeeder.MaintainGraphiteFeed(latestGraphiteRead, graphiteTicker)
	go maintainLatest(latestRead, latestWrite)
	return hch
}

func (hch *hchandlers) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		hch.jsonHandler(w, r)
	} else {
		hch.htmlHandler(w, r)
	}
}

func (hch *hchandlers) loop(latestWrite chan<- fthealth.HealthResult, latestGraphite chan<- *HealthTimed) {

	// get initial period
	period := <-hch.hcPeriod
	log.Printf("set health check period to %v\n", period)

	timer := time.NewTimer(0)

	for {
		select {
		case <-timer.C:
			checks := []fthealth.Check{}
			for _, service := range hch.registry.Services() {
				checks = append(checks, NewCocoServiceHealthCheck(service, hch.checker))
			}
			start := time.Now()
			health := fthealth.RunCheck(hch.name, hch.description, true, checks...)
			now := time.Now()
			log.Printf("got new health results in %v\n", now.Sub(start))
			latestWrite <- health
			latestGraphite <- NewHealthTimed(health, now)
		case period = <-hch.hcPeriod:
			log.Printf("updated health check period to %v\n", period)
		}

		timer = time.NewTimer(period)
	}
}

func (hch *hchandlers) maintainDelayPeriod() {
	response, err := hch.kapi.Get(context.Background(), keyName, nil)
	if err != nil {
		log.Printf("failed to get value from %v because %v. Using default of %v\n", keyName, err.Error(), defaultDuration)
		hch.hcPeriod <- defaultDuration
	} else {
		initialPeriod, err := strconv.Atoi(response.Node.Value)
		if err != nil {
			log.Printf("error reading health check period value '%v'. Defaulting to %v", response.Node.Value, defaultDuration)
			hch.hcPeriod <- defaultDuration
		}
		hch.hcPeriod <- time.Duration(time.Duration(initialPeriod) * time.Second)
	}

	watcher := hch.kapi.Watcher(keyName, nil)
	for {
		next, err := watcher.Next(context.Background())
		if err != nil {
			log.Printf("error waiting for new hc period. sleeping 10s: %v\n", err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		newPeriod, err := strconv.Atoi(next.Node.Value)
		if err != nil {
			log.Printf("error reading health check period value '%v'. Defaulting to %v", next.Node.Value, defaultDuration)
			hch.hcPeriod <- defaultDuration
			continue
		}
		hch.hcPeriod <- time.Duration(time.Duration(newPeriod) * time.Second)
	}
}

func maintainLatest(latestRead chan<- fthealth.HealthResult, latestWrite <-chan fthealth.HealthResult) {
	var latest fthealth.HealthResult
	for {
		select {
		case latest = <-latestWrite:
		case latestRead <- latest:
		}
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
