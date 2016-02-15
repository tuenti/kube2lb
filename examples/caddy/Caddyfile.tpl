{{ $nodes := .Nodes }}
{{ $services := .Services }}
{{ $domain := .Domain }}
{{ range $i, $service := $services }}
http://{{ $service.Name }}:{{ $service.Port }}, http://{{ $service.Name }}.{{ $service.Namespace }}:{{ $service.Port }}, http://{{ $service.Name }}.{{ $service.Namespace }}.svc:{{ $service.Port }}, http://{{ $service.Name }}.{{ $service.Namespace }}.svc.{{ $domain }}:{{ $service.Port }} {
	proxy /{{ range $i, $node := $nodes }} {{ $node }}:{{ $service.NodePort }}{{ end }} {
		policy least_conn
		proxy_header Host {host}
	}
}
{{ end }}
