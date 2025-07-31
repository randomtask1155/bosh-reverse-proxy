package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"math/rand/v2"
)

type RouteMapController struct {
	Maps []RouteMaps
}

type RouteMaps struct {
	Route            string `json:"route"`
	Deployment       string `json:"deployment"`
	DeploymentPrefix string `json:"deployment-prefix"`
	Job              string `json:"job"`
	HostList         []string
}

func LoadRouteMapsFromFile(f string) (RouteMapController, error) {
	rmc := RouteMapController{make([]RouteMaps, 0)}
	b, err := os.ReadFile(f)
	if err != nil {
		return rmc, err
	}
	err = json.Unmarshal(b, &rmc.Maps)
	//go rmc.RouterSyncer()
	return rmc, err
}

func LoadRouteMapsFromString(f string) (RouteMapController, error) {
	rmc := RouteMapController{make([]RouteMaps, 0)}
	err := json.Unmarshal([]byte(f), &rmc)
	//go rmc.RouterSyncer()
	return rmc, err
}

func (rmc *RouteMapController) RouteMapDirector(req *http.Request) {
	backendURLHost := ""
	backendHostHeader := ""
	for _, r := range rmc.Maps {
		if r.Route == req.Host {
			if len(r.HostList) <= 1 {
				logger.Error("Route has no BOSH host mappings", "route", r.Route)
				break
			}
			index := rand.IntN(len(r.HostList))
			backendURLHost = r.HostList[index]
			backendHostHeader = r.Route
		}
	}
	req.URL.Scheme = "https"
	req.URL.Host = backendURLHost
	req.Host = backendHostHeader
	req.Header.Set("Host", backendHostHeader)
}

func (rmc *RouteMapController) LoadBoshMappings(c, s, h string) error {
	b, err := NewBoshClient(c, s, h)
	if err != nil {
		return err
	}

	bi, err := b.GetInstances()
	if err != nil {
		return err
	}

	for i, m := range rmc.Maps {
		for _, deployment := range bi.Deployments {
			if strings.HasPrefix(deployment.Name, m.DeploymentPrefix) || deployment.Name == m.Deployment {
				rmc.Maps[i].Deployment = deployment.Name
				//rmc.Maps[i].HostList = make([]string, 0) // reset hostlist so it does not grow uncontrollably during resync
				for _, instance := range deployment.Instances {
					if instance.Job == m.Job {
						if len(instance.IPs) > 0 {
							logger.Debug("adding instance ip to route map", "deployment", deployment.Name, "instance", instance.Job, "ip_count", len(instance.IPs), "ip", instance.IPs[0])
							rmc.Maps[i].HostList = append(rmc.Maps[i].HostList, instance.IPs...)
						}
					}
				}
			}
		}
	}
	return nil
}

func (rmc *RouteMapController) RouterSyncer() {
	for {
		logger.Debug("reloading bosh mappings")
		err := rmc.LoadBoshMappings(*boshClient, *boshSecret, *boshHost)
		if err != nil {
			logger.Error("failed to reload bosh mappings", "error", err)
		}
		time.Sleep(1 * time.Minute)
	}
}
