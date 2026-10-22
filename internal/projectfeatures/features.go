// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

// Package projectfeatures maps validated project inputs to code generation.
package projectfeatures

import (
	"github.com/openchami/fabrica/internal/config"
	"github.com/openchami/fabrica/pkg/codegen"
)

// FromConfig preserves legacy auth enablement while carrying the complete startup model.
func FromConfig(cfg *config.FabricaConfig, versioned bool) codegen.FeatureConfig {
	if cfg == nil {
		return codegen.FeatureConfig{StorageType: "file", VersioningEnabled: versioned}.Normalize()
	}
	f := cfg.Features
	return codegen.FeatureConfig{
		ProjectName: cfg.Project.Name, StorageType: f.Storage.Type, DBDriver: f.Storage.DBDriver,
		EventsEnabled: f.Events.Enabled, EventBusType: f.Events.BusType,
		ReconciliationEnabled: f.Reconciliation.Enabled, ReconcileWorkers: f.Reconciliation.WorkerCount,
		ReconcileRequeueMinutes: f.Reconciliation.RequeueDelay, MetricsEnabled: f.Metrics.Enabled,
		AuthEnabled: f.Security.AuthN.Enabled || f.Auth.Enabled, AuthZEnabled: f.Security.AuthZ.Enabled, AuthZMode: string(f.Security.AuthZ.Mode),
		ValidationEnabled: f.Validation.Enabled, ValidationMode: f.Validation.Mode,
		ConditionalEnabled: f.Conditional.Enabled, ETagAlgorithm: f.Conditional.ETagAlgorithm, VersioningEnabled: versioned,
	}.Normalize()
}
