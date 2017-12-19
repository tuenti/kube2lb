#!/bin/bash

CONFFILE=/usr/local/etc/haproxy/haproxy.cfg

echo "
global
	daemon

frontend nothing
	bind :80
" > $CONFFILE

sed -i $TEMPLATE \
	-e "s/__HAPROXY_NBPROC__/$HAPROXY_NBPROC/" \
	-e "s/__HAPROXY_MAXCONN__/$HAPROXY_MAXCONN/" \
	-e "s/__HAPROXY_FRONTEND_MAXCONN__/$HAPROXY_FRONTEND_MAXCONN/" \
	-e "s/__HAPROXY_SERVER_MAXCONN__/$HAPROXY_SERVER_MAXCONN/" \
	-e "s/__HAPROXY_HTTP_REUSE__/$HAPROXY_HTTP_REUSE/" \
	-e "s/__HAPROXY_TIMEOUT_CONNECT__/$HAPROXY_TIMEOUT_CONNECT/" \
	-e "s/__HAPROXY_TIMEOUT_CLIENT__/$HAPROXY_TIMEOUT_CLIENT/" \
	-e "s/__HAPROXY_TIMEOUT_SERVER__/$HAPROXY_TIMEOUT_SERVER/" \
	-e "s/__HAPROXY_TIMEOUT_KEEPALIVE__/$HAPROXY_TIMEOUT_KEEPALIVE/" \
	-e "s/__HAPROXY_TIMEOUT_TUNNEL__/$HAPROXY_TIMEOUT_TUNNEL/" \
	-e "s/__SYSLOG__/$SYSLOG/"

exec kube2lb -apiserver="$APISERVER" -kubecfg="$KUBECFG" -template="$TEMPLATE" -server-name-templates="$SERVER_NAME_TEMPLATES" -config="$CONFFILE" -domain="$DOMAIN" -default-lb-ip="$DEFAULT_LB_IP" -notify=command:"curl -s http://$HAPROXY_WRAPPER_CONTROL/reload"
