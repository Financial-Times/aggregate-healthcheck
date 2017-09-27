package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	fthealth "github.com/Financial-Times/go-fthealth/v1a"
)

type healthcheckResponse struct {
	SystemCode string
	Name       string
	Checks     []check
}
type check struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Severity         uint8     `json:"severity"`
	BusinessImpact   string    `json:"businessImpact"`
	TechnicalSummary string    `json:"technicalSummary"`
	PanicGuide       string    `json:"panicGuide"`
	LastUpdated      string    `json:"lastUpdated"`
	CheckOutput      string    `json:"checkOutput"`
	CheckSystemCode  string    `json:"checkSystemCode"`
	OK               bool      `json:"ok"`
}

type HealthChecker interface {
	Check(Service) (string, error)
	IsHighSeverity(string) bool
	FetchHealthcheck(Service) (*healthcheckResponse, error)
}

type HTTPHealthChecker struct {
	client *http.Client
	sos    []string
}

func NewHTTPHealthChecker(client *http.Client, sos []string) *HTTPHealthChecker {
	return &HTTPHealthChecker{client: client, sos: sos}
}

func (c *HTTPHealthChecker) FetchHealthcheck(service Service) (*healthcheckResponse, error) {
	health := &healthcheckResponse{}

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s%s", service.Host, service.Path), nil)
	if err != nil {
		return health, errors.New("Error constructing healthcheck request: " + err.Error())
	}

	req.Host = service.Name

	resp, err := c.client.Do(req)
	if err != nil {
		return health, errors.New("Error performing healthcheck: " + err.Error())
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		return health, fmt.Errorf("Healthcheck endpoint returned non-200 status (%v)", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return health, errors.New("Error reading healthcheck response: " + err.Error())
	}

	if err := json.Unmarshal(body, &health); err != nil {
		return health, errors.New("Error parsing healthcheck response: " + err.Error())
	}
	return health, nil
}
func (c *HTTPHealthChecker) Check(service Service) (string, error) {
	health, err := c.FetchHealthcheck(service)
	if (err != nil) {
		return "", err
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

func NewServiceHealthCheck(service Service, checker HealthChecker) fthealth.Check {
	//horrible hack...but we really need this for the soft go-live
	var severity uint8 = 2

	if checker.IsHighSeverity(service.Name) {
		severity = 1
	}
	return fthealth.Check{
		BusinessImpact:   "On its own this failure does not have a business impact but it represents a degradation of the cluster health.",
		Name:             service.Name,
		PanicGuide:       "https://sites.google.com/a/ft.com/universal-publishing/ops-guides",
		Severity:         severity,
		TechnicalSummary: fmt.Sprintf("The service is not healthy. For detailed information, look at the service healthcheck:  https://%s-up.ft.com%s", service.Environment, service.Path),
		Checker: func() (string, error) {
			return checker.Check(service)
		},
	}
}

func (c *HTTPHealthChecker) IsHighSeverity(serviceName string) bool {
	for _, appName := range c.sos {
		if strings.Contains(serviceName, appName) {
			return true
		}
	}
	return false
}

func NewCheckFromSingularHealthResult(healthResult fthealth.HealthResult) fthealth.CheckResult {
	check := healthResult.Checks[0]
	return fthealth.CheckResult{
		BusinessImpact:   check.BusinessImpact,
		Output:           check.Output,
		LastUpdated:      check.LastUpdated,
		Name:             check.Name,
		Ok:               check.Ok,
		PanicGuide:       check.PanicGuide,
		Severity:         check.Severity,
		TechnicalSummary: check.TechnicalSummary,
	}
}
