package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

type GraphiteFeeder struct {
	host        string
	port        int
	environment string
	connection  net.Conn
}

const pilotLightFormat = "coco.health.%s.pilot-light 1 %d\n"
const metricFormat = "coco.health.%s.services.%s %d %d\n"

func NewGraphiteFeeder(host string, port int, environment string) *GraphiteFeeder {
	connection := tcpConnect(host, port)
	return &GraphiteFeeder{host, port, environment, connection}
}

func (graphite *GraphiteFeeder) maintainGraphiteFeed(bufferGraphite chan *HealthTimed, ticker *time.Ticker) {
	for range ticker.C {
		errPilot := graphite.sendPilotLight()
		errBuff := graphite.sendBuffer(bufferGraphite)
		if errPilot != nil || errBuff != nil {
			graphite.reconnect()
		}
	}
}

func (graphite *GraphiteFeeder) sendBuffer(bufferGraphite chan *HealthTimed) error {
	for {
		select {
		case healthTimed := <-bufferGraphite:
			err := graphite.sendOne(healthTimed)
			if err != nil {
				addBack(bufferGraphite, healthTimed)
				return err
			}
		default:
			return nil
		}
	}
}

func (graphite *GraphiteFeeder) sendPilotLight() error {
	if graphite.connection == nil {
		msg := "WARN Can't send pilot light, no Graphite connection."
		log.Printf(msg)
		return errors.New(msg)
	}
	log.Printf("DEBUG "+pilotLightFormat, graphite.environment, time.Now().Unix())
	_, err := fmt.Fprintf(graphite.connection, pilotLightFormat, graphite.environment, time.Now().Unix())
	if err != nil {
		log.Printf("WARN Error sending pilot-light signal to graphite: [%v]", err.Error())
		return err
	}
	return nil
}

func (graphite *GraphiteFeeder) sendOne(result *HealthTimed) error {
	if graphite.connection == nil {
		msg := "WARN Can't send results, no Graphite connection."
		log.Printf(msg)
		return errors.New(msg)
	}
	checks := result.healthResult.Checks
	time := result.time
	log.Printf("INFO graphite metric: Sending a result set of %v services for time point %v.", len(checks), time)
	for _, check := range checks {
		name := strings.Replace(check.Name, ".", "-", -1)
		log.Printf("DEBUG "+metricFormat, graphite.environment, name, inverseBoolToInt(check.Ok), time.Unix())
		_, err := fmt.Fprintf(graphite.connection, metricFormat, graphite.environment, name, inverseBoolToInt(check.Ok), time.Unix())
		if err != nil {
			log.Printf("WARN Error sending results to graphite: [%v]", err.Error())
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

func (graphite *GraphiteFeeder) reconnect() {
	log.Printf("INFO reconnecting to Graphite host.")
	if graphite.connection != nil {
		graphite.connection.Close()
	}
	graphite.connection = tcpConnect(graphite.host, graphite.port)
}

func tcpConnect(host string, port int) net.Conn {
	conn, err := net.Dial("tcp", host+":"+strconv.Itoa(port))
	if conn == nil || err != nil {
		log.Printf("WARN Error while creating TCP connection [%v]", err)
		return nil
	}
	tcpConn := conn.(*net.TCPConn)
	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(30 * time.Minute)
	return conn
}

func inverseBoolToInt(b bool) int {
	if b {
		return 0
	}
	return 1
}
