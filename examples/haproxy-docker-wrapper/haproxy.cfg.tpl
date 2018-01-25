{{ $services := .Services -}}
{{ $domain := .Domain -}}
{{ $ports := .Ports -}}
{{ $nbproc := __HAPROXY_NBPROC__ -}}
{{ $nbthread := __HAPROXY_NBTHREAD__ -}}
global
	log __SYSLOG__   local0 notice
	maxconn __HAPROXY_MAXCONN__
	nbproc {{ $nbproc }}
	nbthread {{ $nbthread }}
{{- range $i, $cpu := IntRange $nbproc 1 $nbthread }}
	cpu-map auto:{{ Add $i 1 }}/1-{{ $nbthread }} {{ $cpu }}-{{ Add $cpu $nbthread -1 }}
	stats socket /var/lib/haproxy/socket{{ Add $i 1 }} process {{ Add $i 1 }} mode 600 level admin
{{- end }}


defaults
	log        global
	mode       http
	option     dontlognull
	http-reuse __HAPROXY_HTTP_REUSE__
	retries    3
	option     redispatch
	timeout connect __HAPROXY_TIMEOUT_CONNECT__
	timeout client  __HAPROXY_TIMEOUT_CLIENT__
	timeout server  __HAPROXY_TIMEOUT_SERVER__
	timeout http-keep-alive __HAPROXY_TIMEOUT_KEEPALIVE__
	timeout tunnel  __HAPROXY_TIMEOUT_TUNNEL__

{{ range $i, $port := $ports }}
frontend frontend_{{ $port }}
	bind {{ $port.IP }}:{{ $port.Port }}
	maxconn __HAPROXY_FRONTEND_MAXCONN__
{{- if eq $port.Mode "http" }}
	option httplog
	option forwardfor if-none
{{- end }}
{{- range $i, $service := $services }}
{{- if eq $service.Port.String $port.String }}
{{- if eq $port.Mode "http" }}
	{{ range $serverName := ServerNames $service $domain }}
	{{- if $serverName.IsRegexp }}
	acl svc_{{ $service }} hdr_reg(host) {{ $serverName.Regexp }}
	{{- else }}
	acl svc_{{ $service }} hdr_dom(host) -i {{ $serverName }}
	{{- end }}
	{{- end }}
	use_backend backend_{{ $service }} if svc_{{ $service }}
{{- end }}
{{- if eq $port.Mode "tcp" }}
	mode   tcp
	option tcplog

	use_backend backend_{{ $service }}
{{- end }}
{{- end }}
{{- end }}
{{ end }}

{{- range $i, $service := $services }}
backend backend_{{ $service }}
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
