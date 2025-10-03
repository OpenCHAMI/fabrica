<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica Documentation

> Complete documentation for the Fabrica framework - build resource-based REST APIs with automatic code generation.

## 📚 Documentation Map

### 🚀 Getting Started

Perfect for newcomers - get your first API running in minutes.

- **[Getting Started Guide](getting-started.md)** ⭐ - Your first resource in 10 minutes
  - Installation
  - Quick start tutorial
  - Complete working example
  - Common patterns

### 🏗️ Core Concepts

Understand the framework's architecture and design.

- **[Architecture Overview](architecture.md)** - Framework design and principles
  - System architecture
  - Design principles
  - Component overview
  - Data flow diagrams
  - Extension points

- **[Resource Model](resource-model.md)** - Understanding resources
  - Resource structure (APIVersion, Kind, Metadata, Spec, Status)
  - UID generation strategies
  - Labels and annotations
  - Resource lifecycle
  - Best practices

### 🔧 Core Systems

Learn about the main framework systems.

- **[Code Generation](codegen.md)** - Template-based code generation
  - How code generation works
  - Template system overview
  - Customizing templates
  - Template functions and variables
  - Integration into build process
  - Debugging generated code

- **[Storage System](storage.md)** - Pluggable storage backends
  - Storage backend interface
  - File backend usage
  - Implementing custom backends
  - Database patterns
  - Caching strategies
  - Performance optimization

- **[Versioning](versioning.md)** - Multi-version schema support
  - Why versioning matters
  - Schema version design
  - Version registration
  - Conversion patterns
  - Migration strategies
  - HTTP version negotiation
  - Best practices

- **[Authorization](policy.md)** - Policy framework
  - Policy framework overview
  - Implementing custom policies
  - RBAC patterns
  - ABAC patterns
  - JWT integration
  - Multi-tenancy
  - Testing policies

### 📖 Practical Guides

Real-world examples and patterns.

- **[Framework Comparison](comparison.md)** - Fabrica vs Go-Fuego vs Huma vs Goa
  - Feature comparison matrix
  - Decision tree for choosing frameworks
  - Code examples comparing approaches
  - When to use each framework
  - Integration possibilities

- **[Examples](examples.md)** - Complete example implementations
  - Simple device inventory
  - Blog CMS example
  - E-commerce product catalog
  - IoT device management
  - User management system
  - Multi-tenant SaaS application

### 🛠️ Contributing

Help make Fabrica better!

- **[Contributing Guide](../CONTRIBUTING.md)** - How to contribute
  - Development setup
  - Running tests
  - Code style guide
  - Submitting PRs
  - Release process

## 📖 Quick Navigation

### By Experience Level

**Beginners** (New to Fabrica):
1. [Getting Started Guide](getting-started.md) - Start here!
2. [Resource Model](resource-model.md) - Understand resources
3. [Examples](examples.md) - See complete implementations

**Intermediate** (Building real applications):
1. [Code Generation](codegen.md) - Customize generated code
2. [Storage System](storage.md) - Choose storage backend
3. [Authorization](policy.md) - Add access control

**Advanced** (Extending the framework):
1. [Architecture Overview](architecture.md) - Deep dive into design
2. [Versioning](versioning.md) - Multi-version APIs
3. [Contributing Guide](../CONTRIBUTING.md) - Contribute back

### By Task

**I want to...**

- **Get started quickly** → [Getting Started Guide](getting-started.md)
- **Understand the framework** → [Architecture Overview](architecture.md)
- **Define resources** → [Resource Model](resource-model.md)
- **Generate code** → [Code Generation Guide](codegen.md)
- **Store data** → [Storage System Guide](storage.md)
- **Add authentication** → [Authorization Guide](policy.md)
- **Support multiple versions** → [Versioning Guide](versioning.md)
- **See examples** → [Examples](examples.md)
- **Contribute** → [Contributing Guide](../CONTRIBUTING.md)

## 🎯 Learning Paths

### Path 1: Build Your First API (30 minutes)

