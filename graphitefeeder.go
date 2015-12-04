package main

import (
	"net"
	"strconv"
	"fmt"
	"time"
	"log"
	"strings"
)

type GraphiteFeeder struct {
	host        string
	port        int
	environment string
	connection  net.Conn
}

const metricFormat = "coco.health.%s.%s %d %d\n"

func NewGraphiteFeeder(host string, port int, environment string) *GraphiteFeeder {
	connection, _ := net.Dial("tcp", host+":"+strconv.Itoa(port))
	return &GraphiteFeeder{host, port, environment, connection}
}

func (graphite *GraphiteFeeder) maintainGraphiteFeed(bufferGraphite <-chan *HealthTimed, ticker *time.Ticker) {
	for range ticker.C {
		results := drain(bufferGraphite)
		graphite.send(results)
	}
}

func (graphite *GraphiteFeeder) send(results []*HealthTimed) {
	log.Printf("INFO graphite metric: Sending batch with %v result sets.", len(results))
	for _, result := range results {
		checks := result.healthResult.Checks
		time := result.time
		log.Printf("INFO graphite metric: Sending a result set of %v services for time point %v.", len(checks), time)
		for _, check := range checks {
			name := strings.Replace(check.Name, ".", "-", -1)
			_, err := fmt.Fprintf(graphite.connection, metricFormat, graphite.environment, name, inverseBoolToInt(check.Ok), time.Unix())
			if err != nil {
				log.Printf("WARN Error sending stuff to graphite: [%v]", err.Error())
			}
		}
	}
}

func drain(ch <-chan *HealthTimed) []*HealthTimed {
	var results []*HealthTimed
	for {
		select {
		case p := <-ch:
			results = append(results, p)
		default:
			return results
		}
	}
}

func inverseBoolToInt(b bool) int {
	if b {
		return 0
	}
	return 1
}
