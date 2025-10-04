<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Getting Started with Fabrica

> Build your first resource-based REST API in 10 minutes with automatic code generation.

## Table of Contents

- [Installation](#installation)
- [Prerequisites](#prerequisites)
- [Your First Resource](#your-first-resource)
- [Generate Code](#generate-code)
- [Run the Server](#run-the-server)
- [Use the API](#use-the-api)
- [Complete Example](#complete-example)
- [Next Steps](#next-steps)

## Installation

### Install Fabrica

```bash
go get github.com/alexlovelltroy/fabrica
```

### Create a New Project

```bash
mkdir myapi
cd myapi
go mod init github.com/yourname/myapi
```

### Project Structure

Create the following directory structure:

```
myapi/
├── go.mod
├── cmd/
│   ├── codegen/          # Code generator
│   │   └── main.go
│   └── server/           # API server
│       └── main.go
├── pkg/
│   └── resources/        # Resource definitions
│       └── device/
│           └── device.go
└── internal/
    └── storage/          # Generated storage (auto-created)
```

## Prerequisites

**Go Version**: 1.23 or later

**Basic Knowledge:**
- Go programming basics
- REST API concepts
- JSON format

**No prior experience needed with:**
- Code generation
- Kubernetes patterns
- Complex API frameworks

## Your First Resource

Let's create a simple Device resource for an IoT platform.

### Step 1: Define the Resource

Create `pkg/resources/device/device.go`:

```go
package device

import "github.com/alexlovelltroy/fabrica/pkg/resource"

// Device represents an IoT device in your system
type Device struct {
    resource.Resource
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status,omitempty"`
}

// DeviceSpec defines the desired state
type DeviceSpec struct {
    Name        string            `json:"name"`
    Type        string            `json:"type"`        // "sensor", "actuator", "controller"
    Location    string            `json:"location"`
    Model       string            `json:"model"`
    Properties  map[string]string `json:"properties,omitempty"`
}

// DeviceStatus defines the observed state
type DeviceStatus struct {
    Online       bool   `json:"online"`
    LastSeen     string `json:"lastSeen,omitempty"`
    IPAddress    string `json:"ipAddress,omitempty"`
    FirmwareVer  string `json:"firmwareVersion,omitempty"`
    BatteryLevel int    `json:"batteryLevel,omitempty"` // 0-100
}

// Register UID prefix for devices
func init() {
    resource.RegisterResourcePrefix("Device", "dev")
}
```

**What's happening here?**

1. **Device struct** embeds `resource.Resource` - gives you APIVersion, Kind, Metadata for free
2. **Spec** defines what you want (desired state)
3. **Status** defines what you observe (observed state)
4. **UID prefix** registered as "dev" - generates IDs like `dev-1a2b3c4d`

### Step 2: Understanding the Resource Structure

When you create a Device, it will look like this:

```json
{
  "apiVersion": "v1",
  "kind": "Device",
  "metadata": {
    "uid": "dev-1a2b3c4d",
    "name": "temperature-sensor-01",
    "labels": {
      "location": "warehouse-a",
      "type": "sensor"
    },
    "annotations": {
      "description": "Temperature sensor for cold storage"
    },
    "createdAt": "2024-10-03T10:00:00Z",
    "updatedAt": "2024-10-03T10:00:00Z"
  },
  "spec": {
    "name": "Temperature Sensor 01",
    "type": "sensor",
    "location": "Warehouse A, Aisle 3",
    "model": "TMP-100",
    "properties": {
      "samplingRate": "60s",
      "accuracy": "±0.5°C"
    }
  },
  "status": {
    "online": true,
    "lastSeen": "2024-10-03T10:15:00Z",
    "ipAddress": "192.168.1.100",
    "firmwareVersion": "1.2.3",
    "batteryLevel": 85
  }
}
```

## Generate Code

### Step 3: Create Code Generator

Create `cmd/codegen/main.go`:

```go
package main

import (
    "fmt"
    "log"

    "github.com/alexlovelltroy/fabrica/pkg/codegen"
    "github.com/yourname/myapi/pkg/resources/device"
)

func main() {
    // Create generator for server code
    serverGen := codegen.NewGenerator(
        "cmd/server",           // Output directory
        "main",                 // Package name
        "github.com/yourname/myapi", // Module path
    )

    // Register your resource
    if err := serverGen.RegisterResource(&device.Device{}); err != nil {
        log.Fatalf("Failed to register Device: %v", err)
    }

    // Generate all server code
    fmt.Println("Generating server code...")
    if err := serverGen.GenerateAll(); err != nil {
        log.Fatalf("Failed to generate server code: %v", err)
    }

    fmt.Println("✅ Code generation complete!")
    fmt.Println("\nGenerated files:")
    fmt.Println("  - cmd/server/device_handlers_generated.go")
    fmt.Println("  - cmd/server/models_generated.go")
    fmt.Println("  - cmd/server/routes_generated.go")
    fmt.Println("  - internal/storage/storage_generated.go")
}
```

### Step 4: Run Code Generation

```bash
go run cmd/codegen/main.go
```

**Output:**
```
Generating server code...
✅ Code generation complete!

Generated files:
  - cmd/server/device_handlers_generated.go
  - cmd/server/models_generated.go
  - cmd/server/routes_generated.go
  - internal/storage/storage_generated.go
```

**What got generated?**

- **Handlers**: CRUD endpoints (List, Get, Create, Update, Delete)
- **Models**: Request/response types
- **Routes**: URL routing configuration
- **Storage**: File-based persistence operations

## Run the Server

### Step 5: Create Server Main

Create `cmd/server/main.go`:

```go
package main

import (
    "fmt"
    "log"
    "net/http"

    "github.com/alexlovelltroy/fabrica/pkg/storage"
)

func main() {
    // Create file-based storage backend
    backend := storage.NewFileBackend("./data")
    defer backend.Close()

    // Register routes (generated function)
    RegisterRoutes(backend)

    // Start server
    addr := ":8080"
    fmt.Printf("🚀 Server starting on http://localhost%s\n", addr)
    fmt.Println("\nAvailable endpoints:")
    fmt.Println("  GET    /devices           - List all devices")
    fmt.Println("  GET    /devices/{uid}     - Get specific device")
    fmt.Println("  POST   /devices           - Create new device")
    fmt.Println("  PUT    /devices/{uid}     - Update device")
    fmt.Println("  DELETE /devices/{uid}     - Delete device")
    fmt.Println()

    if err := http.ListenAndServe(addr, nil); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
```

### Step 6: Start the Server

```bash
go run cmd/server/*.go
```

**Output:**
```
🚀 Server starting on http://localhost:8080

Available endpoints:
  GET    /devices           - List all devices
  GET    /devices/{uid}     - Get specific device
  POST   /devices           - Create new device
  PUT    /devices/{uid}     - Update device
  DELETE /devices/{uid}     - Delete device
```

🎉 **Your API is now running!**

## Use the API

### Step 7: Create a Device

```bash
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "v1",
    "kind": "Device",
    "metadata": {
      "name": "temp-sensor-01",
      "labels": {
        "location": "warehouse-a",
        "type": "sensor"
      }
    },
    "spec": {
      "name": "Temperature Sensor 01",
      "type": "sensor",
      "location": "Warehouse A",
      "model": "TMP-100"
    }
  }'
```

**Response:**
```json
{
  "apiVersion": "v1",
  "kind": "Device",
  "metadata": {
    "uid": "dev-a1b2c3d4",
    "name": "temp-sensor-01",
    "labels": {
      "location": "warehouse-a",
      "type": "sensor"
    },
    "createdAt": "2024-10-03T10:00:00Z",
    "updatedAt": "2024-10-03T10:00:00Z"
  },
  "spec": {
    "name": "Temperature Sensor 01",
    "type": "sensor",
    "location": "Warehouse A",
    "model": "TMP-100"
  }
}
```

**Note the UID**: `dev-a1b2c3d4` was automatically generated!

### Step 8: List All Devices

```bash
curl http://localhost:8080/devices
```

**Response:**
```json
[
  {
    "apiVersion": "v1",
    "kind": "Device",
    "metadata": {
      "uid": "dev-a1b2c3d4",
      "name": "temp-sensor-01",
      ...
    },
    "spec": {
      "name": "Temperature Sensor 01",
      ...
    }
  }
]
```

### Step 9: Get Specific Device

```bash
curl http://localhost:8080/devices/dev-a1b2c3d4
```

### Step 10: Update Device

```bash
curl -X PUT http://localhost:8080/devices/dev-a1b2c3d4 \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "v1",
    "kind": "Device",
    "metadata": {
      "uid": "dev-a1b2c3d4",
      "name": "temp-sensor-01"
    },
    "spec": {
      "name": "Temperature Sensor 01",
      "type": "sensor",
      "location": "Warehouse A",
      "model": "TMP-100"
    },
    "status": {
      "online": true,
      "ipAddress": "192.168.1.100",
      "firmwareVersion": "1.2.3"
    }
  }'
```

### Step 11: Delete Device

```bash
curl -X DELETE http://localhost:8080/devices/dev-a1b2c3d4
```

## Complete Example

### Full Working Project

Here's a complete, ready-to-run example:

**Directory Structure:**
```
myapi/
├── go.mod
├── cmd/
│   ├── codegen/main.go
│   └── server/main.go
└── pkg/
    └── resources/
        └── device/device.go
```

**go.mod:**
```go
module github.com/yourname/myapi

go 1.23

require github.com/alexlovelltroy/fabrica v0.2.0
```

**Run it:**
```bash
# 1. Generate code
go run cmd/codegen/main.go

# 2. Start server
go run cmd/server/*.go

# 3. Test API (in another terminal)
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{"apiVersion":"v1","kind":"Device","metadata":{"name":"test"},"spec":{"name":"Test Device","type":"sensor","location":"Lab","model":"TEST-1"}}'
```

## Next Steps

### 🎓 Learn More

Now that you have a working API, explore these topics:

1. **[Add Labels and Annotations](resource-model.md#labels-and-annotations)** - Query and organize resources
2. **[Implement Authorization](policy.md)** - Add access control
3. **[Customize Templates](codegen.md#customizing-templates)** - Modify generated code
4. **[Add More Resources](resource-model.md#defining-resources)** - Build a complete API

### 📚 Deep Dives

- **[Resource Model](resource-model.md)** - Understand the full resource structure
- **[Storage System](storage.md)** - Learn about storage backends
- **[Code Generation](codegen.md)** - Master the template system
- **[Architecture](architecture.md)** - Framework design and principles

### 🎯 Common Next Steps

#### Add Another Resource

Create `pkg/resources/sensor/sensor.go`:

```go
package sensor

import "github.com/alexlovelltroy/fabrica/pkg/resource"

type Sensor struct {
    resource.Resource
    Spec   SensorSpec   `json:"spec"`
    Status SensorStatus `json:"status,omitempty"`
}

type SensorSpec struct {
    DeviceUID string `json:"deviceUid"`
    Type      string `json:"type"` // "temperature", "humidity", "pressure"
    Unit      string `json:"unit"`
    Threshold float64 `json:"threshold"`
}

type SensorStatus struct {
    CurrentValue float64 `json:"currentValue"`
    LastReading  string  `json:"lastReading"`
}

func init() {
    resource.RegisterResourcePrefix("Sensor", "sen")
}
```

Register in code generator:

```go
serverGen.RegisterResource(&sensor.Sensor{})
```

Regenerate and restart!

#### Add Query Parameters

Edit the generated handlers to add filtering:

```go
// In cmd/server/device_handlers_generated.go (after regenerating, edit your template)
func ListDevices(w http.ResponseWriter, r *http.Request) {
    devices, err := storage.LoadAllDevices(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Add filtering by label
    location := r.URL.Query().Get("location")
    if location != "" {
        filtered := []Device{}
        for _, d := range devices {
            if d.Metadata.Labels["location"] == location {
                filtered = append(filtered, d)
            }
        }
        devices = filtered
    }

    json.NewEncoder(w).Encode(devices)
}
```

Usage:
```bash
curl http://localhost:8080/devices?location=warehouse-a
```

## Troubleshooting

### Common Issues

**Code generation fails:**
```
Error: failed to register Device: ...
```
**Solution**: Ensure Device struct embeds `resource.Resource` and has json tags

**Server won't start:**
```
Error: address already in use
```
**Solution**: Another process is using port 8080. Change port in main.go or kill the other process

**404 errors:**
```
404 page not found
```
**Solution**: Make sure you called `RegisterRoutes(backend)` in main.go

**Storage errors:**
```
Error: failed to create storage directory
```
**Solution**: Check file permissions. Server needs write access to ./data directory

### Getting Help

- **[GitHub Issues](https://github.com/alexlovelltroy/fabrica/issues)** - Report bugs
- **[Discussions](https://github.com/alexlovelltroy/fabrica/discussions)** - Ask questions
- **[Examples](examples.md)** - See more complete examples

## Summary

In this guide, you:

- ✅ Installed Fabrica
- ✅ Created your first resource
- ✅ Generated REST API code
- ✅ Ran an API server
- ✅ Performed CRUD operations

**Time spent**: ~10 minutes
**Code written**: ~100 lines
**APIs generated**: Complete CRUD with storage ✨

### What You Got

From ~100 lines of code, you got:

- 🌐 Full REST API with 5 endpoints
- 💾 File-based persistence
- 🔑 Structured UID generation
- 🏷️ Label and annotation support
- ⏰ Automatic timestamps
- 📦 Type-safe operations

### Where to Go Next

**To build production APIs:**
- Add [Authorization](policy.md) for access control
- Configure [Storage](storage.md) for your database
- Support [Multiple Versions](versioning.md) for compatibility
- Customize [Code Generation](codegen.md) for your needs

**To learn the framework:**
- Understand [Architecture](architecture.md) and design principles
- Master the [Resource Model](resource-model.md)
- Explore [Examples](examples.md) of real-world applications

---

**Ready for more?** → [Resource Model Guide](resource-model.md)
**Need help?** → [GitHub Discussions](https://github.com/alexlovelltroy/fabrica/discussions)
**Want to contribute?** → [Contributing Guide](../CONTRIBUTING.md)
