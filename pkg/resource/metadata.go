package resource

import "time"

// Metadata contains common metadata for all resources.
//
// Metadata includes identity information (Name, UID), organizational data
// (Labels, Annotations), and lifecycle timestamps (CreatedAt, UpdatedAt).
//
// Labels vs Annotations:
//   - Labels: Short, structured key-value pairs used for selection and grouping.
//     Should be meaningful to automated systems. Limited to 63 characters.
//   - Annotations: Arbitrary key-value pairs for storing metadata that doesn't
//     need to be queryable. Can contain longer, unstructured data.
//
// Fields:
//   - Name: Human-readable name, unique within a namespace/scope
//   - UID: Globally unique identifier, typically generated using GenerateUID()
//   - Labels: Key-value pairs for selection and organization
//   - Annotations: Key-value pairs for arbitrary metadata
//   - CreatedAt: Resource creation timestamp
//   - UpdatedAt: Last modification timestamp
//
// Example Labels:
//
//	resource.SetLabel("environment", "production")
//	resource.SetLabel("location", "datacenter-01")
//	resource.SetLabel("role", "server")
//
// Example Annotations:
//
//	resource.SetAnnotation("deployment.notes", "Deployed during maintenance window")
//	resource.SetAnnotation("contact.email", "ops@example.com")
type Metadata struct {
	Name        string            `json:"name" yaml:"name"`
	UID         string            `json:"uid" yaml:"uid"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	CreatedAt   time.Time         `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt" yaml:"updatedAt"`
}

// Metadata helper methods

// IsEmpty checks if metadata has minimal required fields.
//
// Returns true if either Name or UID is empty. This is useful for
// validating that a resource has been properly initialized.
//
// Example:
//
//	if resource.Metadata.IsEmpty() {
//	    return errors.New("resource metadata is incomplete")
//	}
func (m *Metadata) IsEmpty() bool {
	return m.Name == "" || m.UID == ""
}

// Initialize sets up metadata with required fields and initializes maps.
//
// This is the recommended way to initialize metadata for a new resource.
// Sets CreatedAt and UpdatedAt to current time and initializes empty
// labels and annotations maps.
//
// Parameters:
//   - name: Human-readable name for the resource
//   - uid: Unique identifier, typically generated using GenerateUID()
//
// Example:
//
//	uid, _ := GenerateUIDForResource("Device")
//	resource.Metadata.Initialize("device-001", uid)
func (m *Metadata) Initialize(name, uid string) {
	now := time.Now()
	m.Name = name
	m.UID = uid
	m.CreatedAt = now
	m.UpdatedAt = now
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	if m.Annotations == nil {
		m.Annotations = make(map[string]string)
	}
}

// Clone creates a deep copy of metadata.
//
// Returns a new Metadata instance with all fields copied. The labels
// and annotations maps are also deep-copied, so modifications to the
// clone will not affect the original.
//
// This is useful when you need to create derived resources or when
// implementing copy operations.
//
// Example:
//
//	metadataCopy := originalMetadata.Clone()
//	metadataCopy.Name = "new-name" // Won't affect original
func (m *Metadata) Clone() *Metadata {
	clone := &Metadata{
		Name:      m.Name,
		UID:       m.UID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}

	if m.Labels != nil {
		clone.Labels = make(map[string]string)
		for k, v := range m.Labels {
			clone.Labels[k] = v
		}
	}

	if m.Annotations != nil {
		clone.Annotations = make(map[string]string)
		for k, v := range m.Annotations {
			clone.Annotations[k] = v
		}
	}

	return clone
}
