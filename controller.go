package main

import (
	"encoding/json"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"html/template"
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

type ServiceHealthCheck struct {
	Name        string
	IsHealthy   bool
	IsCritical  bool
	IsAcked     bool
	LastUpdated string
	Ack         string
}

type AggregateHealthCheck struct {
	Environment     string
	ValidCategories string
	IsHealthy       bool
	IsCritical      bool
	HealthChecks    []ServiceHealthCheck
	Ack             Acknowledge
}

type Acknowledge struct {
	IsAcked bool
	Count   int
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

	health := fthealth.HealthResult{
		Checks:        checkResults,
		Description:   desc,
		Name:          *c.environment + " cluster health",
		SchemaVersion: 1,
		Ok:            finalOk,
		Severity:      finalSeverity,
	}
	sort.Sort(ByName(health.Checks))

	return health, matchingCategories
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
		checkResult.Ack = healthResult.Checks[0].Ack
		checkResults = append(checkResults, checkResult)
	}
	return checkResults
}

func (c Controller) runChecksFor(categories []string) []fthealth.CheckResult {
	var checks []fthealth.Check
	var acks map[string]string = make(map[string]string)
	for _, mService := range c.registry.measuredServices {
		if !containsAtLeastOneFrom(categories, mService.service.Categories) {
			continue
		}
		checks = append(checks, NewServiceHealthCheck(*mService.service, c.registry.checker))
		ack := c.registry.getAck(mService.service.ServiceKey)

		if ack != "" {
			acks[mService.service.Name] = ack
		}
	}
	healthChecks := fthealth.RunCheck("Forced check run", "", true, checks...).Checks
	var result []fthealth.CheckResult
	for _, ch := range healthChecks {
		if ack, found := acks[ch.Name]; found {
			ch.Ack = ack
		}
		result = append(result, ch)
	}
	return result
}

func (c Controller) computeResilientHealthResult(checkResults []fthealth.CheckResult) (bool, uint8) {
	finalOk := true
	var finalSeverity uint8 = 2
	serviceCheckResults := make(map[string]fthealth.CheckResult)
	severities := make(map[string]uint8)
	for _, result := range checkResults {
		serviceGroupName := result.Name[0:strings.LastIndex(result.Name, "-")]
		if _, isPresent := severities[serviceGroupName]; !isPresent || result.Ok {
			severities[serviceGroupName] = result.Severity
			serviceCheckResults[serviceGroupName] = result
		}
	}
	for serviceGroupName, result := range serviceCheckResults {
		if !result.Ok && result.Ack == "" {
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
		if !result.Ok && result.Ack == "" {
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

	// check if any of the categories are disabled
	enabled := c.catEnabled(categories)
	if !enabled {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	healthResults, validCategories := c.buildHealthResultFor(categories, useCache(r.URL))
	if len(validCategories) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !healthResults.Ok {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func (c Controller) catEnabled(validCats []string) bool {
	for _, cat := range c.registry.categories {
		for _, validCat := range validCats {
			if validCat == cat.Name && !cat.Enabled {
				return false
			}
		}
	}
	return true
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

	mainTemplate, err := template.ParseFiles("main.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Couldn't open template file for html response"))
		return
	}

	var healthChecks []ServiceHealthCheck
	var aggAck Acknowledge
	for _, check := range health.Checks {
		hc := ServiceHealthCheck{
			Name:        check.Name,
			IsHealthy:   check.Ok,
			IsCritical:  check.Severity == 1,
			LastUpdated: check.LastUpdated.Format(timeLayout),
		}
		if check.Ack != "" {
			hc.IsAcked = true
			hc.Ack = check.Ack
			aggAck.IsAcked = true
			aggAck.Count++
		}
		healthChecks = append(healthChecks,
			hc)
	}

	param := &AggregateHealthCheck{
		Environment:     *c.environment,
		ValidCategories: strings.Join(validCategories, ", "),
		IsHealthy:       health.Ok,
		IsCritical:      health.Severity == 1,
		HealthChecks:    healthChecks,
		Ack:             aggAck,
	}
	if err = mainTemplate.Execute(w, param); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Couldn't render template file for html response"))
		return
	}

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
