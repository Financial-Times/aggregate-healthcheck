package main

import (
	"errors"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	pilotLightFormat = "coco.health.%s.pilot-light 1 %d\n"
	metricFormat     = "coco.health.%s.services.%s %d %d\n"
)

type GraphiteFeeder struct {
	host        string
	port        int
	environment string
	connection  net.Conn
	ticker      *time.Ticker
	registry    ServiceRegistry
}

func NewGraphiteFeeder(host string, port int, environment string, registry ServiceRegistry) *GraphiteFeeder {
	connection := tcpConnect(host, port)
	ticker := time.NewTicker(60 * time.Second)
	return &GraphiteFeeder{host, port, environment, connection, ticker, registry}
}

type BufferedHealths struct {
	buffer chan fthealth.HealthResult
}

func NewBufferedHealths() *BufferedHealths {
	buffer := make(chan fthealth.HealthResult, 60)
	return &BufferedHealths{buffer}
}

func (g GraphiteFeeder) feed() {
	for range g.ticker.C {
		errPilot := g.sendPilotLight()
		errBuff := g.sendBuffers()
		if errPilot != nil {
			warnLogger.Printf("[%v]", errPilot.Error())
		}
		if errBuff != nil {
			warnLogger.Printf("[%v]", errBuff.Error())
		}
		if errPilot != nil || errBuff != nil {
			g.reconnect()
		}
	}
}

func (g GraphiteFeeder) sendBuffers() error {
	for _, mService := range g.registry.measuredServices() {
		err := g.sendOneBuffer(mService)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g GraphiteFeeder) sendPilotLight() error {
	if g.connection == nil {
		return errors.New("Can't send pilot light, no Graphite connection.")
	}
	_, err := fmt.Fprintf(g.connection, pilotLightFormat, g.environment, time.Now().Unix())
	if err != nil {
		warnLogger.Printf("Error sending pilot-light signal to graphite: [%v]", err.Error())
		return err
	}
	return nil
}

func (g GraphiteFeeder) sendOneBuffer(mService MeasuredService) error {
	for {
		select {
		case healthResult := <-mService.bufferedHealths.buffer:
			err := g.sendOne(healthResult)
			if err != nil {
				addBack(mService.bufferedHealths, healthResult)
				return err
			}
		default:
			return nil
		}
	}
}

func (g *GraphiteFeeder) sendOne(result fthealth.HealthResult) error {
	if g.connection == nil {
		return errors.New("Can't send results, no Graphite connection.")
	}
	check := result.Checks[0]
	name := strings.Replace(check.Name, ".", "-", -1)
	_, err := fmt.Fprintf(g.connection, metricFormat, g.environment, name, inverseBoolToInt(check.Ok), check.LastUpdated.Unix())
	if err != nil {
		warnLogger.Printf("Error sending results to graphite: [%v]", err.Error())
		return err
	}
	return nil
}

func addBack(bufferedHealths *BufferedHealths, healthResult fthealth.HealthResult) {
	select {
	case bufferedHealths.buffer <- healthResult:
	default:
	}
}

func (g *GraphiteFeeder) reconnect() {
	infoLogger.Println("Reconnecting to Graphite host.")
	if g.connection != nil {
		g.connection.Close()
	}
	g.connection = tcpConnect(g.host, g.port)
}

func tcpConnect(host string, port int) net.Conn {
	conn, err := net.Dial("tcp", host+":"+strconv.Itoa(port))
	if err != nil {
		warnLogger.Printf("Error while creating TCP connection [%v]", err)
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
