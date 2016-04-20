FROM haproxy:1.6.2

MAINTAINER Jaime Soriano Pastor <jsoriano@tuenti.com>

COPY haproxy.cfg.tpl /etc/kube2lb/haproxy.cfg.tpl
COPY kube2lb /usr/local/bin/kube2lb
COPY entrypoint.sh /entrypoint.sh

ENV HAPROXY_STATS_BIND "127.0.0.1:9090"
ENV TEMPLATE /etc/kube2lb/haproxy.cfg.tpl

EXPOSE 80

CMD ["/entrypoint.sh"]
