package main

import (
	"encoding/json"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const timeLayout = "15:04:05 MST"

var defaultCategories = []string{"default"}

type Controller struct {
	registry    *ServiceRegistry
	environment *string
}

func NewController(registry *ServiceRegistry, environment *string) *Controller {
	return &Controller{registry, environment}
}

func (c Controller) buildHealthResultFor(categories []string, useCache bool) (fthealth.HealthResult, []string) {
	var checkResults []fthealth.CheckResult
	matchingCategories := c.registry.matchingCategories(categories)
	desc := "Health of the whole cluster of the moment served directly."
	if useCache {
		checkResults = c.collectChecksFromCachesFor(categories)
		desc = "Health of the whole cluster served from cache."
	} else {
		checkResults = c.runChecksFor(categories)
	}
	var finalOk bool
	var finalSeverity uint8
	if c.registry.areResilient(matchingCategories) {
		finalOk, finalSeverity = c.computeResilientHealthResult(checkResults)
	} else {
		finalOk, finalSeverity = c.computeNonResilientHealthResult(checkResults)
	}
	return fthealth.HealthResult{
		Checks:        checkResults,
		Description:   desc,
		Name:          *c.environment + " cluster health",
		SchemaVersion: 1,
		Ok:            finalOk,
		Severity:      finalSeverity,
	}, matchingCategories
}

func (c Controller) collectChecksFromCachesFor(categories []string) []fthealth.CheckResult {
	var checkResults []fthealth.CheckResult
	for _, mService := range c.registry.measuredServices {
		if !containsAtLeastOneFrom(categories, mService.service.Categories) {
			continue
		}
		healthResult := <-mService.cachedHealth.toReadFromCache
		if len(healthResult.Checks) == 0 {
			continue
		}
		checkResult := NewCheckFromSingularHealthResult(healthResult)
		checkResults = append(checkResults, checkResult)
	}
	return checkResults
}

func (c Controller) runChecksFor(categories []string) []fthealth.CheckResult {
	var checks []fthealth.Check
	for _, mService := range c.registry.measuredServices {
		if !containsAtLeastOneFrom(categories, mService.service.Categories) {
			continue
		}
		checks = append(checks, NewServiceHealthCheck(*mService.service, c.registry.checker))
	}
	return fthealth.RunCheck("Forced check run", "", true, checks...).Checks
}

func (c Controller) computeResilientHealthResult(checkResults []fthealth.CheckResult) (bool, uint8) {
	finalOk := true
	var finalSeverity uint8 = 2
	oks := make(map[string]bool)
	severities := make(map[string]uint8)
	for _, result := range checkResults {
		serviceGroupName := result.Name[0:strings.LastIndex(result.Name, "-")]
		if _, isPresent := severities[serviceGroupName]; !isPresent || result.Ok {
			severities[serviceGroupName] = result.Severity
			oks[serviceGroupName] = result.Ok
		}
	}
	for serviceGroupName, ok := range oks {
		if !ok {
			finalOk = false
			if severities[serviceGroupName] < finalSeverity {
				finalSeverity = severities[serviceGroupName]
			}
		}
	}
	return finalOk, finalSeverity
}

func (c Controller) computeNonResilientHealthResult(checkResults []fthealth.CheckResult) (bool, uint8) {
	finalOk := true
	var finalSeverity uint8 = 2
	for _, result := range checkResults {
		if !result.Ok {
			finalOk = false
			if result.Severity < finalSeverity {
				finalSeverity = result.Severity
			}
		}
	}
	return finalOk, finalSeverity
}

func (c Controller) handleHealthcheck(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		c.jsonHandler(w, r)
	} else {
		c.htmlHandler(w, r)
	}
}

func (c Controller) handleGoodToGo(w http.ResponseWriter, r *http.Request) {
	categories := parseCategories(r.URL)
	healthResults, validCategories := c.buildHealthResultFor(categories, useCache(r.URL))
	if len(validCategories) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !healthResults.Ok {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func (c Controller) jsonHandler(w http.ResponseWriter, r *http.Request) {
	categories := parseCategories(r.URL)
	healthResults, validCategories := c.buildHealthResultFor(categories, useCache(r.URL))
	w.Header().Set("Content-Type", "application/json")
	if len(validCategories) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	enc := json.NewEncoder(w)
	err := enc.Encode(healthResults)
	if err != nil {
		panic("Couldn't encode health results to ResponseWriter.")
	}
}

func (c Controller) htmlHandler(w http.ResponseWriter, r *http.Request) {
	categories := parseCategories(r.URL)
	w.Header().Add("Content-Type", "text/html")
	health, validCategories := c.buildHealthResultFor(categories, useCache(r.URL))
	if len(validCategories) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Category does not exist."))
		return
	}
	htmlTemplate := "<!DOCTYPE html>" +
		"<head>" +
		"<title>CoCo Aggregate Healthcheck</title>" +
		"</head>" +
		"<body>" +
		"<h1>CoCo " + *c.environment + " cluster's " + strings.Join(validCategories, ", ") + " services are "
	if health.Ok {
		htmlTemplate += "<span style='color: green;'>healthy</span></h1>"
	} else {
		if health.Severity > 1 {
			htmlTemplate += "<span style='color: orange;'>unhealthy</span></h1>"
		} else {
			htmlTemplate += "<span style='color: red;'>CRITICAL</span></h1>"
		}
	}
	htmlTemplate += "<table style='font-size: 10pt; font-family: MONOSPACE;'>" +
		"%s" +
		"</table>" +
		"</body>" +
		"</html>"
	serviceTrTemplate := "<tr><td><a href=\"%s\">%s</a></td><td>&nbsp;%s</td><td>&nbsp;<td>&nbsp;%v</td></tr>\n"
	serviceURLTemplate := "/health/%s/__health"
	servicesHTML := ""
	sort.Sort(ByName(health.Checks))
	for _, check := range health.Checks {
		serviceHealthURL := fmt.Sprintf(serviceURLTemplate, check.Name)
		var status string
		if check.Ok {
			status = "<span style='color: green;'>OK</span>"
		} else {
			if check.Severity > 1 {
				status = "<span style='color: orange;'>WARNING</span>"
			} else {
				status = "<span style='color: red;'>CRITICAL</span>"
			}
		}
		servicesHTML += fmt.Sprintf(serviceTrTemplate, serviceHealthURL, check.Name, status, check.LastUpdated.Format(timeLayout))
	}
	fmt.Fprintf(w, htmlTemplate, servicesHTML)
}

func useCache(theURL *url.URL) bool {
	//use cache by default
	return theURL.Query().Get("cache") != "false"
}

func parseCategories(theURL *url.URL) []string {
	queriedCategories := theURL.Query().Get("categories")
	if queriedCategories == "" {
		return defaultCategories
	}
	return strings.Split(queriedCategories, ",")
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
