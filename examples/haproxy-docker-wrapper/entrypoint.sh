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
	-e "s/__SYSLOG__/$SYSLOG/"

exec kube2lb -apiserver="$APISERVER" -kubecfg="$KUBECFG" -template="$TEMPLATE" -server-name-templates="$SERVER_NAME_TEMPLATES" -config="$CONFFILE" -domain="$DOMAIN" -notify=command:"curl -s http://$HAPROXY_WRAPPER_CONTROL/reload"
