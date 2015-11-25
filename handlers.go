package main

import (
	"encoding/json"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"strconv"
	"time"
)

type hchandlers struct {
	registry     ServiceRegistry
	checker      ServiceHealthChecker
	name         string
	description  string
	latestResult <-chan fthealth.HealthResult
	kapi         client.KeysAPI
}

func NewHCHandlers(registry ServiceRegistry, checker ServiceHealthChecker, kapi client.KeysAPI) *hchandlers {
	lr := make(chan fthealth.HealthResult)
	hch := &hchandlers{registry, checker, "Coco Aggregate Healthcheck", "Checks the health of all deployed services", lr, kapi}
	go hch.loop(lr)
	return hch
}

func (hch *hchandlers) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		hch.jsonHandler(w, r)
	} else {
		hch.htmlHandler(w, r)
	}

}

const (
	defaultDuration = time.Duration(60 * time.Second)
	keyName         = "/ft/config/aggregate_healthcheck_period_seconds"
)

func (hch *hchandlers) loop(lr chan<- fthealth.HealthResult) {

	newResult := make(chan fthealth.HealthResult)

	hcPeriod := make(chan time.Duration)

	go func() {

		response, err := hch.kapi.Get(context.Background(), keyName, nil)
		if err != nil {
			log.Printf("failed to get value from %v because %v. Using default of %v\n", keyName, err.Error(), defaultDuration)
			hcPeriod <- defaultDuration
		} else {
			initialPeriod, err := strconv.Atoi(response.Node.Value)
			if err != nil {
				log.Printf("error reading health check period value '%v'. Defaulting to %v", response.Node.Value, defaultDuration)
				hcPeriod <- defaultDuration
			}
			hcPeriod <- time.Duration(time.Duration(initialPeriod) * time.Second)
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
				hcPeriod <- defaultDuration
				continue
			}
			hcPeriod <- time.Duration(time.Duration(newPeriod) * time.Second)
		}

	}()

	go func() {

		// get initial period
		period := <-hcPeriod
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
				log.Printf("got new health results in %v\n", time.Now().Sub(start))
				newResult <- health
			case period = <-hcPeriod:
				log.Printf("updated health check period to %v\n", period)
			}

			timer = time.NewTimer(period)
		}
	}()

	var latest fthealth.HealthResult
	for {
		select {
		case latest = <-newResult:
		case lr <- latest:
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
