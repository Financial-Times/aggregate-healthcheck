package main

import (
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
)

type CachedHealth struct {
	latestResult <-chan fthealth.HealthResult
	latestWrite  chan<- fthealth.HealthResult
	terminate    chan bool
}

func NewCachedHealth() *CachedHealth {
	latestRead := make(<-chan fthealth.HealthResult)
	latestWrite := make(chan<- fthealth.HealthResult)
	terminate := make(chan bool)
	return &CachedHealth{latestRead, latestWrite, terminate}
}

func (c *CachedHealth) maintainLatest() {
	var latest fthealth.HealthResult
	for {
		select {
		case latest = <-c.latestResult:
		case c.latestWrite <- latest:
		}
	}
}
