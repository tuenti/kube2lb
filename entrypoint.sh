#!/bin/bash

PIDFILE=/var/run/caddy.pid
CONFFILE=/etc/caddy/Caddyfile

caddy -conf=$CONFFILE -pidfile=$PIDFILE -log=stdout &

kube2lb -apiserver=$APISERVER -template=$TEMPLATE -config=$CONFFILE -notify=pidfile:$PIDFILE
