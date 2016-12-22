#!/bin/bash

CONFFILE=/usr/local/etc/haproxy/haproxy.cfg

echo "
global
	daemon

frontend nothing
	bind :80
" > $CONFFILE

sed -i $TEMPLATE \
	-e "s/__HAPROXY_STATS_BIND__/$HAPROXY_STATS_BIND/" \
	-e "s/__HAPROXY_MAXCONN__/$HAPROXY_MAXCONN/" \
	-e "s/__SYSLOG__/$SYSLOG/"

exec kube2lb -apiserver="$APISERVER" -kubecfg="$KUBECFG" -template="$TEMPLATE" -server-name-templates="$SERVER_NAME_TEMPLATES" -config="$CONFFILE" -domain="$DOMAIN" -notify=command:"curl -s http://$HAPROXY_WRAPPER_CONTROL/reload"
