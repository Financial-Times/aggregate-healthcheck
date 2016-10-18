package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRegistry struct {
	mock.Mock
}

func (r *MockRegistry) Categories() map[string]Category {
	args := r.Called()
	return args.Get(0).(map[string]Category)
}

func (r *MockRegistry) MatchingCategories(categories []string) []string {
	args := r.Called(categories)
	return args.Get(0).([]string)
}

func (r *MockRegistry) AreResilient(categories []string) bool {
	args := r.Called(categories)
	return args.Bool(0)
}

func (r *MockRegistry) MeasuredServices() map[string]MeasuredService {
	args := r.Called()
	return args.Get(0).(map[string]MeasuredService)
}

func (r *MockRegistry) Checker() HealthChecker {
	args := r.Called()
	return args.Get(0).(HealthChecker)
}

func (r *MockRegistry) GetAck(serviceKey string) string {
	args := r.Called(serviceKey)
	return args.String(0)
}

func (r *MockRegistry) DisableCategoryIfSticky(category string) {
	r.Called(category)
}

func (r *MockRegistry) UpdateCachedAndBufferedHealth(service *MeasuredService, result *fthealth.HealthResult) {
	r.Called(service, result)
}

type MockHealthChecker struct {
	mock.Mock
}

func (c *MockHealthChecker) Check(service Service) (string, error) {
	args := c.Called(service)
	return args.String(0), args.Error(1)
}

func (c *MockHealthChecker) IsHighSeverity(service string) bool {
	args := c.Called(service)
	return args.Bool(0)
}

func mockCategories(r *MockRegistry, enabled []string, disabled []string) {
	categories := make(map[string]Category)
	for _, cat := range enabled {
		categories[cat] = Category{Name: cat, Enabled: true}
	}
	for _, cat := range disabled {
		categories[cat] = Category{Name: cat, Enabled: false}
	}

	r.On("Categories").Return(categories)
}

func mockServices(r *MockRegistry, healthyServicesAndCategories map[string][]string, unhealthyServicesAndCategories map[string][]string) {
	any := func(x interface{}) bool { return true }

	c := new(MockHealthChecker)
	c.On("IsHighSeverity", mock.MatchedBy(any)).Return(false)
	r.On("Checker").Return(c)

	measuredServices := make(map[string]MeasuredService)

	for service, categories := range healthyServicesAndCategories {
		key := strings.ToLower(strings.Replace(service, " ", "-", -1))
		s := Service{Name: service, ServiceKey: key, Categories: categories}
		measuredService := MeasuredService{service: &s}
		measuredServices[service] = measuredService
		c.On("Check", s).Return("ok", nil)
	}

	for service, categories := range unhealthyServicesAndCategories {
		key := strings.ToLower(strings.Replace(service, " ", "-", -1))
		s := Service{Name: service, ServiceKey: key, Categories: categories}
		measuredService := MeasuredService{service: &s}
		measuredServices[service] = measuredService
		c.On("Check", s).Return("nok", errors.New("Service "+service+" is unhealthy"))
	}

	r.On("MeasuredServices").Return(measuredServices)
	r.On("GetAck", mock.MatchedBy(any)).Return("")
	r.On("UpdateCachedAndBufferedHealth", mock.MatchedBy(any), mock.MatchedBy(any)).Return()
}

func TestCatEnabledWhenAllEnabled(t *testing.T) {
	registry := new(MockRegistry)
	mockCategories(registry, []string{"foo", "bar"}, []string{})

	env := "test"
	controller := NewController(registry, &env)

	b := controller.catEnabled([]string{"foo", "bar"})
	assert.True(t, b, "categories should be enabled")
}

func TestCatEnabledFalseWhenNotAllEnabled(t *testing.T) {
	registry := new(MockRegistry)
	mockCategories(registry, []string{"foo"}, []string{"bar"})

	env := "test"
	controller := NewController(registry, &env)

	b := controller.catEnabled([]string{"foo", "bar"})
	assert.False(t, b, "categories should not be enabled")
}

func TestCatEnabledWhenCategoryUnknown(t *testing.T) {
	registry := new(MockRegistry)
	mockCategories(registry, []string{"foo", "bar"}, []string{})

	env := "test"
	controller := NewController(registry, &env)

	b := controller.catEnabled([]string{"foo", "bar", "baz"})
	assert.True(t, b, "categories should be enabled")
}

func TestRunChecksFor(t *testing.T) {
	registry := new(MockRegistry)

	healthy := make(map[string][]string)
	healthy["Test Service"] = []string{"foo"}

	unhealthy := make(map[string][]string)

	mockServices(registry, healthy, unhealthy)

	env := "test"
	controller := NewController(registry, &env)

	results, categorisedResults := controller.runChecksFor([]string{"foo"})
	assert.Len(t, results, 1, "results")
	actual := results[0]
	assert.True(t, actual.Ok, "service should be healthy")
	assert.NotEqual(t, actual.Severity, 1, "service should not be high severity")

	assert.Len(t, categorisedResults, 1, "categorised results")
	assert.Len(t, categorisedResults["foo"], 1, "results in foo category")
	assert.Equal(t, actual, categorisedResults["foo"][0], "categorised result")
}

