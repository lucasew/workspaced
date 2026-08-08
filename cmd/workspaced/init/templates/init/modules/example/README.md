# example module

Starter module: conditionals, hostname/IPs, phone vs desktop, looping config.

Enable in `workspaced.cue`:

```cue
workspaced: modules: example: {
	input: "self"
	path: "modules/example"
	config: {
		enable: true
		greeting: "Hello from workspaced!"
		show_hostname: true
		show_ips: true
	}
}
```

Writes `~/.config/example/welcome.txt`.

## Template bits

```go
{{ .module.greeting }}
{{ .runtime.hostname }}
{{ .root.hosts }}

{{- if .runtime.is_phone }}
  // Termux
{{- else }}
  // desktop
{{- end }}

{{- range $name, $host := .root.hosts }}
  Host {{ $name }}: {{ index $host.ips 0 }}
{{- end }}

{{ .List | toJSON }}
{{ index .Array 0 }}
```

## Your own module

1. Copy this directory.
2. Edit `module.cue` (meta + config schema).
3. Put templates under `home/`, `etc/`, or another preset root.
4. Point `workspaced.cue` at it.
