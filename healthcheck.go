package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Financial-Times/go-fthealth"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

type healthcheckResponse struct {
	Name   string
	Checks []struct {
		Name string
		OK   bool
	}
}

type ServiceHealthChecker interface {
	Check(Service) error
}

type CocoServiceHealthChecker struct {
	client *http.Client
}

func NewCocoServiceHealthChecker(client *http.Client) *CocoServiceHealthChecker {
	return &CocoServiceHealthChecker{client: client}
}

func (c *CocoServiceHealthChecker) Check(service Service) error {
	log.Printf("INFO Sending client request: http://%s%s", service.Host, service.Healthcheck)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s%s", service.Host, service.Healthcheck), nil)
	if err != nil {
		return errors.New("Error constructing healthcheck request: " + err.Error())
	}

	req.Host = service.Name

	resp, err := c.client.Do(req)
	if err != nil {
		return errors.New("Error performing healthcheck: " + err.Error())
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Healthcheck endpoint returned non-200 status (%v)", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.New("Error reading healthcheck response: " + err.Error())
	}

	health := &healthcheckResponse{}
	if err := json.Unmarshal(body, &health); err != nil {
		return errors.New("Error parsing healthcheck response: " + err.Error())
	}

	failed := []string{}
	for _, check := range health.Checks {
		if check.OK != true {
			failed = append(failed, check.Name)
		}
	}

	if count := len(failed); count > 0 {
		return fmt.Errorf("%d healthchecks failing (%v)", count, strings.Join(failed, ", "))
	}

	return nil
}

func NewCocoServiceHealthCheck(service Service, checker ServiceHealthChecker) fthealth.Check {
	return fthealth.Check{
		Name:     service.Name,
		Severity: 2,
		Checker:  func() error { return checker.Check(service) },
	}
}
