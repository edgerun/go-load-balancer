# Go Load Balancer

The aim of this project is to provide a simple and extendable load balancer implementation to use in a Kubernetes
cluster.

While the behavior of each component will be changeable, the main focus lies on a weighted round robin strategy.
Whereas, the weights are obtained from `etcd`. This allows us to develop external optimisation components that modify
the `etcd` storage, that will ultimately lead to updating the weights for the servers.

The load balancer should be exposed and callable on the node. The server will act as a reverse proxy and routes traffic
based on the called url.

The request url is oriented towards a Function-as-a-Service platform. Therefore, requests have to follow the following
pattern: `ip:port/function/<function-to-call>`
The call will be forwarded to a `Pod` that runs an image with the same name.

Build
-----

To build the module's binaries using your local go installation run:

    make

To build Docker images for local usage (without a go installation) run - tagged as `edgebench/go-load-balancer:latest`:

    make docker

To publish the Docker images, you can pass an argument that serves as version:

    make docker-release 

## Components

1. `Server`: Listens and accepts connections, passes them to `Handler`
2. `Handler`: gets requests and must implement reverse-proxy mechanism.
3. `WeightedRoundRobinHandler`: implements `Handler` interface with weighted round-robin strategy to distributed load.
   Uses `WeightUpdater` to receive weight updates.
4. `WeightUpdater`: can be subscribed to and publishes new weights for services.
5. `EtcdWeightUpdater`: implements the `WeightUpdater` interface with `etcd`

## Weight updates

The daemon watches etcd keys with the following prefix: `golb/function/<eb_go_lb_zone>`.

Following command can be used to update the weights:

    etcdctl put golb/function/zone-b/resnet '{"ips": ["10.0.0.1"], "weights":[3]}'

Be aware that previous weights are overwritten, and therefore all data must be supplied - partial updates are not
supported

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `eb_go_lb_etcd_host`     | localhost:2379  | The host of  etcd | 
| `eb_go_lb_zone`          | -  | In which zone the LB starts (important for watching the right `etcd` keys |
| `eb_go_lb_handler_type`     | dummy | Handler type (currently: `dummy` & `wrr`)
| `eb_go_lb_mode`          | `dev` | Mode of execution (`prod` or `dev`) |
| `eb_go_lb_node_name`  | $HOSTNAME | The node name used to indicate a forward |
| `eb_go_lb_listen_port` | 8080 | The port to listen on |
| `eb_go_lb_gateways` |  | A whitespace seperated list of URLs that indicate the load balancer which url is a load balancer |

## Thanks to

[JJNP](https://github.com/jjnp) for
the [Weighted WRR implementation](https://github.com/jjnp/traefik/blob/df39dad2e9ebacfca3e7b39df038814dafa98be3/pkg/server/loadbalancer/custom/wrr_provider.go)