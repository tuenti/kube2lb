## Cluster Information Schema

This is the schema of the data structure that is passed to templates:

* `ClusterInformation`
  * `Services`: List of services in the cluster
    * `Name`
    * `Namespace`
    * `Port`
      * `Port`: Port number
      * `Mode`: "Mode" from haproxy terminology, if TCP or HTTP
      * `Protocol`: TCP/UDP
    * `Endpoints`: List of endpoints of pods serving this service
      * `Name`
      * `IP`
      * `Port`
    * `NodePort`
    * `External`: Additional external names
    * `Timeout`: Connection and response timeout for endpoints of this service
  * `Ports`
    * `Port`
    * `Mode`
    * `Protocol`
  * `Nodes`: List of hostnames of nodes in the cluster
  * `Domain`: Domain of the cluster
