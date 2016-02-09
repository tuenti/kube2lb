{{ $nodes := .Nodes }}
{{ $services := .ServicePorts }}
{{ range $i, $service := $services }}
log stdout
:{{ $service.Port }}, http://{{ $service.ServiceName }} {
	proxy /{{ range $i, $node := $nodes }} {{ $node }}:{{ $service.NodePort }}{{ end }} {
		policy least_conn
		proxy_header Host {host}
	}
}
{{ end }}
