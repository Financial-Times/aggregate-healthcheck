package main

import fthealth "github.com/Financial-Times/go-fthealth/v1a"

type cache struct {
	latestResult   <-chan fthealth.HealthResult
	latestWrite    chan<- fthealth.HealthResult
}

func NewCache() *cache {
	latestRead := make(chan fthealth.HealthResult)
	latestWrite := make(chan fthealth.HealthResult)
	go maintainCache(latestRead, latestWrite)
	return &cache{latestRead}
}



func maintainCache(latestRead chan<- fthealth.HealthResult, latestWrite <-chan fthealth.HealthResult) {
	var latest fthealth.HealthResult
	for {
		select {
		case latest = <-latestWrite:
		case latestRead <- latest:
		}
	}
}
