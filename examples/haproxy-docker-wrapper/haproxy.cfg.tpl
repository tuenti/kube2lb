{{ $services := .Services -}}
{{ $domain := .Domain -}}
{{ $ports := .Ports -}}
global
	log __SYSLOG__   local0 notice
	maxconn __HAPROXY_MAXCONN__
	daemon
{{- if gt __HAPROXY_NBPROC__ 1 }}
	nbproc __HAPROXY_NBPROC__
{{- range $, $i := IntRange __HAPROXY_NBPROC__ 1 1 }}
	cpu-map {{ $i }} {{ $i }}
	stats socket /var/lib/haproxy/socket{{ $i }} process {{ $i }} mode 600 level admin
{{- end }}
{{- else }}
	stats socket /var/lib/haproxy/socket mode 600 level admin
{{- end }}


defaults
	log        global
	mode       http
	option     dontlognull
	http-reuse aggressive
	retries    3
	option     redispatch
	timeout connect __HAPROXY_TIMEOUT_CONNECT__
	timeout client  __HAPROXY_TIMEOUT_CLIENT__
	timeout server  __HAPROXY_TIMEOUT_SERVER__
	timeout http-keep-alive __HAPROXY_TIMEOUT_KEEPALIVE__
	timeout tunnel  __HAPROXY_TIMEOUT_TUNNEL__

{{ range $i, $port := $ports }}
frontend frontend_{{ $port.String }}
	bind *:{{ $port.Port }}
	maxconn __HAPROXY_FRONTEND_MAXCONN__
{{- if eq $port.Mode "http" }}
	option httplog
	option forwardfor if-none
{{- end }}
{{- range $i, $service := $services }}
{{- if eq $service.Port.String $port.String }}
{{- $label := printf "%s_%s_%s" $service.Name $service.Namespace $service.Port }}
{{- if eq $port.Mode "http" }}
	{{ range $serverName := ServerNames $service $domain }}
	{{- if $serverName.IsRegexp }}
	acl svc_{{ $label }} hdr_reg(host) {{ $serverName.Regexp }}
	{{- else }}
	acl svc_{{ $label }} hdr_dom(host) -i {{ $serverName }}
	{{- end }}
	{{- end }}
	use_backend backend_{{ $label }} if svc_{{ $label }}
{{- end }}
{{- if eq $port.Mode "tcp" }}
	mode   tcp
	option tcplog

	use_backend backend_{{ $label }}
{{- end }}
{{- end }}
{{- end }}
{{ end }}

{{- range $i, $service := $services }}
{{- $label := printf "%s_%s_%s" $service.Name $service.Namespace $service.Port }}
backend backend_{{ $label }}
	balance leastconn
{{- if eq $service.Port.Mode "http" }}
	option httplog
	option http-server-close
{{- end }}
{{- if eq $service.Port.Mode "tcp" }}
	mode   tcp
	option tcplog
{{- end }}
{{- if gt $service.Timeout 0 }}
	timeout server {{ $service.Timeout }}
{{- end }}
	{{ range $i, $endpoint := $service.Endpoints }}
	server {{ EscapeNode $endpoint.Name }} {{ $endpoint }} maxconn __HAPROXY_SERVER_MAXCONN__ check inter 5s downinter 10s slowstart 60s{{ end }}
{{ end }}
