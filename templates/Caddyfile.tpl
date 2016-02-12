{{ $nodes := .Nodes }}
{{ $services := .Services }}
{{ range $i, $service := $services }}
http://{{ $service.ServiceName }}:{{ $service.Port }} {
	proxy /{{ range $i, $node := $nodes }} {{ $node }}:{{ $service.NodePort }}{{ end }} {
		policy least_conn
		proxy_header Host {host}
	}
}
{{ end }}
