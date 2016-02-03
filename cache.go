package main

import (
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"time"
)

type Cache struct {
	service      *Service
	latestResult <-chan HealthTimed
	latestWrite  chan<- HealthTimed
	timer       time.Ticker
}

func NewCache(service *Service) *Cache {
	latestRead := make(chan fthealth.HealthResult)
	latestWrite := make(chan fthealth.HealthResult)

	minSeconds := defaultDuration.Seconds()
	minDuration := defaultDuration
	for _, category := range service.Categories {
		if category.Period.Seconds() < minSeconds {
			minSeconds = category.Period.Seconds()
			minDuration = category.Period
		}
	}
	timer := time.NewTimer(minDuration)

	return &Cache{service, latestRead, latestWrite, timer}
}

func (c Cache) maintainLatest(latestRead chan<- fthealth.HealthResult, latestWrite <-chan fthealth.HealthResult) {
	var latest fthealth.HealthResult
	for {
		select {
		case latest = <-latestWrite:
		case latestRead <- latest:
		}
	}
}
