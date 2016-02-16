# Kube2lb

Dinamically configure a load balancer by reading services information from
Kubernetes API.

`kube2lb` uses a template and the information obtained from a Kubernetes API to
generate a configuration file for a load balancer, then `kube2lb` signals the
load balancer to reload the configuration.

It's intended to be used on Kubernetes clusters deployed on bare-metal that
need to expose services to applications running out of the cluster, with a
similar approach to cloud providers in Kubernetes.

## Quick start

`kube2lb` needs to know the location of the template, the configuration file
used by the load balancer, its PID and the Kubernetes API endpoint.

For example, to configure a [Caddy server](https://caddyserver.com/):

```
echo localhost :8080 > Caddyfile
caddy -conf=Caddyfile -pidfile=caddy.pid

kube2lb -kubecfg=~/.kube/config \
	-template=examples/caddy/Caddyfile.tpl \
	-config=Caddyfile \
	-domain=cluster.local \
	-notify=pidfile:SIGUSR1:caddy.pid
```

This will read Kubernetes client configuration from `~/.kube/config`, the
template from the examples directory and will pass the domain `cluster.local`
to the template execution to generate the host names. Then when something
changes it will generate the configuration file and notify on the process
whose PID is the one in caddy.pid with the `SIGUSR1` signal, the one used
by Caddy for online configuration reload.

You can see examples in the `examples` directory.

## Compiling

`godep restore` is required to use vendorized dependencies.

```
make
```

## Configuration details

Kubernetes connections to the API are done using the same libraries as other
tools as `kubectl` or `kube2sky` and following similar principles.

Configuration is taken in this order:

1. API server (`-apiserver` flag)
1. Configuration file (`-kubecfg` flag, api server endpoint can be overriden
   with `-apiserver`)
1. In cluster configuration, useful if `kube2lb` is deployed in a pod

Templates receive the list of nodes, services and the domain passed with the
`-domain` flag.

`kube2lb` can be used with any service that is configured with configuration
files and can do online configuration reload.

By now these notifier definitions can be used:

* `pid:SIGNAL:PID` (e.g: `-notify pid:SIGHUP:5678`), to notify to an specific
  pid
* `pidfile:SIGNAL:PIDFILE` (e.g: `-notify pidfile:SIGUSR1:/var/run/caddy.pid`),
  to notify to the pid in a pidfile.
* `debug:` (`-notify debug:`), doesn't notify, it just logs when `kube2lb`
  detects a change in nodes or services, it can be used to test
  configurations.

## Credits & Contact

`kube2lb` was created by [Tuenti Technologies S.L.](http://github.com/tuenti)

You can follow Tuenti engineering team on Twitter [@tuentieng](http://twitter.com/tuentieng).

## License

`kube2lb` is available under the Apache License, Version 2.0. See LICENSE file
for more info.
