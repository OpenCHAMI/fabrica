# fru-service-test



## Getting Started

1. Define your resources in pkg/resources/
2. Generate code: fabrica generate
3. Run the server: go run cmd/server/main.go

## Configuration

The server supports configuration via:
- Command line flags
- Environment variables (FRU-SERVICE-TEST_*)
- Configuration file (~/.fru-service-test.yaml)

## Features

- 💾 File-based storage

## Development

```bash
# Install dependencies
go mod tidy

# Run the server
go run cmd/server/main.go serve

# Run with custom config
go run cmd/server/main.go serve --config config.yaml
```
