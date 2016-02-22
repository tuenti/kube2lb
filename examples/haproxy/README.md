# How to use this example

```
make
docker run -it --rm \
	-e KUBECFG=/etc/kube/conf \
	-e DOMAIN=cluster.local \
	-v ~/.kube/config:/etc/kube/conf:ro \
	kube2lb:haproxy
```
