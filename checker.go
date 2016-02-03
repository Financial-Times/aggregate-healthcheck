package main

import (
	"encoding/json"
	"errors"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
"time"
)

type healthcheckResponse struct {
	Name   string
	Checks []struct {
		Name string
		OK   bool
	}
}

type HealthChecker interface {
	Check(Service) (string, error)
}

type CocoHealthChecker struct {
	client *http.Client
}

func NewCocoServiceHealthChecker(client *http.Client) *CocoHealthChecker {
	return &CocoHealthChecker{client: client}
}

func (c *CocoHealthChecker) Check(service Service) (string, error) {
	log.Printf("INFO Sending client request: http://%s%s", service.Host, service.Path)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s%s", service.Host, service.Path), nil)
	if err != nil {
		return "", errors.New("Error constructing healthcheck request: " + err.Error())
	}

	req.Host = service.Name

	resp, err := c.client.Do(req)
	if err != nil {
		return "", errors.New("Error performing healthcheck: " + err.Error())
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Healthcheck endpoint returned non-200 status (%v)", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("Error reading healthcheck response: " + err.Error())
	}

	health := &healthcheckResponse{}
	if err := json.Unmarshal(body, &health); err != nil {
		return "", errors.New("Error parsing healthcheck response: " + err.Error())
	}

	failed := []string{}
	for _, check := range health.Checks {
		if check.OK != true {
			failed = append(failed, check.Name)
		}
	}

	if count := len(failed); count > 0 {
		return "", fmt.Errorf("%d healthchecks failing (%v)", count, strings.Join(failed, ", "))
	}

	return "", nil
}

func (checker HealthChecker) checkHealthSimple(service *Service) TimedHealth {
	start := time.Now()
	checks := []fthealth.Check{NewCocoServiceHealthCheck(service, checker)}
	health := fthealth.RunCheck("Coco Aggregate Healthcheck", "Checks the health of all deployed services", true, checks...)
	now := time.Now()
	healthTimed := NewHealthTimed(health, now)
	log.Printf("DEBUG - got new health results in %v\n", now.Sub(start))
	return healthTimed
}

func NewCocoServiceHealthCheck(service Service, checker HealthChecker) fthealth.Check {
	//horrible hack...but we really need this for the soft go-live
	var severity uint8 = 2
	if strings.Contains(service.Name, "synthetic-image-publication-monitor") {
		severity = 1
	}
	return fthealth.Check{
		BusinessImpact:   "On its own this failure does not have a business impact but it represents a degradation of the cluster health.",
		Name:             service.Name,
		PanicGuide:       "https://sites.google.com/a/ft.com/technology/systems/dynamic-semantic-publishing/coco/runbook",
		Severity:         severity,
		TechnicalSummary: "The service is not healthy. Please check the panic guide.",
		Checker:          func() (string, error) { return checker.Check(service) },
	}
}
