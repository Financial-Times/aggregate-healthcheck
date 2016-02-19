package main

import "time"

type EventLimiter struct {
	ticker       *time.Ticker
	trigger      chan bool
	wasTriggered chan bool
	timePassed   chan bool
}

func NewEventLimiter(f func()) *EventLimiter {
	ticker := time.NewTicker(60 * time.Second)
	trigger := make(chan bool, 1)
	wasTriggered := make(chan bool, 1)
	timePassed := make(chan bool, 1)
	limiter := &EventLimiter{ticker, trigger, wasTriggered, timePassed}
	limiter.maintainLimiter()
	go limiter.limit(f)
	return limiter
}

func (l EventLimiter) maintainLimiter() {
	go l.maintainTicker()
	go l.maintainTrigger()
}

func (l EventLimiter) maintainTicker() {
	for {
		<-l.ticker.C
		select {
		case l.timePassed <- true:
		default:
		}
	}
}

func (l EventLimiter) maintainTrigger() {
	for {
		<-l.trigger
		select {
		case l.wasTriggered <- true:
		default:
		}
	}
}

func (l EventLimiter) limit(f func()) {
	for {
		<-l.timePassed
		<-l.wasTriggered
		f()
	}
}
