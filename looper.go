package main

import (
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"time"
	"log"
)

type Looper struct {
	registry       *CocoServiceRegistry
	services       *map[string]Service
	caches         *map[string]Cache
	stacks         *map[string]GraphiteStack
	checker        *HealthChecker
}

func (l Looper) runPeriodicChecks(cache Cache) {
	<- cache.timer.C

	service := l.registry.services[cache.service.Name]
	if service == nil {
		return
	}
	healthTimed := l.runCheck(service)
	cache.latestWrite <- healthTimed
	service.


	//for {
	//	select {
	//	case <-timer.C:
	//
	//
	//		latestWrite <- health
	//			select {
	//			case bufferGraphite <- NewHealthTimed(health, now):
	//			default:
	//			}
	//	case period = <-hch.hcPeriod:
	//		log.Printf("INFO updated health check period to %v\n", period)
	//	}
	//
	//	timer = time.NewTimer(period)
	//}
}

func (l Looper) runCheck(service *Service) HealthTimed {
	start := time.Now()
	checks := []fthealth.Check{NewCocoServiceHealthCheck(service, l.checker)}
	health := fthealth.RunCheck("Coco Aggregate Healthcheck", "Checks the health of all deployed services", true, checks...)
	now := time.Now()
	healthTimed := NewHealthTimed(health, now)
	log.Printf("DEBUG - got new health results in %v\n", now.Sub(start))
	return healthTimed
}