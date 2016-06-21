package main

import fthealth "github.com/Financial-Times/go-fthealth/v1a"

type CachedHealth struct {
	toWriteToCache  chan fthealth.HealthResult
	toReadFromCache chan fthealth.HealthResult
	terminate       chan bool
}

func NewCachedHealth() *CachedHealth {
	a := make(chan fthealth.HealthResult)
	b := make(chan fthealth.HealthResult)
	terminate := make(chan bool)
	return &CachedHealth{a, b, terminate}
}

func (c *CachedHealth) maintainLatest() {
	var aux fthealth.HealthResult
	for {
		select {
		case aux = <-c.toWriteToCache:
		case c.toReadFromCache <- aux:
		}
	}
}
