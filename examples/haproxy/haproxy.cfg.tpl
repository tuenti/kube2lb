{{ $nodes := .Nodes }}
{{ $services := .Services }}
{{ $domain := .Domain }}
{{ $ports := .Ports }}

global
	log 127.0.0.1   local0
	log 127.0.0.1   local1 notice
	maxconn 4096
	daemon

defaults
	log        global
	mode       http
	option     httplog
	option     dontlognull
	option     forwardfor
	http-reuse aggressive

{{ range $i, $port := $ports }}
frontend frontend_{{ $port }}
	bind *:{{ $port }}
{{ range $i, $service := $services }}
{{ if eq $service.Port $port }}
{{ $label := printf "%s_%s_%d" $service.Name $service.Namespace $service.Port }}
	acl svc_{{ $label }} hdr(host) -i {{ $service.Name }}.{{ $service.Namespace }}.svc.{{ $domain }}
	use_backend backend_{{ $label }} if svc_{{ $label }}
{{ end }}
{{ end }}
{{ end }}

{{ range $i, $service := $services }}
{{ $label := printf "%s_%s_%d" $service.Name $service.Namespace $service.Port }}
backend backend_{{ $label }}
	balance leastconn
	option httpclose
	{{ range $i, $node := $nodes }}
	server node{{ $i }} {{ $node }}:{{ $service.NodePort }} check{{ end }}
{{ end }}