1. **[Getting Started - Installation](getting-started.md#installation)** (5 min)
2. **[Getting Started - Your First Resource](getting-started.md#your-first-resource)** (10 min)
3. **[Getting Started - Generate Code](getting-started.md#generate-code)** (5 min)
4. **[Getting Started - Run the Server](getting-started.md#run-the-server)** (5 min)
5. **[Getting Started - Test the API](getting-started.md#test-the-api)** (5 min)

**Outcome**: Working REST API with CRUD operations ✅

### Path 2: Production-Ready API (2 hours)

1. **Complete Path 1** (30 min)
2. **[Resource Model - Labels and Annotations](resource-model.md#labels-and-annotations)** (15 min)
3. **[Authorization - RBAC Setup](policy.md#rbac-patterns)** (30 min)
4. **[Storage - File Backend Configuration](storage.md#file-backend)** (15 min)
5. **[Code Generation - Custom Templates](codegen.md#customizing-templates)** (30 min)

**Outcome**: Production-ready API with auth and persistence ✅

### Path 3: Multi-Version API (3 hours)

1. **Complete Path 2** (2.5 hours)
2. **[Versioning - Understanding Versions](versioning.md#why-versioning-matters)** (15 min)
3. **[Versioning - Register Versions](versioning.md#version-registration)** (15 min)
4. **[Versioning - Version Conversion](versioning.md#conversion-patterns)** (30 min)

**Outcome**: API supporting multiple versions with automatic conversion ✅

## 📚 Reference Documentation

### API Reference

- **[Go Package Docs](https://pkg.go.dev/github.com/alexlovelltroy/fabrica)** - Complete API reference
- **[Template Reference](../templates/README.md)** - Code generation templates

### Package Documentation

Core packages:
- [`pkg/resource`](https://pkg.go.dev/github.com/alexlovelltroy/fabrica/pkg/resource) - Resource model
- [`pkg/codegen`](https://pkg.go.dev/github.com/alexlovelltroy/fabrica/pkg/codegen) - Code generation
- [`pkg/storage`](https://pkg.go.dev/github.com/alexlovelltroy/fabrica/pkg/storage) - Storage backends
- [`pkg/policy`](https://pkg.go.dev/github.com/alexlovelltroy/fabrica/pkg/policy) - Authorization
- [`pkg/versioning`](https://pkg.go.dev/github.com/alexlovelltroy/fabrica/pkg/versioning) - Multi-version support

## 🎓 Tutorials

### Quick Tutorials (5-15 minutes each)

- [Create Your First Resource](getting-started.md#your-first-resource)
- [Add Labels and Annotations](resource-model.md#using-labels)
- [Generate Client Code](codegen.md#client-generation)
- [Implement a Policy](policy.md#implementing-policies)
- [Configure File Storage](storage.md#file-backend-configuration)

### In-Depth Tutorials (30-60 minutes each)

- [Build a Complete IoT Platform](examples.md#iot-device-management)
- [Create a Multi-Tenant SaaS App](examples.md#multi-tenant-saas)
- [Build a CMS with Versioning](examples.md#blog-cms)
- [Implement Custom Storage Backend](storage.md#implementing-custom-backends)

## 💡 Best Practices

### General
- Follow Kubernetes resource conventions
- Use structured UIDs with meaningful prefixes
- Add comprehensive labels for querying
- Include metadata annotations for context

### Code Generation
- Customize templates for cross-cutting concerns
- Test generated code thoroughly
- Version your templates alongside code
- Document template customizations

### Storage
- Choose storage backend based on scale needs
- Implement proper error handling
- Use context for timeouts and cancellation
- Consider caching for read-heavy workloads

### Authorization
- Start with RBAC, add ABAC as needed
- Test policies with multiple user contexts
- Document authorization rules clearly
- Use JWT claims for fine-grained control

### Versioning
- Use semantic versioning (v1, v2, v3)
- Mark beta/alpha versions appropriately
- Provide conversion for all version pairs
- Deprecate versions gracefully

## 🔍 Troubleshooting

### Common Issues

**Code generation fails:**
- Check template syntax
- Verify resource struct tags
- Ensure resource is registered
- See [Code Generation - Debugging](codegen.md#debugging)

**Storage errors:**
- Check file permissions
- Verify storage path exists
- Review error logs
- See [Storage - Troubleshooting](storage.md#troubleshooting)

**Authorization not working:**
- Verify JWT configuration
- Check policy registration
- Test with permissive policy first
- See [Authorization - Testing](policy.md#testing-policies)

**Version conversion fails:**
- Verify converter implementation
- Check version registration
- Test conversion paths
- See [Versioning - Debugging](versioning.md#troubleshooting)

## 📞 Getting Help

### Resources

- **[GitHub Issues](https://github.com/alexlovelltroy/fabrica/issues)** - Report bugs
- **[Discussions](https://github.com/alexlovelltroy/fabrica/discussions)** - Ask questions
- **[Examples](examples.md)** - Working code samples
- **[API Docs](https://pkg.go.dev/github.com/alexlovelltroy/fabrica)** - Package reference

### Support Channels

- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/alexlovelltroy/fabrica/issues/new?template=bug_report.md)
- 💡 **Feature Requests**: [GitHub Issues](https://github.com/alexlovelltroy/fabrica/issues/new?template=feature_request.md)
- ❓ **Questions**: [GitHub Discussions](https://github.com/alexlovelltroy/fabrica/discussions)
- 📖 **Documentation Issues**: [GitHub Issues](https://github.com/alexlovelltroy/fabrica/issues/new?template=documentation.md)

## 🚀 What's Next?

After reading the documentation:

1. **Build Something**: Start with [Getting Started Guide](getting-started.md)
2. **Share Your Project**: Show us what you built!
3. **Contribute**: Help improve Fabrica - [Contributing Guide](../CONTRIBUTING.md)
4. **Stay Updated**: Watch the [GitHub repo](https://github.com/alexlovelltroy/fabrica) for updates

## 📝 Documentation Standards

Our documentation follows these principles:

- **Beginner-Friendly**: Assume no prior knowledge
- **Example-Rich**: Show, don't just tell
- **Practical**: Focus on real-world use cases
- **Up-to-Date**: Kept in sync with code
- **Well-Organized**: Easy to navigate and search

### Contributing to Docs

Found an issue or want to improve the docs?

1. **Quick Fix**: Click "Edit" on any doc page
2. **Larger Changes**: See [Contributing Guide](../CONTRIBUTING.md)
3. **New Content**: Open an issue to discuss first

---

**Ready to build?** Start with the [Getting Started Guide](getting-started.md) →

**Have questions?** Check out [GitHub Discussions](https://github.com/alexlovelltroy/fabrica/discussions) →

**Want to contribute?** Read the [Contributing Guide](../CONTRIBUTING.md) →
