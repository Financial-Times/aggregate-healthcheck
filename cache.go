package main

import (
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
)

type CachedHealth struct {
	latestResult <-chan TimedHealth
	latestWrite  chan<- TimedHealth
}

func NewCachedHealth() *CachedHealth {
	latestRead := make(chan TimedHealth)
	latestWrite := make(chan TimedHealth)
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
