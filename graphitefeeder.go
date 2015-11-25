package main

import (
	"net"
	"strconv"
	"fmt"
	"time"
	"log"
)

type GraphiteFeeder struct {
	host       string
	port       int
	connection net.Conn
}

const prefix = "content.health.%s"
const suffix = " %d %d\n"
const metricformat = prefix + "." + suffix

func NewGraphiteFeeder(host string, port int) *GraphiteFeeder {
	connection, _ := net.Dial("tcp", host+":"+strconv.Itoa(port))
	return &GraphiteFeeder{host, port, connection}
}

func (graphite *GraphiteFeeder) MaintainGraphiteFeed(latestGraphiteRead <-chan *HealthTimed, ticker *time.Ticker) {
	for range ticker.C {
		results := drain(latestGraphiteRead)
		graphite.Send(results)
	}
}

func (graphite *GraphiteFeeder) Send(results []*HealthTimed) {
	log.Printf("DEBUG graphite metric: Sending batch with %v result sets.", len(results))
	for _, result := range results {
		log.Printf("DEBUG graphite metric: Sending a result set")
		time := result.time
		for _, check := range result.healthResult.Checks {
//			_, err := fmt.Fprintf(graphite.connection, metricformat, check.Name, booltoint(check.Ok), time.Unix())
			msg := fmt.Sprintf(metricformat, check.Name, booltoint(check.Ok), time.Unix())
			log.Printf("DEBUG graphite metric: " + msg)
//			if err != nil {
//				log.Printf("WARN Error sending stuff to graphite: [%v]", err.Error())
//			}
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

func booltoint(b bool) int {
	if b {
		return 1
	}
	return 0
}
