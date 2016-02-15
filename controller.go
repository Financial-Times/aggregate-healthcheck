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
}

func (c controller) combineHealthResultsFor(categories []string) fthealth.HealthResult {
	checkResults := c.healthResultsToCheckResults(categories)
	return c.computeHealthResult(categories, checkResults)
}

func (c controller) healthResultsToCheckResults(categories []string) []fthealth.CheckResult {
	log.Printf("DEBUG - Combining health results.", )
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

func (c controller) computeHealthResult(categories []string, checkResults []fthealth.CheckResult) fthealth.HealthResult {
	intersectingCategories := getIntersectingCategories(categories, c.registry.categories)
	resilient := isResilient(intersectingCategories, c.registry.categories)
	finalOk := true
	finalSeverity := 100
	if resilient {
		resilientSeverities := make(map[string]int)
		for _, result := range checkResults {
			serviceGroupName := result.Name[0:strings.LastIndex(result.Name, "-")]
			if _, ok := resilientSeverities[serviceGroupName]; ok {
				if resilientSeverities[serviceGroupName] >= 0 && result.Ok {
					resilientSeverities[serviceGroupName] = -1 //means ok
				}
			} else {
				if result.Ok {
					resilientSeverities[serviceGroupName] = -1
				} else {
					resilientSeverities[serviceGroupName] = int(result.Severity) //means not ok
				}
			}
		}
		for _, resilientSeverity := range resilientSeverities {
			if resilientSeverity >= 0 {
				finalOk = false
				if resilientSeverity < finalSeverity {
					finalSeverity = resilientSeverity
				}
			}
		}
	} else {
		for _, result := range checkResults {
			if !result.Ok {
				finalOk = false
				if int(result.Severity) < finalSeverity {
					finalSeverity = int(result.Severity)
				}
			}
		}
	}
	return fthealth.HealthResult{
		Checks: checkResults,
		Description: "Checks the health of the whole cluster.",
		Name: "Cluster health",
		SchemaVersion: 1,
		Ok: finalOk,
		Severity: uint8(finalSeverity),
	}
}

func (c controller) handleHealthcheck(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		c.jsonHandler(w, r)
	} else {
		c.htmlHandler(w, r)
	}
}

func (c controller) handleGoodToGo(w http.ResponseWriter, r *http.Request) {
	categories := parseCategories(r.URL)
	healthResults := c.combineHealthResultsFor(categories)
	if !healthResults.Ok {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func (c controller) jsonHandler(w http.ResponseWriter, r *http.Request) {
	categories := parseCategories(r.URL)
	health := c.combineHealthResultsFor(categories)
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
	health := c.combineHealthResultsFor(categories)
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

func getIntersectingCategories(s []string, c map[string]Category) []string {
	result := make([]string, 0)
	for _, a := range s {
		if _, ok := c[a]; ok {
			result = append(result, a)
		}
	}
	return result
}

//returns true, only if all categoryNames are considered resilient.
func isResilient(categoryNames []string, allCategories map[string]Category) bool {
	for _, c := range categoryNames {
		if !allCategories[c].IsResilient {
			return false
		}
	}
	return true
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
