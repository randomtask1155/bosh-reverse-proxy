# bosh reverse proxy

Given a config that includes an array of objects with route, deployment and job specificied for each bosh reverse proxy will go and find the instance ip addresses and map them to the specified route. 

# Config

example

```
[ 
    {
        "route": "grafana.cfapps.domain", 
        "deployment-prefix": "prometheus", 
        "job": "grafana"
    }
]
```

# manifest

example Replace BOSH_SECRET and BOSH_IP with the bosh client `ops_manager` secret and bosh director IP.  This enables the reverse proxy to query the `/deployments` and `/deployments/<CF-DEPLOYMENT>/instances` api endpoints for instance ip addresses.

```
---
applications:
  - name: bosh-reverse-proxy
    memory: 128M
    instances: 1
    buildpacks: 
    - go_buildpack
    command: bosh-reverse-proxy -f map-config.json -secret BOSH_SECRET -host BOSH_IP
    readiness-health-check-type: process
    routes:
    - route: grafana.cfapps.domain
```

# deploy

cf push