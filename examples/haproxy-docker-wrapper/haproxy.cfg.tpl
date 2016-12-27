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
	maxconn __HAPROXY_MAXCONN__

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
	{{ range $i, $endpoint := $service.Endpoints }}
	server {{ EscapeNode $endpoint.Name }} {{ $endpoint }} check inter 5s downinter 10s slowstart 60s{{ end }}
{{ end }}
