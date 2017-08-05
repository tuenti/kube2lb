# Kube2lb design

## Overview

Kube2lb is a tool to configure load balancers or similar software according
to the state of services and its endpoints in a Kubernetes cluster.

In general terms, it watches the state of a Kubernetes cluster and with this
information and a template file generates a configuration file that can be
used by the load balancer. Finally it can notify the load balancer so it
knows that must reload the configuration.

## Architecture

Kube2lb is based on these components:
* Kubernetes client
* Local store
* Template processor
* Notifier

Kube2lb itself works basically as a loop that on events received from the
Kubernetes client, it updates the information stored in a local store, then it
generates a new configuration file with the template processor and finally
notifies the required reload with the notifier.

Kube2lb is interested in these Kubernetes resources:
* Nodes
* Services
* Endpoints

### Kubernetes client

This component maintains all the pieces together, it has these resposibilities:
* Run the main loop
* Keep the connection with the Kubernetes apiserver alive
* Locally cache received cluster information
* Invoke the template processor and the notifier when state changes

For each one of the Kubernetes resources we are interested on, the client
keeps a watcher and a local store. For the watchers it also keeps the last
resource version received. Resource version is a monotonically increasing
number that any resource or event in Kubernetes has. When watching for
events a resource version can be provided so only events referencing
greater resource versions are received, if no resource version is provided,
the watcher receives `Added` events for any resource in the cluster, so the
whole state can be known from them. If we have to reconnect the watchers we
provide the last resource version seen so we don't lose or get duplicated
information. Events received can be of type `Added`, `Modified`, `Deleted`
or `Error`.

For each kind of event we do:
* `Added`, the object is added to the store and an update is triggered.
* `Modified`, the state of the object in the store is updated, and if they
  are not equal, an update is triggered. Equality here depends on the type
  of the resource.
* `Deleted`, the object is removed from the store and an update is triggered.
* `Error`, coherence of resource versions between kube2lb and apiservers is
  considered lost, state is reset and watchers are reconnected with no
  resource version so state can be rebuilt from events.

With this approach all the state is received from events and no additional
query is done to the apiserver.

#### Triggering updates

When after receiving an event, an update is triggered, these are the actions
executed:
* The information in local stores is converted to an
  [schema](cluster_information_schema.md) that can be more easily consumed by
  templates.
* Template is processed.
* Notifier is executed.

Cluster information contains information about services of type LoadBalancer
or NodePort, we consider that other types of services are not though to be
externaly exposed.

In case of bursts of events (for example when kube2lb is started), even if
several updates are triggered, only one is executed.

### Local stores

Local stores are data structures that cache all the information received from
events so it doesn't need to be requested again.

For each type of resource there is an specific local store. All of them
implement a local interface, and some additional specific methods.
The common interface is:
* `Delete(runtime.Object) runtime.Object`
* `Update(runtime.Object) runtime.Object`
* `Equal(runtime.Object, runtime.Object) (bool, error)`

`Delete` and `Update` methods return the object that was stored if any.
All objects are identified by its `SelfLink` field that should be unique
in a Kubernetes cluster.

`Equal` method is intended to compare two objects of the type stored.
Two objects should be considered equal if the information they contain
generates the same load balancers configuration. So for example for
Kube2lb two `Endpoints` objects are equal if their lists of endpoints
are the same, it doesn't care about having different resource versions
or annotations.

The equality check we are considering for each kind of objects are:
* `Service`: Equal if their resource versions are equal
* `Endpoints`:  Equal if their lists of endpoints are equal
* `Node`: Equal if their hostnames are equal

### Template processor

Templates in kube2lb are go templates and when executed they receive all the
cluster information in a proper [schema](cluster_information_schema.md) that
should be easily consumed, and a set of functions that can help in filling the
templates.

### Notifier

Notifiers can be configured to notify a service that its configuration has
changed. This notification can be done by running a command or sending a
signal.
