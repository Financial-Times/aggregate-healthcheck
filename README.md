# Coco Aggregate Healthcheck Service

Looks up the services exposed through vulcand, calls their /__health endpoint and reports back in the standard FT Healthcheck format.

## Building

```
CGO_ENABLED=0 go build -a -installsuffix cgo -o coco-aggregate-healthcheck .

docker build -t coco/coco-aggregate-healthcheck .
```

## Running

```
docker run \
    --env ETCD_PEERS=http://localhost:2379 \
    --env VULCAND_ADDRESS=localhost:8080 \
	--env GRAPHITE_HOST=graphite.ft.com \
	--env GRAPHITE_PORT=2003 \
	--env ENVIRONMENT=local \
    coco/coco-aggregate-healthcheck
```

Binary
```
ssh -D 2323 -N core@$FLEETCTL_TUNNEL
./coco-aggregate-healthcheck --socks-proxy localhost:2323 --etcd-peers "http://localhost:2379" --vulcand "localhost:8080" --graphite-host "graphite.ft.com" --graphite-port "2003" --environment "local"
```

## Usage


#### Endpoints:

* /__health
* /__gtg

Both are FT Standard compliant

#### Query Params:

* categories: comma separated list of service categories; e.g. `/__health?categories=read,system` will output the health of services labeled `read` or `system`
* cache: flag indicating to use the cached health check results or not (default is true). It could come handy right after deployment. E.g. `/__health?cache=false` will force to re-run all the services health checks

You can use both parameters in your query; e.g. `/__gtg?categories=read&cache=false`
