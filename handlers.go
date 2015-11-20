package main

import (
	"encoding/json"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"net/http"
)

func CocoAggregateHealthHandler(registry ServiceRegistry, checker ServiceHealthChecker) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := []fthealth.Check{}
		for _, service := range registry.Services() {
			checks = append(checks, NewCocoServiceHealthCheck(service, checker))
		}

		if r.Header.Get("Accept") == "application/json" {
			jsonHandler("Coco Aggregate Healthcheck", "Checks the health of all deployed services", checks...)(w, r)
		} else {

			htmlHandler("Coco Aggregate Healthcheck", "Checks the health of all deployed services", checks...)(w, r)
		}
	}
}

func jsonHandler(name, description string, checks ...fthealth.Check) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		health := fthealth.RunCheck(name, description, true, checks...)

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		err := enc.Encode(health)
		if err != nil {
			panic("write this bit")
		}
	}
}

func htmlHandler(name, description string, checks ...fthealth.Check) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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

		checkResult := fthealth.RunCheck(name, name, true, checks...)
		for _, check := range checkResult.Checks {
			serviceHealthUrl := fmt.Sprintf("/health/%s/__health", check.Name)
			if !check.Ok {
				servicesHtml += fmt.Sprintf(serviceHtmlTemplate, serviceHealthUrl, check.Name, "CRITICAL")
			} else {
				servicesHtml += fmt.Sprintf(serviceHtmlTemplate, serviceHealthUrl, check.Name, "OK")
			}

		}

		w.Header().Add("Content-Type", "text/html")
		fmt.Fprintf(w, resp, name, servicesHtml)
	}
}
