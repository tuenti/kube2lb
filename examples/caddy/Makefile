TAG:=kube2lb:caddy

all: kube2lb
	docker build --tag=$(TAG) .

kube2lb:
	go build ../..

clean:
	rm -f kube2lb
	docker rmi $(TAG) || true
