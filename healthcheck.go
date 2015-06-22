package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"git.svc.ft.com/scm/gl/fthealth.git"
	"github.com/gorilla/http/client"
	"golang.org/x/net/proxy"
	"io/ioutil"
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

type ServiceHealthChecker interface {
	Check(Service) error
}

type CocoServiceHealthChecker struct {
	dialer proxy.Dialer
}

func NewCocoServiceHealthChecker(dialer proxy.Dialer) *CocoServiceHealthChecker {
	return &CocoServiceHealthChecker{dialer: dialer}
}

func (c *CocoServiceHealthChecker) Check(service Service) error {
	conn, err := c.dialer.Dial("tcp", service.Host)
	if err != nil {
		return errors.New("Error performing healthcheck: " + err.Error())
	}

	conn.SetDeadline(time.Now().Add(1 * time.Second))
	defer conn.Close()

	http := client.NewClient(conn)
	headers := []client.Header{client.Header{Key: "Host", Value: service.Name}}
	req := &client.Request{Version: client.HTTP_1_1, Method: "GET", Path: "/__health", Headers: headers}

	err = http.WriteRequest(req)
	if err != nil {
		return errors.New("Error performing healthcheck: " + err.Error())
	}

	resp, err := http.ReadResponse()
	if err != nil {
		return errors.New("Error reading healthcheck response: " + err.Error())
	}

	if resp.Status.Code != 200 {
		return fmt.Errorf("Healthcheck endpoint returned non-200 status (%v %v)", resp.Status.Code, resp.Status.Reason)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.New("Error reading healthcheck response: " + err.Error())
	}

	health := &healthcheckResponse{}
	err = json.Unmarshal(body, &health)
	if err != nil {
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
		Severity: 1,
		Checker:  func() error { return checker.Check(service) },
	}
}

func CocoAggregateHealthHandler(registry ServiceRegistry, checker ServiceHealthChecker) func(w http.ResponseWriter, r *http.Request) {
	checks := []fthealth.Check{}
	for _, service := range registry.Services() {
		checks = append(checks, NewCocoServiceHealthCheck(service, checker))
	}

	return fthealth.Handler("Coco Aggregate Healthcheck", "Checks the health of all deployed services", checks...)
}
