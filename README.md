# CoCo Aggregate Healthcheck

Looks up multiple services, calls their health check endpoint, caches it and reports back in the standard FT Healthcheck format.
In the same time sends statistics to graphite in a timely manner.

## Usage


#### Endpoints:

* /__health with application/json Accept header
* /__gtg

Both the above are FT Standard compliant

* /__health with text/html Accept header or other

#### Query Params:

* categories: comma separated list of service categories; e.g. `/__health?categories=read,system` will output the health of services labeled `read` or `system`
* cache: flag indicating to use the cached health check results or not (default is true). It could come handy right after deployment. E.g. `/__health?cache=false` will force to re-run all the services health checks

You can use both parameters in your query both on the good-to-go and healthcheck and endpoints even with `application/json` Accept header on the latter; e.g. `/__gtg?categories=read&cache=false`

### Ack support:

Currently if you want to acknowledge a service, you have to manually create an etcd key within the cluster. The etcd key would look like this:

`etcdctl get /ft/healthcheck/foo-service-1/ack`
 `Details of the ack - who acked the service, why, etc.`

The ACK that is not needed anymore should be removed also manually from etcd: `etcdctl rm /ft/healthcheck/foo-service-1/ack`

## Building and running the binary

```
go build
ssh -D 2323 -N core@$FLEETCTL_TUNNEL
./aggregate-healthcheck --socks-proxy localhost:2323 --etcd-peers "http://localhost:2379" --vulcand "localhost:8080" --graphite-host "graphite.ft.com" --graphite-port "2003" --environment "local"
```

## Running in a docker container

```
CGO_ENABLED=0 go build -a -installsuffix cgo -o aggregate-healthcheck .
docker build -t coco/aggregate-healthcheck .
docker run \
    --env ETCD_PEERS=http://localhost:2379 \
    --env VULCAND_ADDRESS=localhost:8080 \
	--env GRAPHITE_HOST=graphite.ft.com \
	--env GRAPHITE_PORT=2003 \
	--env ENVIRONMENT=local \
    coco/aggregate-healthcheck
```

## Code explained

* Every time a change is detected in etcd under the specific keys the servies and categories get redefined.
* When services and categories get redefined only the difference will be copied over in measuredServices.
* Every service has alongside its latest health result cached and a queue/channel containing n health results back in time.
* Every service schedules its next check during the current check. They all roll parallel.
* Every minute the queues/channels are emptied and sent to graphite to store in health timeline for statistics.
