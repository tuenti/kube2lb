{{ $nodes := .Nodes }}
{{ $services := .Services }}
{{ $domain := .Domain }}
{{ range $i, $service := $services }}
{{ range $j, $serverName := ServerNames $service $domain }}
http://{{ $serverName }}:{{ $service.Port }} {
	log / stdout "{host} {remote} - [{when}] \"{method} {path} {proto}\" {status} {size} \"{>Referer}\" \"{>User-Agent}\" \"{latency}\""
	proxy /{{ range $i, $node := $nodes }} {{ $node }}:{{ $service.NodePort }}{{ end }} {
		policy least_conn
		proxy_header Host {host}
	}
}

{{ end }}
{{ end }}
