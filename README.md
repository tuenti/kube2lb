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

### Kubeconfig

Kubernetes connections to the API are done using the same libraries as other
tools as `kubectl` or `kube2sky` and following similar principles.

Configuration is taken in this order:

1. API server (`-apiserver` flag)
1. Configuration file (`-kubecfg` flag, api server endpoint can be overriden
   with `-apiserver`)
1. In cluster configuration, useful if `kube2lb` is deployed in a pod

### Server names

Templates receive the list of nodes, services and the domain passed with the
`-domain` flag.

Templates can use the `ServerNames` function, that generates a list of server
names to be used in load balancers configuration. This list is generated using
a comma-separated list of templates passed with the `-server-name-templates`.

For example, this flag could be used to generate two server names for each
service, one with just the service name as a subdomain of example.com,
and another one with the default names used by kubernetes:
```
kube2lb ... -server-name-templates "{{ .Service.Name }}.example.com,{{ .Service.Name }}.{{ .Service.Namespace }}.svc.{{ .Domain }}"
```

Additional server names can be added also as a comma-sepparated list in the
`kube2lb/external-domains` annotation in the service definition, e.g:
```
apiVersion: v1
kind: Service
metadata:
  annotations:
    kube2lb/external-domains: test.example.com,~^(test1|test2)\.example\.(com|net)$
...
```

Use `~` to indicate that it must be handled as a regular expression.

And in the configuration file template:
```
{{ range $serverName := ServerNames $service $domain }}
{{- if $serverName.IsRegexp }}
acl svc_{{ $label }} hdr_reg(host) {{ $serverName.Regexp }}{{- else }}
acl svc_{{ $label }} hdr(host) -i {{ $serverName }}{{- end }}
{{- end }}
```

### Port modes

Load balancers use to differenciate TCP and HTTP connections, for HTTP
connections they have additional features as the possibility of choosing a
backend depending on an specific HTTP header. `kube2lb` allows to declare
different modes for some ports.

Default mode can be changed with the `-default-port-mode` flag, it is 
http by default.

An annotation can be used to declare different modes. e.g:
```
apiVersion: v1
kind: Service
metadata:
  annotations:
    kube2lb/port-mode: |
      { "mysql": "tcp" }
...
```
The annotation must be a string to string map represented as valid JSON, with
the port name as key and the mode as value. Ports must declare their names
in order to use this feature.

### Notifiers

`kube2lb` can be used with any service that is configured with configuration
files and can do online configuration reload. To notify the service that it
must reload its configuration a notifier needs to be configured.

By now these notifier definitions can be used:

* `command:COMMAND` executes a command to notify, this command is executed
  inside a shell (e.g: `-notify command:"haproxy -f /etc/haproxy.cfg -p /run/haproxy.pid -sf \$(cat /run/haproxy.pid)"`)
* `pid:SIGNAL:PID` notifies to an specific pid (e.g: `-notify pid:SIGHUP:5678`)
* `pidfile:SIGNAL:PIDFILE` notifies to the pid in a pidfile (e.g: `-notify pidfile:SIGUSR1:/var/run/caddy.pid`)
* `debug:` doesn't notify, it just logs when `kube2lb` detects a change in
  nodes or services, it can be used to test configurations.

## Credits & Contact

`kube2lb` was created by [Tuenti Technologies S.L.](http://github.com/tuenti)

You can follow Tuenti engineering team on Twitter [@tuentieng](http://twitter.com/tuentieng).

## License

`kube2lb` is available under the Apache License, Version 2.0. See LICENSE file
for more info.