func TestRunChecksForUnhealthy(t *testing.T) {
	registry := new(MockRegistry)

	healthy := make(map[string][]string)

	unhealthy := make(map[string][]string)
	unhealthy["Test Service"] = []string{"foo"}

	mockServices(registry, healthy, unhealthy)

	env := "test"
	controller := NewController(registry, &env)

	results, categorisedResults := controller.runChecksFor([]string{"foo"})
	assert.Len(t, results, 1, "results")
	actual := results[0]
	assert.False(t, actual.Ok, "service should not be healthy")
	assert.NotEqual(t, actual.Severity, 1, "service should not be high severity")
	assert.Equal(t, "Service Test Service is unhealthy", actual.Output, "healthcheck output")
	assert.Len(t, categorisedResults, 1, "categorised results")
	assert.Len(t, categorisedResults["foo"], 1, "results in foo category")
	assert.Equal(t, actual, categorisedResults["foo"][0], "categorised result")
}

func TestHandleGtgOk(t *testing.T) {
	initLogs(os.Stdout, os.Stdout, os.Stderr)
	any := func(x interface{}) bool { return true }

	registry := new(MockRegistry)

	healthy := make(map[string][]string)
	healthy["Test Service"] = []string{"foo"}

	unhealthy := make(map[string][]string)

	mockServices(registry, healthy, unhealthy)

	env := "test"
	controller := NewController(registry, &env)

	mockCategories(registry, []string{"foo"}, []string{})
	registry.On("MatchingCategories", []string{"foo"}).Return([]string{"Test Service"})
	registry.On("AreResilient", mock.MatchedBy(any)).Return(false)

	req, _ := http.NewRequest("GET", "http://www.example.com/__gtg?categories=foo&cache=false", nil)
	w := httptest.NewRecorder()

	controller.handleGoodToGo(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "HTTP status")
	registry.AssertExpectations(t)
}

func TestHandleGtgForDisabledCategory(t *testing.T) {
	registry := new(MockRegistry)
	mockCategories(registry, []string{"foo"}, []string{"bar"})

	env := "test"
	controller := NewController(registry, &env)

	req, _ := http.NewRequest("GET", "http://www.example.com/__gtg?categories=bar", nil)
	w := httptest.NewRecorder()

	controller.handleGoodToGo(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "HTTP status")
}

func TestHandleGtgUnhealthySetsDisabledIfSticky(t *testing.T) {
	initLogs(os.Stdout, os.Stdout, os.Stderr)
	any := func(x interface{}) bool { return true }

	registry := new(MockRegistry)

	healthy := make(map[string][]string)

	unhealthy := make(map[string][]string)
	unhealthy["Test Service"] = []string{"foo"}

	mockServices(registry, healthy, unhealthy)

	env := "test"
	controller := NewController(registry, &env)

	mockCategories(registry, []string{"foo"}, []string{})
	registry.On("MatchingCategories", []string{"foo"}).Return([]string{"Test Service"})
	registry.On("AreResilient", mock.MatchedBy(any)).Return(false)
	registry.On("DisableCategoryIfSticky", "foo").Return()

	req, _ := http.NewRequest("GET", "http://www.example.com/__gtg?categories=foo&cache=false", nil)
	w := httptest.NewRecorder()

	controller.handleGoodToGo(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "HTTP status")
	registry.AssertExpectations(t)
}

func TestHandleGtgUnhealthySetsUnhealthyCategoriesOnlyDisabled(t *testing.T) {
	initLogs(os.Stdout, os.Stdout, os.Stderr)
	any := func(x interface{}) bool { return true }

	registry := new(MockRegistry)

	healthy := make(map[string][]string)
	healthy["Test Service 1"] = []string{"foo"}

	unhealthy := make(map[string][]string)
	unhealthy["Test Service 2"] = []string{"bar"}

	mockServices(registry, healthy, unhealthy)

	env := "test"
	controller := NewController(registry, &env)

	mockCategories(registry, []string{"foo", "bar"}, []string{})
	registry.On("MatchingCategories", []string{"foo", "bar"}).Return([]string{"Test Service 1", "Test Service 2"})
	registry.On("AreResilient", mock.MatchedBy(any)).Return(false)
	registry.On("DisableCategoryIfSticky", "bar").Return() // only expect to be called for category "bar"

	req, _ := http.NewRequest("GET", "http://www.example.com/__gtg?categories=foo,bar&cache=false", nil)
	w := httptest.NewRecorder()

	controller.handleGoodToGo(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "HTTP status")
	registry.AssertExpectations(t)
}
