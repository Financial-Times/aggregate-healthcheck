package main

import (
	"time"
)

func (registry CocoServiceRegistry) scheduleCheck(mService *MeasuredService, timer *time.Timer) {
	// wait
	<- timer.C

	// check
	timedHealth := registry.checker.checkHealthSimple(mService.service)

	// write to cache
	mService.cachedHealth.latestWrite <- timedHealth

	// write to graphite buffer
	select {
	case mService.bufferedHealths.buffer <- timedHealth:
	default:
	}

	// schedule next check
	service := registry.services[mService.service.Name]
	if service == nil {
		return
	}
	waitDuration := findShortestPeriod(service)
	go registry.scheduleCheck(mService, time.NewTimer(waitDuration))
}
