FROM haproxy:1.6.3

MAINTAINER Jaime Soriano Pastor <jsoriano@tuenti.com>

COPY haproxy.cfg.tpl /etc/kube2lb/haproxy.cfg.tpl
COPY kube2lb /usr/local/bin/kube2lb
COPY entrypoint.sh /entrypoint.sh

ENV TEMPLATE /etc/kube2lb/haproxy.cfg.tpl

EXPOSE 80

CMD ["/entrypoint.sh"]
