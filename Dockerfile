FROM debian:jessie

MAINTAINER Jaime Soriano Pastor <jsoriano@tuenti.com>

RUN apt-get update && \
	apt-get install -y supervisor wget && \
	apt-get clean

RUN wget "https://caddyserver.com/download/build?os=linux&arch=amd64&features=" -O /tmp/caddy.tar.gz && \
	tar xvfz /tmp/caddy.tar.gz -C /usr/local/bin caddy && \
	mkdir -p /etc/caddy && \
	touch /etc/caddy/Caddyfile && \
	rm -f /tmp/caddy.tar.gz

COPY templates /etc/kube2lb
COPY kube2lb /usr/local/bin/kube2lb
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

ENV TEMPLATE /etc/kube2lb/Caddyfile.tpl

EXPOSE 80

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/supervisord.conf"]
