package main

import (
	"encoding/json"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"net/http"
)

type hchandlers struct {
	registry    ServiceRegistry
	checker     ServiceHealthChecker
	name        string
	description string
}

func NewHCHandlers(registry ServiceRegistry, checker ServiceHealthChecker) *hchandlers {
	return &hchandlers{registry, checker, "Coco Aggregate Healthcheck", "Checks the health of all deployed services"}
}

func (hch *hchandlers) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		hch.jsonHandler(w, r)
	} else {
		hch.htmlHandler(w, r)
	}

}

func (hch *hchandlers) result() fthealth.HealthResult {
	checks := []fthealth.Check{}
	for _, service := range hch.registry.Services() {
		checks = append(checks, NewCocoServiceHealthCheck(service, hch.checker))
	}
	return fthealth.RunCheck(hch.name, hch.description, true, checks...)
}

func (hch *hchandlers) jsonHandler(w http.ResponseWriter, r *http.Request) {
	health := hch.result()

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

	checkResult := hch.result()
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
