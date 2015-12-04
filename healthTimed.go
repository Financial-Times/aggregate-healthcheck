package main

import (
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"time"
)

type HealthTimed struct {
	healthResult fthealth.HealthResult
	time         time.Time
}

func NewHealthTimed(healthResult fthealth.HealthResult, time time.Time) *HealthTimed {
	return &HealthTimed{healthResult, time}
}
