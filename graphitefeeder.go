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
	connection, _ := connect(host, port)
	return &GraphiteFeeder{host, port, environment, connection}
}

func (graphite *GraphiteFeeder) maintainGraphiteFeed(bufferGraphite chan *HealthTimed, ticker *time.Ticker) {
	for range ticker.C {
		err := graphite.sendBuffer(bufferGraphite)
		if (err != nil) {
			if (strings.Contains(err.Error(), "broken pipe")) {
				graphite.connection.Close()
				graphite.reconnect()
			}
		}
	}
}

func (graphite *GraphiteFeeder) sendBuffer(bufferGraphite chan *HealthTimed) error {
	for {
		select {
		case healthTimed := <-bufferGraphite:
			err := graphite.sendOne(healthTimed)
			if (err != nil) {
				addBack(bufferGraphite, healthTimed)
				return err
			}
		default:
			return nil
		}
	}
}

func (graphite *GraphiteFeeder) sendOne(result *HealthTimed) error {
	checks := result.healthResult.Checks
	time := result.time
	log.Printf("INFO graphite metric: Sending a result set of %v services for time point %v.", len(checks), time)
	for _, check := range checks {
		name := strings.Replace(check.Name, ".", "-", -1)
		_, err := fmt.Fprintf(graphite.connection, metricFormat, graphite.environment, name, inverseBoolToInt(check.Ok), time.Unix())
		if err != nil {
			log.Printf("WARN Error sending stuff to graphite: [%v]", err.Error())
			return err
		}
	}
	return nil
}

func addBack(bufferGraphite chan<- *HealthTimed, healthTimed *HealthTimed) {
	select {
	case bufferGraphite <- healthTimed:
	default:
	}
}

func connect(host string, port int) (net.Conn, error) {
	connection, err := net.Dial("tcp", host+":"+strconv.Itoa(port))
	return connection, err
}

func (graphite *GraphiteFeeder) reconnect() {
	log.Printf("INFO reconnecting to Graphite host.")
	connection, _ := connect(graphite.host, graphite.port)
	graphite.connection = connection
}

func inverseBoolToInt(b bool) int {
	if b {
		return 0
	}
	return 1
}
