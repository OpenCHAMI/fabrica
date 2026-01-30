<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 11: Node Service Shim with Profiles

**Time to complete:** ~30-40 minutes
**Difficulty:** Advanced
**Prerequisites:** Go 1.23+, fabrica CLI built locally, SQLite3

## What You'll Build

A node-service (shim) that exposes a Node API composed from inventory + boot + metadata concepts and introduces profile bindings without using SMD groups for config intent.

This example demonstrates:
- **Profiles** as a first-class selector (`?profile=` semantics)
- **NodeSet** resolution from labels/xnames
- **ProfileBinding** to bind nodes or nodesets to profiles
- **Ent (SQLite) storage**
- **Reconciliation** to compute resolved node lists and effective profile
- **Generated CLI** used to create/update/list/delete resources

## Step-by-Step Guide

### Step 1: Initialize a Project with Local Fabrica

```bash
# From a local Fabrica checkout
FABRICA_BIN=/path/to/fabrica/bin/fabrica

$FABRICA_BIN init node-service \
  --module github.com/openchami/node-service \
  --storage-type ent \
  --storage \
  --db sqlite \
  --events \
  --events-bus memory \
  --reconcile

cd node-service
```

### Step 2: Use Local Fabrica Library

```bash
FABRICA_REPO=/path/to/fabrica
go mod edit -replace github.com/openchami/fabrica=$FABRICA_REPO
go mod tidy
```

### Step 3: Add Resources

```bash
$FABRICA_BIN add resource Node
$FABRICA_BIN add resource NodeSet
$FABRICA_BIN add resource ProfileBinding
```

### Step 4: Copy Reference Resource Definitions

```bash
FABRICA_REPO=/path/to/fabrica
cp -R "$FABRICA_REPO/examples/11-node-service/apis/example.fabrica.dev/v1/." apis/example.fabrica.dev/v1/
```

### Step 5: Generate Code (Server + Client)

```bash
$FABRICA_BIN generate --client
go mod tidy
```

### Step 6: Add Reconciliation Logic (Reference)

```bash
FABRICA_REPO=/path/to/fabrica
cp -R "$FABRICA_REPO/examples/11-node-service/pkg/reconcilers/." pkg/reconcilers/

# Remove build tags from copied files
sed -i.bak '1{/^\/\/go:build ignore$/d;}' pkg/reconcilers/*_reconciler.go
rm -f pkg/reconcilers/*.bak
```

### Step 7: Enable Reconciliation in main.go

Uncomment the reconciliation controller setup in `cmd/server/main.go` (created by `fabrica init --reconcile`). Ensure the controller is started after the event bus is ready.

### Step 8: Build and Run

```bash
mkdir -p data
go build -o node-server ./cmd/server

# Ent/SQLite requires a database URL
./node-server serve --database-url "file:./data.db?cache=shared&_fk=1"
```

## Test with the Generated Client

```bash
go build -o nodectl ./cmd/client

# Create nodes
./nodectl node create --spec '{
  "metadata": {"name": "x3000c0s1b0n0"},
  "spec": {
    "xname": "x3000c0s1b0n0",
    "role": "compute",
    "labels": {"rack": "r1", "pool": "blue"}
  }
}' --output json

./nodectl node create --spec '{
  "metadata": {"name": "x3000c0s1b0n1"},
  "spec": {
    "xname": "x3000c0s1b0n1",
    "role": "compute",
    "labels": {"rack": "r1", "pool": "blue"}
  }
}' --output json

# Create a NodeSet
./nodectl nodeset create --spec '{
  "metadata": {"name": "blue-compute"},
  "spec": {
    "selector": {
      "labels": {"pool": "blue"}
    }
  }
}' --output json

# Bind the NodeSet to a profile
./nodectl profilebinding create --spec '{
  "metadata": {"name": "blue-profile"},
  "spec": {
    "profile": "blue",
    "target": {"nodeSetUID": "<nodeset-uid>"},
    "bootProfile": "compute-default",
    "configGroups": ["base", "compute", "blue"]
  }
}' --output json

# List nodes and bindings
./nodectl node list --output json
./nodectl profilebinding list --output json

# Update a node
./nodectl node update <node-uid> --spec '{"role": "compute", "labels": {"pool": "green"}}' --output json

# Delete a binding
./nodectl profilebinding delete <binding-uid>
```

## Notes

- This example keeps all data in Ent/SQLite for easy local development.
- The reconciliation logic resolves NodeSets from labels/xnames and applies ProfileBindings by updating node status fields.
- In production, NodeSet resolution should integrate with SMD; ProfileBindings should write-through to metadata-service and boot-service.

## Files in This Example

- `apis/example.fabrica.dev/v1/node_types.go`
- `apis/example.fabrica.dev/v1/nodeset_types.go`
- `apis/example.fabrica.dev/v1/profilebinding_types.go`
- `pkg/reconcilers/nodeset_reconciler.go` (reference)
- `pkg/reconcilers/profilebinding_reconciler.go` (reference)
