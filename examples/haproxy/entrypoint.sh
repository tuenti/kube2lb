#!/bin/bash

CONFFILE=/usr/local/etc/haproxy/haproxy.cfg
PIDFILE=/var/run/haproxy.pid

_term() {
	pkill -SIGTERM haproxy
	pkill kube2lb
	exit 0
}

trap _term SIGTERM SIGINT

echo "
global
	daemon

frontend nothing
	bind :80
" > $CONFFILE

sed -i $TEMPLATE \
	-e "s/__HAPROXY_STATS_BIND__/$HAPROXY_STATS_BIND/"

haproxy -f $CONFFILE -p $PIDFILE
kube2lb -apiserver="$APISERVER" -kubecfg="$KUBECFG" -template="$TEMPLATE" -server-name-templates="$SERVER_NAME_TEMPLATES" -config="$CONFFILE" -domain="$DOMAIN" -notify=command:"haproxy -f $CONFFILE -p $PIDFILE -sf \$(cat $PIDFILE)" &
wait $!
