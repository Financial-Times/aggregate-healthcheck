package main

import (
	"fmt"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"log"
	"strings"
)

type ServiceRegistry interface {
	Services() []Service
}

type Service struct {
	Name        string
	Host        string
	Healthcheck string
}

type CocoServiceRegistry struct {
	kapi        client.KeysAPI
	keyPrefix   string
	vulcandAddr string
}

func (r *CocoServiceRegistry) Services() []Service {
	resp, err := r.kapi.Get(context.Background(), r.keyPrefix, &client.GetOptions{Sort: true})
	if err != nil {
		panic(err.Error())
	}

	services := []Service{}
	for _, service := range resp.Node.Nodes {
		log.Printf("INFO Service: %s", service)
		resp, err := r.kapi.Get(context.Background(), service.Key, nil)
		if err == nil {
			name := strings.TrimPrefix(service.Key, r.keyPrefix)
			healthcheck := fmt.Sprintf("/health/%s%s", name, resp.Node.Value)
			log.Printf("INFO Healtheck: %s", healthcheck)
			services = append(services, Service{Name: name, Host: r.vulcandAddr, Healthcheck: healthcheck})
		}
	}

	return services
}

func NewCocoServiceRegistry(kapi client.KeysAPI, keyPrefix, vulcandAddr string) *CocoServiceRegistry {
	return &CocoServiceRegistry{kapi: kapi, keyPrefix: keyPrefix, vulcandAddr: vulcandAddr}
}
