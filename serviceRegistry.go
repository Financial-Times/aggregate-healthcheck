package main

import (
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"reflect"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"fmt"
	"sort"
)

const (
	servicesKeyPre   = "/ft/healthcheck"
	categoriesKeyPre = "/ft/healthcheck-categories"
	periodKeySuffix  = "/period_seconds"
	resilientSuffix  = "/is_resilient"
	pathSuffix       = "/path"
	categoriesSuffix = "/categories"
	defaultDuration  = time.Duration(60 * time.Second)
	pathPre          = "/health/%s%s"
	defaultPath      = "/__health"
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
	cachedHealth    *CachedHealth    //latest healthiness measurement
	bufferedHealths *BufferedHealths //up to 60 healthiness measurements to be buffered and sent at once graphite
}

func NewMeasuredService(service *Service) MeasuredService {
	cachedHealth := NewCachedHealth()
	bufferedHealths := NewBufferedHealths()
	go cachedHealth.maintainLatest();
	return MeasuredService{service, cachedHealth, bufferedHealths}
}

type ServiceRegistry struct {
	etcd             client.KeysAPI
	vulcandAddr      string
	checker          HealthChecker
	services         map[string]Service
	categories       map[string]Category
	measuredServices map[string]MeasuredService
}

func NewCocoServiceRegistry(etcd client.KeysAPI, vulcandAddr string, checker HealthChecker) *ServiceRegistry {
	services := make(map[string]Service)
	categories := make(map[string]Category)
	measuredServices := make(map[string]MeasuredService)
	return &ServiceRegistry{etcd, vulcandAddr, checker, services, categories, measuredServices}
}

func (r *ServiceRegistry) watchServices() {
	watcher := r.etcd.Watcher(servicesKeyPre, &client.WatcherOptions{0, true})
	for {
		_, err := watcher.Next(context.Background())
		if err != nil {
			log.Printf("ERROR - Error waiting for change under %v in etcd. %v\n Sleeping 10s...", servicesKeyPre, err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		log.Printf("DEBUG - Change detected under %v in etcd.", servicesKeyPre)
		r.redefineServiceList()
		r.updateMeasuredServiceList()
	}
}

func (r *ServiceRegistry) updateMeasuredServiceList() {
	// adding new services, not touching existing
	var debugLines []string
	for key, _ := range r.services {
		service := r.services[key];
		if mService, ok := r.measuredServices[service.Name]; !ok || !reflect.DeepEqual(service, r.measuredServices[service.Name].service) {
			debugLines = append(debugLines, fmt.Sprintf("    %v\n", service.Name))
			if ok {
				mService.cachedHealth.terminate <- true
			}
			newMService := NewMeasuredService(&service)
			r.measuredServices[service.Name] = newMService
			go r.scheduleCheck(&newMService, time.NewTimer(0))
		}
	}
	sort.Strings(debugLines)
	log.Printf("DEBUG - Measured service to be (re-)created:\n%v", debugLines)

	// removing services that don't exist, not toching the rest
	debugLines = []string {}
	for _, mService := range r.measuredServices {
		if _, ok := r.services[mService.service.Name]; !ok {
			debugLines = append(debugLines, fmt.Sprintf("    %v\n", mService.service.Name))
			delete(r.measuredServices, mService.service.Name)
			mService.cachedHealth.terminate <- true
		}
	}
	sort.Strings(debugLines)
	log.Printf("DEBUG - Measured service to be removed:\n%v", debugLines)

	debugLines = []string {}
	for _, mService := range r.measuredServices {
		debugLines = append(debugLines, fmt.Sprintf("    %v\n", mService.service.Name))
	}
	log.Printf("DEBUG - All measured services:\n%v", debugLines)
}

func (r *ServiceRegistry) watchCategories() {
	watcher := r.etcd.Watcher(categoriesKeyPre, &client.WatcherOptions{0, true})
	for {
		_, err := watcher.Next(context.Background())
		if err != nil {
			log.Printf("ERROR - Error waiting for change under %v in etcd. %v\n Sleeping 10s...", categoriesKeyPre, err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		log.Printf("DEBUG - Change detected under %v in etcd.", categoriesKeyPre)
		r.redefineCategoryList()
	}
}

func (r *ServiceRegistry) redefineServiceList() {
	log.Printf("DEBUG - Redefining service list.")
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
			log.Printf("WARN - %v is not a directory", serviceNode.Key)
			continue
		}
		name := filepath.Base(serviceNode.Key)
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
				log.Printf("WARN - Failed to get app category from %v: %v. Using default 'default'", categoriesResp.Node.Key, err.Error())
				categories = append(categories, "default")
			}
		}
		services[name] = Service{Name: name, Host: r.vulcandAddr, Path: fmt.Sprintf(pathPre, name, path), Categories: categories}
	}

	var debugList []string
	for _, service := range services {
		debugList = append(debugList, fmt.Sprintf("    %v: %v\n", service.Name, service.Categories))
	}
	sort.Strings(debugList)
	debug := "DEBUG - Services are:\n"
	for _, debugElem := range debugList {
		debug += debugElem
	}
	log.Print(debug)
	r.services = services
}

func (r *ServiceRegistry) redefineCategoryList() {
	log.Printf("DEBUG - Redefining category list.")
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
			log.Printf("WARN - %v is not a directory", categoryNode.Key)
			continue
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
			resilientBool, err := strconv.ParseBool(resilientResp.Node.Value)
			if err != nil {
				log.Printf("WARN - Error reading resilient setting '%v' at key %v. Using default: false.", resilientResp.Node.Value, resilientResp.Node.Key)
				resilientBool = false
			}
			resilient = resilientBool
		}

		categories[name] = Category{Name: name, Period: period, IsResilient: resilient}
	}
	log.Printf("DEBUG - Categories are: %v", categories)
	r.categories = categories
}

func (registry ServiceRegistry) scheduleCheck(mService *MeasuredService, timer *time.Timer) {
	// wait
	select {
	case <- mService.cachedHealth.terminate:
		log.Printf("DEBUG - Terminating schedule for %v\n", mService.service.Name)
		return
	case <-timer.C:
	}

	// check
	//log.Printf("DEBUG - Checking %v\n", mService.service.Name)
	healthResult := fthealth.RunCheck(mService.service.Name,
		fmt.Sprintf("Checks the health of %v", mService.service.Name),
		true,
		NewServiceHealthCheck(*mService.service, registry.checker))
	//log.Printf("DEBUG - Received new health results for [%v]. name: %v, status: [%v], nChecks: %v, firstCheckStatus: %v, output: %v", mService.service.Name, healthResult.Name, healthResult.Ok, len(healthResult.Checks), healthResult.Checks[0].Ok, healthResult.Checks[0].Output)

	// write to cache
	mService.cachedHealth.toWriteToCache <- healthResult

	// write to graphite buffer
	select {
	case mService.bufferedHealths.buffer <- healthResult:
	default:
	}

	waitDuration := registry.findShortestPeriod(*mService.service)
	go registry.scheduleCheck(mService, time.NewTimer(waitDuration))
}

func (registry ServiceRegistry) findShortestPeriod(service Service) time.Duration {
	minSeconds := defaultDuration.Seconds()
	minDuration := defaultDuration
	for _, categoryName := range service.Categories {
		category, ok := registry.categories[categoryName]
		if !ok {
			continue
		}
		if category.Period.Seconds() < minSeconds {
			minSeconds = category.Period.Seconds()
			minDuration = category.Period
		}
	}
	return minDuration
}
