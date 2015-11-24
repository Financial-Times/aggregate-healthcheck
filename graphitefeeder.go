package main

import (
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
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

func (graphite GraphiteFeeder) Send(h fthealth.HealthResult) {
	for _, check := range h.Checks {
		//		_, err := fmt.Fprintf(graphite.connection, metricformat, check.Name, booltoint(check.Ok), time.Now().Unix())
		msg := fmt.Sprintf(metricformat, check.Name, booltoint(check.Ok), time.Now().Unix())
		log.Printf("DEBUG graphite metric: " + msg)
		//		if err != nil {
		//			log.Printf("WARN Error sending stuff to graphite: [%v]", err.Error())
		//		}
	}
}

func booltoint(b bool) int {
	if b {
		return 1
	}
	return 0
}
