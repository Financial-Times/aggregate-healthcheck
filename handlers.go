package main

import (
	"fmt"
	"git.svc.ft.com/scm/gl/fthealth.git"
	"net/http"
)

func CocoAggregateHealthHandler(elbHost string, registry ServiceRegistry, checker ServiceHealthChecker) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := []fthealth.Check{}
		for _, service := range registry.Services() {
			checks = append(checks, NewCocoServiceHealthCheck(service, checker))
		}

		if r.Header.Get("Accept") == "application/json" {
			fthealth.Handler("Coco Aggregate Healthcheck", "Checks the health of all deployed services", checks...)(w, r)
		} else {

			htmlHandler("Coco Aggregate Healthcheck", elbHost, checks...)(w, r)
		}
	}
}

func htmlHandler(name, elbHost string, checks ...fthealth.Check) func(w http.ResponseWriter, r *http.Request) {
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

		for _, check := range checks {
			serviceHealthUrl := fmt.Sprintf("http://%s/health/%s/__health", elbHost, check.Name)
			err := check.Checker()
			if err != nil {
				servicesHtml += fmt.Sprintf(serviceHtmlTemplate, serviceHealthUrl, check.Name, "CRITICAL")
			} else {
				servicesHtml += fmt.Sprintf(serviceHtmlTemplate, serviceHealthUrl, check.Name, "OK")
			}

		}

		w.Header().Add("Content-Type", "text/html")
		fmt.Fprintf(w, resp, name, servicesHtml)
	}
}
