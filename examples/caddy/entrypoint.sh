#!/bin/bash

PIDFILE=/var/run/caddy.pid
CONFFILE=/etc/caddy/Caddyfile

_term() {
	pkill -SIGQUIT caddy
	pkill kube2lb
	exit 0
}

trap _term SIGTERM SIGINT

caddy -conf=$CONFFILE -pidfile=$PIDFILE -log=stdout &
kube2lb -apiserver="$APISERVER" -kubecfg="$KUBECFG" -template="$TEMPLATE" -server-name-templates="$SERVER_NAME_TEMPLATES" -config="$CONFFILE" -domain="$DOMAIN" -notify="pidfile:SIGUSR1:$PIDFILE" &
wait $!
