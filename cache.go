package main

import (
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
)

type CachedHealth struct {
	latestResult <-chan fthealth.HealthResult
	latestWrite  chan<- fthealth.HealthResult
	terminate    chan<- bool
}

func NewCachedHealth() *CachedHealth {
	latestRead := make(chan fthealth.HealthResult)
	latestWrite := make(chan fthealth.HealthResult)
	return &CachedHealth{latestRead, latestWrite}
}

func (c CachedHealth) maintainLatest(latestRead chan<- fthealth.HealthResult, latestWrite <-chan fthealth.HealthResult) {
	var latest fthealth.HealthResult
	for {
		select {
		case latest = <-latestWrite:
		case latestRead <- latest:
		}
	}
}
