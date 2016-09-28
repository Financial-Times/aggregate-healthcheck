package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"net/http"
	"time"
	"golang.org/x/net/proxy"
)

var sos = []string{"publish-availability-monitor", "synthetic-list-publish-monitor", "some-other-service"}


func getClient() http.Client {
	transport := &http.Transport{
		Dial: proxy.Direct.Dial,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConnsPerHost:   100,
	}

	return http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}
}

func TestHTTPHealthChecker_IsHighSeverity(t *testing.T) {

	assert := assert.New(t)
	httpClient := getClient()

	testChecker := NewHTTPHealthChecker(&httpClient, sos)
	assert.False(testChecker.IsHighSeverity("document-store-api@1.service"));
	assert.True(testChecker.IsHighSeverity("publish-availability-monitor@1.service"));

}


