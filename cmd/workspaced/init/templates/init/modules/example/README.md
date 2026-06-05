# Example Module

This is an example workspaced module demonstrating how to:
- Use conditionals in templates
- Access system information (hostname, IPs)
- Detect environment (phone vs desktop)
- Loop over config data

## Usage

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

## Generated Files

- `~/.config/example/welcome.txt` - Example file with system info

## Template Features

The template demonstrates:

### Variables
```go
{{ .module.greeting }}        // Module config
{{ .runtime.hostname }}       // System hostname
{{ .root.hosts }}             // Map of configured hosts
```

### Conditionals
```go
{{- if .runtime.is_phone }}
  // Termux-specific config
{{- else }}
  // Desktop config
{{- end }}
```

### Loops
```go
{{- range $name, $host := .root.hosts }}
  Host {{ $name }}: {{ index $host.ips 0 }}
{{- end }}
```

### Functions
```go
{{ .List | toJSON }}          // Convert to JSON
{{ index .Array 0 }}          // Access array element
```

## Creating Your Own Module

1. Copy this module as a template
2. Edit `module.cue` with your metadata and config schema
3. Create templates in `home/`, `etc/`, or another preset directory
4. Enable in `workspaced.cue`
