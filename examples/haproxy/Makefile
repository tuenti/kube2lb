TAG:=kube2lb:haproxy

all: kube2lb
	docker build --tag=$(TAG) .

kube2lb:
	make -C ../..
	cp ../../kube2lb .

clean:
	rm -f kube2lb
	docker rmi $(TAG) || true
