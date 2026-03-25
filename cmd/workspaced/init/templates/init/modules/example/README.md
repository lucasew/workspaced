# Example Module

This is an example workspaced module demonstrating how to:
- Use conditionals in templates
- Access system information (hostname, IPs)
- Detect environment (phone vs desktop)
- Loop over config data

## Usage

Enable in `settings.toml`:

```toml
[modules.example]
enable = true
greeting = "Hello from workspaced!"
show_hostname = true
show_ips = true
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
2. Edit `module.toml` with your metadata
3. Define config schema in `defaults.toml` and `schema.json`
4. Optionally mirror the same metadata and config defaults in `module.cue`
5. Create templates in `home/`, `etc/`, or another preset directory
6. Enable in `settings.toml`

## Experimental CUE Format

This module also ships with a `module.cue` file as an experiment for a future
module format. It is not used by workspaced yet, and exists only as a parallel
definition of:

- module metadata
- config fields
- default values

For now, the legacy files remain the source of truth:

- `module.toml`
- `defaults.toml`
- `schema.json`
