package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	fthealth "github.com/Financial-Times/go-fthealth/v1a"
)

const timeLayout = "15:04:05 MST"
const serviceInstanceDelimiter = '@'

var serverInstanceRegex = regexp.MustCompile("-\\d+$")
var defaultCategories = []string{"default"}

type Controller struct {
	registry    ServiceRegistry
	environment *string
}

type ServiceHealthCheck struct {
	FleetName   string
	EtcdName    string
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

func NewController(registry ServiceRegistry, environment *string) *Controller {
	return &Controller{registry, environment}
}

func (c Controller) buildHealthResultFor(categories []string, useCache bool) (fthealth.HealthResult, []string, []string) {
	var checkResults []fthealth.CheckResult
	var categorisedResults map[string][]fthealth.CheckResult
	unhealthyCategories := []string{}
	matchingCategories := c.registry.MatchingCategories(categories)
	desc := "Health of the whole cluster of the moment served directly."
	if useCache {
		checkResults, categorisedResults = c.collectChecksFromCachesFor(categories)
		desc = "Health of the whole cluster served from cache."
	} else {
		checkResults, categorisedResults = c.runChecksFor(categories)
	}
	var finalOk bool
	var finalSeverity uint8
	if c.registry.AreResilient(matchingCategories) {
		finalOk, finalSeverity = c.computeResilientHealthResult(checkResults)
	} else {
		finalOk, finalSeverity = c.computeNonResilientHealthResult(checkResults)
	}

	for category, results := range categorisedResults {
		var catOk bool
		if c.registry.AreResilient([]string{category}) {
			catOk, _ = c.computeResilientHealthResult(results)
		} else {
			catOk, _ = c.computeNonResilientHealthResult(results)
		}

		if !catOk {
			unhealthyServices := []string{}
			for _, result := range results {
				if !result.Ok {
					unhealthyServices = append(unhealthyServices, result.Name)
				}
			}
			warnLogger.Printf("In category %v, the following services are unhealthy: %v", category, strings.Join(unhealthyServices, ","))
			unhealthyCategories = append(unhealthyCategories, category)
		}
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

	return health, matchingCategories, unhealthyCategories
}

func (c Controller) collectChecksFromCachesFor(categories []string) ([]fthealth.CheckResult, map[string][]fthealth.CheckResult) {
	var checkResults []fthealth.CheckResult

	categorisedResults := make(map[string][]fthealth.CheckResult)
	for _, c := range categories {
		categorisedResults[c] = []fthealth.CheckResult{}
	}

	for _, mService := range c.registry.MeasuredServices() {
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
		for _, category := range mService.service.Categories {
			if categoryResults, exists := categorisedResults[category]; exists {
				categorisedResults[category] = append(categoryResults, checkResult)
			}
		}
	}
	return checkResults, categorisedResults
}

func (c Controller) runChecksFor(categories []string) ([]fthealth.CheckResult, map[string][]fthealth.CheckResult) {
	var checks []fthealth.Check

	categorisedChecks := make(map[string][]*fthealth.Check)
	categorisedResults := make(map[string][]fthealth.CheckResult)
	for _, c := range categories {
		categorisedChecks[c] = []*fthealth.Check{}
		categorisedResults[c] = []fthealth.CheckResult{}
	}

	var acks map[string]string = make(map[string]string)
	for _, mService := range c.registry.MeasuredServices() {
		if !containsAtLeastOneFrom(categories, mService.service.Categories) {
			continue
		}
		check := NewServiceHealthCheck(*mService.service, c.registry.Checker())
		checks = append(checks, check)
		for _, category := range mService.service.Categories {
			if categoryChecks, exists := categorisedChecks[category]; exists {
				categorisedChecks[category] = append(categoryChecks, &check)
			}
		}

		ack := c.registry.GetAck(mService.service.ServiceKey)

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

		for category, checks := range categorisedChecks {
			for _, check := range checks {
				if check.Name == ch.Name {
					categorisedResults[category] = append(categorisedResults[category], ch)
				}
			}
		}
	}
	updateCachedAndBufferedHealth(c.registry, healthChecks)

	return result, categorisedResults
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

	healthResults, validCategories, unhealthyCategories := c.buildHealthResultFor(categories, useCache(r.URL))
	if len(validCategories) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !healthResults.Ok {
		for _, cat := range unhealthyCategories {
			c.registry.DisableCategoryIfSticky(cat)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func (c Controller) catEnabled(validCats []string) bool {
	for _, cat := range c.registry.Categories() {
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
	healthResults, validCategories, _ := c.buildHealthResultFor(categories, useCache(r.URL))
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
	health, validCategories, _ := c.buildHealthResultFor(categories, useCache(r.URL))
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
			EtcdName:    check.Name,
			FleetName:   formatServiceName(check.Name),
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

func formatServiceName(name string) string {
	loc := serverInstanceRegex.FindStringIndex(name)
	if loc == nil {
		return name
	}
	nameAsRunes := []rune(name)
	nameAsRunes[loc[0]] = serviceInstanceDelimiter
	return string(nameAsRunes)
}

func updateCachedAndBufferedHealth(registry ServiceRegistry, healthChecks []fthealth.CheckResult) {
	healthResults := splitChecksInHealthResults(healthChecks)
	measuredServices := registry.MeasuredServices()
	for _, healthResult := range healthResults {
		if mService, found := measuredServices[healthResult.Checks[0].Name]; found {
			registry.UpdateCachedAndBufferedHealth(&mService, &healthResult)
		}
	}
}

func splitChecksInHealthResults(healthChecks []fthealth.CheckResult) []fthealth.HealthResult {
	healthResults := make([]fthealth.HealthResult, len(healthChecks))
	for i, check := range healthChecks {
		healthResults[i].Name = check.Name
		healthResults[i].SchemaVersion = 1
		healthResults[i].Checks = make([]fthealth.CheckResult, 1)
		healthResults[i].Checks[0] = check
		healthResults[i].Ok = check.Ok
		healthResults[i].Severity = check.Severity
	}
	return healthResults
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
