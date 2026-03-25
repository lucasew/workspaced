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
{{ .Example.Greeting }}       // Module config
{{ .Hostname }}               // System hostname
{{ .LocalIPs }}               // Array of local IPs
```

### Conditionals
```go
{{- if .IsPhone }}
  // Termux-specific config
{{- else }}
  // Desktop config
{{- end }}
```

### Loops
```go
{{- range .Hosts }}
  Host {{ .Name }}: {{ .IP }}
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
