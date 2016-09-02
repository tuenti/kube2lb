{{ $nodes := .Nodes -}}
{{ $services := .Services -}}
{{ $domain := .Domain -}}
{{ $ports := .Ports -}}
global
	log 127.0.0.1   local0
	log 127.0.0.1   local1 notice
	maxconn 65536
	daemon

defaults
	log        global
	mode       http
	option     httplog
	option     dontlognull
	option     forwardfor
	http-reuse aggressive
	retries    3
	option     redispatch
	timeout connect 3s
	timeout client  200s
	timeout client  30s
	timeout server  200s
	timeout tunnel  1h

frontend stats-frontend
	mode http
	bind __HAPROXY_STATS_BIND__
	default_backend stats-backend

backend stats-backend
	mode http
	stats enable
	stats show-node
	stats refresh 60s
	stats uri /

{{ range $i, $port := $ports }}
frontend frontend_{{ $port.String }}
	bind *:{{ $port.Port }}
{{- range $i, $service := $services }}
{{- if eq $service.Port.String $port.String }}
{{- $label := printf "%s_%s_%s" $service.Name $service.Namespace $service.Port }}
{{- if eq $port.Mode "http" }}
	{{ range $serverName := ServerNames $service $domain }}
	{{- if $serverName.IsRegexp }}
	acl svc_{{ $label }} hdr_reg(host) {{ $serverName.Regexp }}
	{{- else }}
	acl svc_{{ $label }} hdr(host) -i {{ $serverName }}
	{{- end }}
	{{- end }}
	use_backend backend_{{ $label }} if svc_{{ $label }}
{{- end }}
{{- if eq $port.Mode "tcp" }}
	mode tcp

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
	option http-server-close
{{- end }}
{{- if eq $service.Port.Mode "tcp" }}
	mode tcp
{{- end }}
	{{ range $i, $node := $nodes }}
	server {{ EscapeNode $node }} {{ $node }}:{{ $service.NodePort }} check{{ end }}
{{ end }}
