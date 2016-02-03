package main

import (
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"time"
)

type TimedHealth struct {
	healthResult fthealth.HealthResult
	time         time.Time
}

func NewHealthTimed(healthResult fthealth.HealthResult, time time.Time) *TimedHealth {
	return &TimedHealth{healthResult, time}
}
