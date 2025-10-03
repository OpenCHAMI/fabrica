package resource

import "time"

// Condition represents a specific condition of a resource.
//
// Conditions follow the Kubernetes pattern for representing the status of
// various aspects of a resource. Each condition has a type, status, reason,
// message, and transition time.
//
// Status Values:
//   - "True": The condition is satisfied
//   - "False": The condition is not satisfied
//   - "Unknown": The condition status cannot be determined
//
// Fields:
//   - Type: The type of condition (e.g., "Ready", "Healthy", "Reachable")
//   - Status: Current status of the condition ("True", "False", "Unknown")
//   - Reason: Machine-readable reason for the condition's last transition
//   - Message: Human-readable message explaining the condition
//   - LastTransitionTime: When the condition last changed status
//
// Example Usage:
//
//	// Create a new condition
//	condition := NewCondition("Ready", "True", "Healthy", "All health checks passed")
//
//	// Add to a conditions slice
//	var conditions []Condition
//	SetCondition(&conditions, "Ready", "True", "Healthy", "Resource is operational")
//
//	// Check condition status
//	if IsConditionTrue(conditions, "Ready") {
//	    // Handle ready state
//	}
//
// Common Condition Types:
//   - "Ready": Resource is ready for use
//   - "Healthy": Resource is functioning properly
//   - "Reachable": Resource can be contacted
//   - "Progressing": Resource is making progress toward desired state
type Condition struct {
	Type               string    `json:"type" yaml:"type"`
	Status             string    `json:"status" yaml:"status"` // "True", "False", "Unknown"
	Reason             string    `json:"reason,omitempty" yaml:"reason,omitempty"`
	Message            string    `json:"message,omitempty" yaml:"message,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
}

// NewCondition creates a new condition with the specified parameters.
//
// The LastTransitionTime is automatically set to the current time.
// This is the recommended way to create new conditions.
//
// Parameters:
//   - conditionType: The type of condition (e.g., "Ready", "Healthy")
//   - status: The condition status ("True", "False", "Unknown")
//   - reason: Machine-readable reason for the condition
//   - message: Human-readable message explaining the condition
//
// Example:
//
//	condition := NewCondition("Ready", "True", "Healthy", "All health checks passed")
func NewCondition(conditionType, status, reason, message string) Condition {
	return Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: time.Now(),
	}
}

// IsTrue checks if condition status is "True".
//
// This is a convenience method for checking if a condition is in the "True" state.
//
// Example:
//
//	if condition.IsTrue() {
//	    // Handle true condition
//	}
func (c *Condition) IsTrue() bool {
	return c.Status == "True"
}

// IsFalse checks if condition status is "False".
//
// This is a convenience method for checking if a condition is in the "False" state.
func (c *Condition) IsFalse() bool {
	return c.Status == "False"
}

// IsUnknown checks if condition status is "Unknown".
//
// This is a convenience method for checking if a condition is in the "Unknown" state.
func (c *Condition) IsUnknown() bool {
	return c.Status == "Unknown"
}

// Update updates the condition if status, reason, or message changed.
//
// Returns true if the condition was modified, false if no changes were made.
// The LastTransitionTime is only updated if the status changes, not if only
// the reason or message changes.
//
// This method implements proper Kubernetes-style condition semantics where
// transition time only changes when the status changes.
//
// Example:
//
//	changed := condition.Update("False", "NotReady", "Resource is unreachable")
//	if changed {
//	    // Log the condition change
//	}
func (c *Condition) Update(status, reason, message string) bool {
	if c.Status == status && c.Reason == reason && c.Message == message {
		return false // No change
	}

	// Only update transition time if status changed
	if c.Status != status {
		c.LastTransitionTime = time.Now()
	}

	c.Status = status
	c.Reason = reason
	c.Message = message
	return true
}

// Age returns how long ago the condition last transitioned.
//
// This is useful for determining how long a condition has been in its current state.
//
// Example:
//
//	if condition.Age() > 5*time.Minute {
//	    // Condition has been in this state for over 5 minutes
//	}
func (c *Condition) Age() time.Duration {
	return time.Since(c.LastTransitionTime)
}

// Clone creates a copy of the condition.
//
// Returns a new Condition with all fields copied. Useful when you need to
// modify a condition without affecting the original.
func (c *Condition) Clone() Condition {
	return Condition{
		Type:               c.Type,
		Status:             c.Status,
		Reason:             c.Reason,
		Message:            c.Message,
		LastTransitionTime: c.LastTransitionTime,
	}
}

// Helper functions for working with condition slices

// FindCondition finds a condition by type in a slice.
//
// Returns a pointer to the condition if found, nil otherwise.
// The returned pointer points to the condition in the original slice,
// so modifications will affect the original.
//
// Example:
//
//	condition := FindCondition(resource.Status.Conditions, "Ready")
//	if condition != nil && condition.IsTrue() {
//	    // Handle ready condition
//	}
func FindCondition(conditions []Condition, conditionType string) *Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// SetCondition sets or updates a condition in a slice.
//
// If a condition with the specified type already exists, it will be updated.
// If no condition exists, a new one will be added to the slice.
// Returns true if any changes were made.
//
// This function handles slice initialization if the slice is nil.
//
// Parameters:
//   - conditions: Pointer to the conditions slice
//   - conditionType: The type of condition to set
//   - status: The condition status ("True", "False", "Unknown")
//   - reason: Machine-readable reason
//   - message: Human-readable message
//
// Example:
//
//	changed := SetCondition(&resource.Status.Conditions, "Ready", "True", "Healthy", "All checks passed")
//	if changed {
//	    // Update resource or trigger events
//	}
func SetCondition(conditions *[]Condition, conditionType, status, reason, message string) bool {
	if *conditions == nil {
		*conditions = make([]Condition, 0)
	}

	// Find existing condition
	for i := range *conditions {
		if (*conditions)[i].Type == conditionType {
			return (*conditions)[i].Update(status, reason, message)
		}
	}

	// Add new condition
	*conditions = append(*conditions, NewCondition(conditionType, status, reason, message))
	return true
}

// RemoveCondition removes a condition by type from a slice.
//
// Returns true if a condition was removed, false if no condition
// with the specified type was found.
//
// Example:
//
//	removed := RemoveCondition(&resource.Status.Conditions, "Progressing")
//	if removed {
//	    // Log the condition removal
//	}
func RemoveCondition(conditions *[]Condition, conditionType string) bool {
	if *conditions == nil {
		return false
	}

	for i, condition := range *conditions {
		if condition.Type == conditionType {
			*conditions = append((*conditions)[:i], (*conditions)[i+1:]...)
			return true
		}
	}
	return false
}

// HasCondition checks if a condition type exists in the slice.
//
// Returns true if a condition with the specified type exists,
// regardless of its status.
//
// Example:
//
//	if HasCondition(resource.Status.Conditions, "Ready") {
//	    // Resource has a Ready condition
//	}
func HasCondition(conditions []Condition, conditionType string) bool {
	return FindCondition(conditions, conditionType) != nil
}

// IsConditionTrue checks if a specific condition exists and is true.
//
// This is a common pattern for checking if a resource is in a particular
// state. Returns false if the condition doesn't exist or if it's not "True".
//
// Example:
//
//	if IsConditionTrue(resource.Status.Conditions, "Ready") {
//	    // Resource is ready
//	}
func IsConditionTrue(conditions []Condition, conditionType string) bool {
	condition := FindCondition(conditions, conditionType)
	return condition != nil && condition.IsTrue()
}

// GetConditionStatus gets the status of a specific condition type.
//
// Returns the status of the condition if it exists, "Unknown" if the
// condition doesn't exist. This is useful when you need to handle
// all possible states consistently.
//
// Example:
//
//	status := GetConditionStatus(resource.Status.Conditions, "Ready")
//	switch status {
//	case "True":
//	    // Handle ready state
//	case "False":
//	    // Handle not ready state
//	default: // "Unknown"
//	    // Handle unknown state
//	}
func GetConditionStatus(conditions []Condition, conditionType string) string {
	condition := FindCondition(conditions, conditionType)
	if condition == nil {
		return "Unknown"
	}
	return condition.Status
}
