<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica Documentation

> Complete documentation for the Fabrica framework - build resource-based REST APIs with automatic code generation.

## 📚 Quick Links

- **[Quick Start](guides/quickstart.md)** - Get started in 30 minutes
- **[Getting Started Guide](guides/getting-started.md)** - Full tutorial
- **[Architecture Overview](reference/architecture.md)** - Framework design
- **[API Reference](https://pkg.go.dev/github.com/openchami/fabrica)** - Go package docs

## 📖 Documentation Structure

### Guides (`guides/`)

Step-by-step tutorials and how-tos for building with Fabrica:

**Getting Started:**
- **[Quick Start](guides/quickstart.md)** - 30-minute simple REST API
- **[Getting Started](guides/getting-started.md)** - Complete resource management tutorial

**Core Features:**
- **[Resource Model](guides/resource-model.md)** - Understanding Fabrica resources
- **[Storage Systems](guides/storage.md)** - File and database backends
- **[Ent Storage Integration](guides/storage-ent.md)** - Using Ent ORM with databases
- **[Validation](guides/validation.md)** - Request validation and error handling

**Advanced Features:**
- **[Events](guides/events.md)** - CloudEvents integration and event-driven patterns
- **[Reconciliation](guides/reconciliation.md)** - Controller pattern for declarative management
- **[Versioning](guides/versioning.md)** - Multi-version API support
- **[Conditional Requests](guides/conditional-and-patch.md)** - ETags and PATCH operations

### Reference (`reference/`)

Technical reference and in-depth explanations:

- **[Architecture](reference/architecture.md)** - Framework design and principles
- **[Code Generation](reference/codegen.md)** - How templates work and customization
- **[Framework Comparison](reference/comparison.md)** - Fabrica vs other Go frameworks
- **[Releasing](reference/releasing.md)** - Release process and versioning
- **[Status Subresources](status-subresource.md)** - Kubernetes-style status management

## 🎯 Learning Paths

### Path 1: Beginners (30 minutes)
1. [Quick Start](guides/quickstart.md) - Simple REST API
2. [Resource Model](guides/resource-model.md) - Understand resources

### Path 2: Building Applications (2-4 hours)
1. [Getting Started](guides/getting-started.md) - Full tutorial
2. [Storage Systems](guides/storage.md) - Choose your backend
3. [Validation](guides/validation.md) - Add validation
4. [Events](guides/events.md) - Event-driven patterns

### Path 3: Advanced (1-2 days)
1. [Architecture](reference/architecture.md) - Deep dive into design
2. [Reconciliation](guides/reconciliation.md) - Declarative management
3. [Versioning](guides/versioning.md) - Multi-version APIs
4. [Code Generation](reference/codegen.md) - Customize templates

## 🔍 Common Tasks

**I want to...**

- **Build a simple API quickly** → [Quick Start](guides/quickstart.md)
- **Learn the full framework** → [Getting Started](guides/getting-started.md)
- **Understand resources** → [Resource Model](guides/resource-model.md)
- **Choose a storage backend** → [Storage Systems](guides/storage.md)
- **Use a database** → [Ent Storage](guides/storage-ent.md)
- **Add validation** → [Validation](guides/validation.md)
- **Build event-driven systems** → [Events](guides/events.md)
- **Implement reconciliation** → [Reconciliation](guides/reconciliation.md)
- **Support multiple API versions** → [Versioning](guides/versioning.md)
- **Customize generated code** → [Code Generation](reference/codegen.md)
- **Compare frameworks** → [Framework Comparison](reference/comparison.md)

## 📞 Getting Help

### Resources

- **[GitHub Issues](https://github.com/openchami/fabrica/issues)** - Report bugs or request features
- **[Discussions](https://github.com/openchami/fabrica/discussions)** - Ask questions
- **[API Docs](https://pkg.go.dev/github.com/openchami/fabrica)** - Package reference
- **[Examples](../examples/)** - Working code samples

### Support Channels

- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/openchami/fabrica/issues/new?template=bug_report.md)
- 💡 **Feature Requests**: [GitHub Issues](https://github.com/openchami/fabrica/issues/new?template=feature_request.md)
- ❓ **Questions**: [GitHub Discussions](https://github.com/openchami/fabrica/discussions)

## 🤝 Contributing

Help make Fabrica better!

- **[Contributing Guide](../CONTRIBUTING.md)** - How to contribute
- **[Releasing Guide](reference/releasing.md)** - Release process

## 🚀 What's Next?

1. **Start Building**: Follow the [Getting Started Guide](guides/getting-started.md)
2. **Explore Examples**: Check out the [examples directory](../examples/)
3. **Join the Community**: Participate in [GitHub Discussions](https://github.com/openchami/fabrica/discussions)
4. **Contribute**: Help improve Fabrica - see the [Contributing Guide](../CONTRIBUTING.md)

---

**Ready to build?** Start with the [Quick Start](guides/quickstart.md) →
