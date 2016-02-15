TAG:=caddy

all: kube2lb
	docker build --tag=kube2lb:$(TAG) .

kube2lb:
	go build ../..

clean:
	rm -f kube2lb
	docker rmi kube2lb:$(TAG)
