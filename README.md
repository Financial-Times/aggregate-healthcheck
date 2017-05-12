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
#### Service level ack
Currently if you want to acknowledge a service, you have to manually create an etcd key within the cluster. The etcd key would look like this:

`etcdctl get /ft/healthcheck/foo-service-1/ack`
 `Details of the ack - who acked the service, why, etc.`

The etcd key have to be set in the following way:

`etcdctl set /ft/healthcheck/foo-service-1/ack 'Details of the ack - who acked the service, why, etc.'`

The ACK that is not needed anymore should be removed also manually from etcd: `etcdctl rm /ft/healthcheck/foo-service-1/ack`

#### Cluster level ack
The cluster can be acked as a whole.

Consequences of acking the whole cluster:
- When a cluster is acked, it will appear its dashing tile will be green
- The UI will have a text message in the heading: (Cluster is acked: ACK-MESSAGE)
- In the JSON response of agg-hc __health endpoint the main ok field will be true, event though some services might be unhealthy

Note:
 - The gtg endpoint will have the same functionality as before (it will continue to respond with 503 if it is the case, even though the cluster is acknowledged) to ensure a proper work of failovers


To ack the cluster, add the following etcd entry:

`etcdctl set /ft/config/aggregate-healthcheck/cluster-ack <ACK-MESSAGE>`

To remove the ack, remove the etcd entry that holds the ack message:

`etcdctl rm /ft/config/aggregate-healthcheck/cluster-ack`

### Categories:

Possible categories, that an app can be part of are defined in _etcd_ under `/ft/healthcheck-categories/`. Attributes are `period_seconds` and `is_resilient` (true or false).

If a category is resilient, it means that the overall health of the cluster will only degrade if all instances of any app group are unhealthy.

If document-store-api 1 and 2 are both down, only then will the overall health of the cluster be a _warn_.

If category is not resilient, any app's degradation will affetct the cluster health immediately.

`period_seconds` is the maximum time period at which apps in the respective category must be checked upon. For a given app this period may be shorter, but not longer, depending on which other shorter period categories it resides in also.

### Sticky support:

A healthcheck can be marked as 'sticky' by setting the etcd value for the category:

`etcdctl set /ft/healthcheck-categories/<category>/sticky true`

This lets the healthcheck know that if the healthcheck for the category ever fails, it should stay failed rather than healing as normal.  It does this by setting the `/enabled` key to false.  To re-enable the healthcheck, `/enabled` will need to manually be set to true (in the same manner as a manual failover):

`etcdctl set /ft/healthcheck-categories/<category>/enabled true`

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

* Every time a change is detected in etcd under the specific keys the services and categories get redefined.
* When services and categories get redefined only the difference will be copied over in measuredServices.
* Every service has alongside its latest health result cached and a queue/channel containing n health results back in time.
* Every service schedules its next check during the current check. They all roll parallel.
* Every minute the queues/channels are emptied and sent to graphite to store in health timeline for statistics.
