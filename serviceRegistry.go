package main

import (
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	servicesKeyPre   = "/ft/healthcheck"
	categoriesKeyPre = "/ft/healthcheck-categories"
	periodKeySuffix  = "/period_seconds"
	resilientSuffix  = "/is_resilient"
	pathSuffix       = "/path"
	categoriesSuffix = "/categories"
	defaultDuration  = time.Duration(60 * time.Second)
	defaultPath      = "__health"
	//defaultCategory = Category{"default", defaultDuration, false}
)

type Service struct {
	Name       string
	Host       string
	Path       string
	Categories []string
}

type Category struct {
	Name        string
	Period      time.Duration
	IsResilient bool
}

type MeasuredService struct {
	service         *Service
	cachedHealth    *CachedHealth
	bufferedHealths *BufferedHealths //up to 60 healthiness measurements to be buffered and sent at once graphite
}

func NewMeasuredService(service *Service) MeasuredService {
	return MeasuredService{service, NewCachedHealth(), NewBufferedHealth()}
}

type ServiceRegistry struct {
	etcd             client.KeysAPI
	vulcandAddr      string
	checker          HealthChecker
	services         map[string]Service
	categories       map[string]Category
	measuredServices map[string]MeasuredService
}

func NewCocoServiceRegistry(etcd client.KeysAPI, vulcandAddr string) *ServiceRegistry {
	return &ServiceRegistry{etcd: etcd, vulcandAddr: vulcandAddr}
}

func (r *ServiceRegistry) watchContinuously(key string) {
	watcher := r.etcd.Watcher(key, &client.WatcherOptions{0, true})
	for {
		_, err := watcher.Next(context.Background())
		if err != nil {
			log.Printf("ERROR - Error waiting for change under %v in etcd. %v\n Sleeping 10s...", key, err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		r.redefineCategoryList()
		r.redefineServiceList()

		// adding new services, not touching existing
		for _, service := range r.services {
			if _, ok := r.measuredServices[service.Name]; !ok {
				mService := NewMeasuredService(&service)
				r.measuredServices[service.Name] = mService
				go r.scheduleCheck(&mService, time.NewTimer(0))
			}
		}

		// removing services that don't exist, not toching the rest
		for _, measuredService := range r.measuredServices {
			if _, ok := r.services[measuredService.service.Name]; !ok {
				delete(r.measuredServices, measuredService.service.Name)
			}
		}
	}
}

func (r *ServiceRegistry) redefineServiceList() {
	services := make(map[string]Service)
	servicesResp, err := r.etcd.Get(context.Background(), servicesKeyPre, &client.GetOptions{Sort: true})
	if err != nil {
		log.Printf("ERROR - Failed to get value from %v: %v.", servicesKeyPre, err.Error())
		return
	}
	if !servicesResp.Node.Dir {
		log.Printf("ERROR - %v is not a directory", servicesResp.Node.Key)
		return
	}
	for _, serviceNode := range servicesResp.Node.Nodes {
		if !serviceNode.Dir {
			log.Printf("ERROR - %v is not a directory", serviceNode.Key)
			return
		}
		name := filepath.Base(servicesResp.Node.Key)
		path := defaultPath
		pathResp, err := r.etcd.Get(context.Background(), serviceNode.Key+pathSuffix, &client.GetOptions{Sort: true})
		if err != nil {
			log.Printf("WARN - Failed to get health check path from %v: %v. Using default %v", serviceNode.Key, err.Error(), defaultPath)
		} else {
			path = pathResp.Node.Value
		}

		categoriesResp, err := r.etcd.Get(context.Background(), serviceNode.Key+categoriesSuffix, &client.GetOptions{Sort: true})
		var categories []string

		//TODO simplify this
		if err != nil {
			log.Printf("WARN - Failed to get app category from %v: %v. Using default 'default'", serviceNode.Key, err.Error())
			categories = append(categories, "default")
		} else {
			if !categoriesResp.Node.Dir {
				categories = strings.Split(categoriesResp.Node.Value, ",")
			} else {
				log.Printf("WARN - Failed to get app category from %v: %v. Using default 'default'", serviceNode.Key, err.Error())
				categories = append(categories, "default")
			}
		}
		services[name] = Service{Name: name, Host: r.vulcandAddr, Path: path, Categories: categories}
	}
	r.services = services
}

func (r *ServiceRegistry) redefineCategoryList() {
	categories := make(map[string]Category)
	categoriesResp, err := r.etcd.Get(context.Background(), categoriesKeyPre, &client.GetOptions{Sort: true})
	if err != nil {
		log.Printf("ERROR - Failed to get value from %v: %v.", categoriesKeyPre, err.Error())
		return
	}
	if !categoriesResp.Node.Dir {
		log.Printf("ERROR - %v is not a directory", categoriesResp.Node.Key)
		return
	}
	for _, categoryNode := range categoriesResp.Node.Nodes {
		if !categoryNode.Dir {
			log.Printf("ERROR - %v is not a directory", categoryNode.Key)
			return
		}
		name := filepath.Base(categoryNode.Key)
		periodResp, err := r.etcd.Get(context.Background(), categoryNode.Key+periodKeySuffix, &client.GetOptions{Sort: true})
		period := defaultDuration
		if err != nil {
			log.Printf("WARN - Failed to get health check period from %v: %v. Using default %v", categoryNode.Key, err.Error(), defaultDuration)
		} else {
			periodInt, err := strconv.Atoi(periodResp.Node.Value)
			if err != nil {
				log.Printf("WARN - Error reading health check period value '%v'. Using default %v", periodResp.Node.Value, defaultDuration)
				periodInt = int(defaultDuration.Seconds())
			}
			period = time.Duration(periodInt) * time.Second
		}
		resilientResp, err := r.etcd.Get(context.Background(), categoryNode.Key+resilientSuffix, nil)
		resilient := false
		if err != nil {
			log.Printf("WARN - Failed to get resilient setting from %v: %v. Using default: false.\n", categoryNode.Key, err.Error())
		} else {
			resilientBool, err := strconv.ParseBool(periodResp.Node.Value)
			if err != nil {
				log.Printf("WARN - Error reading resilient setting '%v'. Using default: false.", resilientResp.Node.Value)
				resilientBool = false
			}
			resilient = resilientBool
		}

		categories[name] = Category{Name: name, Period: period, IsResilient: resilient}
	}
	r.categories = categories
}

func (registry ServiceRegistry) scheduleCheck(mService *MeasuredService, timer *time.Timer) {
	// wait
	<-timer.C

	// check
	timedHealth := registry.checker.checkHealthSimple(mService.service)

	// write to cache
	mService.cachedHealth.latestWrite <- *timedHealth

	// write to graphite buffer
	select {
	case mService.bufferedHealths.buffer <- timedHealth:
	default:
	}

	// schedule next check
	_, ok := registry.services[mService.service.Name]
	if !ok {
		return
	}
	waitDuration := findShortestPeriod(registry.categories, mService.service.Categories)
	go registry.scheduleCheck(mService, time.NewTimer(waitDuration))
}

func findShortestPeriod(allCategories map[string]Category, categoryNames []string) time.Duration {
	minSeconds := defaultDuration.Seconds()
	minDuration := defaultDuration
	for _, categoryName := range categoryNames {
		category := allCategories[categoryName]
		if category.Period.Seconds() < minSeconds {
			minSeconds = category.Period.Seconds()
			minDuration = category.Period
		}
	}
	return minDuration
}
