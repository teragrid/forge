// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tsd

import (
	"fmt"

	"github.com/teragrid/forge/internal/errcode"
)

// ErrUnknownMapping is returned when a TSD field value has no module mapping.
var ErrUnknownMapping = errcode.Register(errcode.Code(6402), "no module mapping for TSD field value")

// UnknownMappingError carries the field and value that had no mapping.
type UnknownMappingError struct {
	Key   string
	Value string
}

func (e *UnknownMappingError) Error() string {
	return fmt.Sprintf("[%s] no module mapping for %s=%q", ErrUnknownMapping.Format(), e.Key, e.Value)
}

// coreModules are always included in the output regardless of stack choices.
var coreModules = []string{
	"core/multi-tenancy",
	"core/rbac",
	"core/audit-log",
	"core/feature-flags",
	"core/soft-delete",
	"core/mcp-server", // MCP server — every forge project exposes MCP tools to AI agents
}

// frontendModuleMap maps stack.frontend.framework to a module ID.
var frontendModuleMap = map[string]string{
	"nextjs-15": "frontend/nextjs-15-supabase",
	"nuxt-3":    "frontend/nuxt-3-supabase",
	"remix":     "frontend/remix-supabase",
	"sveltekit": "frontend/sveltekit-supabase",
	"vue-3":     "frontend/vue-3-supabase",
	"none":      "",
}

// backendModuleMap maps "language/framework" to a module ID.
var backendModuleMap = map[string]string{
	"python/fastapi":     "backend/fastapi-supabase",
	"go/chi":             "backend/go-chi-supabase",
	"typescript/nestjs":  "backend/nestjs-supabase",
	"typescript/express": "backend/express-supabase",
	"go/gin":             "backend/go-gin-supabase",
	"go/fiber":           "backend/go-fiber-supabase",
}

// aiModuleMap maps stack.ai.orchestration to a module ID.
var aiModuleMap = map[string]string{
	"langgraph": "backend/langgraph-agent",
	"langchain": "backend/langchain-agent",
	"autogen":   "backend/autogen-agent",
	"none":      "",
}

// paymentModuleMap maps a payment provider name to a module ID.
// stripe and paypal have full-stack frontend modules that include the payment
// integration; other providers map to stub IDs that scaffold.Compose will skip
// when SkipMissing is set (directories do not yet exist in forge-knowledge/).
var paymentModuleMap = map[string]string{
	"stripe":   "frontend/nextjs-15-supabase-stripe",
	"paypal":   "frontend/nextjs-15-supabase-paypal",
	"adyen":    "backend/payments-adyen",
	"square":   "backend/payments-square",
	"razorpay": "backend/payments-razorpay",
}

// infraModuleMap maps stack.infra.container to a module ID.
var infraModuleMap = map[string]string{
	"docker-compose": "infra/docker-compose-fullstack",
	"kubernetes":     "infra/kubernetes-base",
	"none":           "",
}

// infraCloudModuleMap maps stack.infra.cloud to a module ID (TG-09).
var infraCloudModuleMap = map[string]string{
	"digitalocean": "infra/digitalocean-app-platform",
	"aws":          "",
	"gcp":          "",
	"azure":        "",
	"fly-io":       "",
	"none":         "",
}

// infraCICDModuleMap maps stack.infra.ci_cd to a module ID (TG-32).
var infraCICDModuleMap = map[string]string{
	"github-actions": "infra/github-actions-ci",
	"gitlab-ci":      "",
	"circleci":       "",
	"none":           "",
}

// observabilityTracingModuleMap maps stack.observability.tracing to a module ID (TG-32).
var observabilityTracingModuleMap = map[string]string{
	"opentelemetry": "observability/otel-prometheus",
	"jaeger":        "observability/otel-prometheus",
	"datadog":       "",
	"none":          "",
}

// observabilityMetricsModuleMap maps stack.observability.metrics to a module ID (TG-32).
var observabilityMetricsModuleMap = map[string]string{
	"prometheus-grafana": "observability/otel-prometheus",
	"datadog":            "",
	"cloudwatch":         "",
	"none":               "",
}

// messagingQueueModuleMap maps stack.messaging.queue to a module ID (TG-08).
var messagingQueueModuleMap = map[string]string{
	"celery-redis": "backend/fastapi-redis-queue",
	"redis-bullmq": "backend/nestjs-redis-queue",
	"sqs":          "",
	"pubsub":       "",
	"none":         "",
}

// Resolve returns the ordered list of module IDs to compose for the given TSD.
// The order is: core modules, frontend, backend, AI, payments (per-provider),
// infra, observability. Core modules always appear regardless of stack choices.
func Resolve(t *TSD) ([]string, error) {
	var modules []string

	// 1. Core modules are always included.
	modules = append(modules, coreModules...)

	// 2. Frontend module.
	if t.Stack.Frontend.Framework != "" && t.Stack.Frontend.Framework != "none" {
		mod, ok := frontendModuleMap[t.Stack.Frontend.Framework]
		if !ok {
			return nil, &UnknownMappingError{Key: "stack.frontend.framework", Value: t.Stack.Frontend.Framework}
		}
		if mod != "" {
			modules = append(modules, mod)
		}
	}

	// 3. Backend module (language + framework combination).
	if t.Stack.Backend.Language != "" && t.Stack.Backend.Language != "none" {
		key := t.Stack.Backend.Language + "/" + t.Stack.Backend.Framework
		mod, ok := backendModuleMap[key]
		if !ok {
			// Unknown combination — surface as an error.
			return nil, &UnknownMappingError{Key: "stack.backend.language+framework", Value: key}
		}
		if mod != "" {
			modules = append(modules, mod)
		}
	}

	// 4. AI module (orchestration).
	if t.Stack.AI.Orchestration != "" && t.Stack.AI.Orchestration != "none" {
		mod, ok := aiModuleMap[t.Stack.AI.Orchestration]
		if !ok {
			return nil, &UnknownMappingError{Key: "stack.ai.orchestration", Value: t.Stack.AI.Orchestration}
		}
		if mod != "" {
			modules = append(modules, mod)
		}
	}

	// 4a. Multi-LLM module when 2+ providers are declared (TG-07).
	if len(t.Stack.AI.LLMProviders) >= 2 {
		modules = append(modules, "backend/fastapi-multi-llm")
	}

	// 5. Payment modules — one per provider.
	for _, p := range t.Stack.Payments.Providers {
		mod, ok := paymentModuleMap[p]
		if !ok {
			return nil, &UnknownMappingError{Key: "stack.payments.providers[]", Value: p}
		}
		if mod != "" {
			modules = append(modules, mod)
		}
	}

	// 6. Infra module (container).
	if t.Stack.Infra.Container != "" && t.Stack.Infra.Container != "none" {
		mod, ok := infraModuleMap[t.Stack.Infra.Container]
		if !ok {
			return nil, &UnknownMappingError{Key: "stack.infra.container", Value: t.Stack.Infra.Container}
		}
		if mod != "" {
			modules = append(modules, mod)
		}
	}

	// 6a. Infra cloud module (TG-09).
	if t.Stack.Infra.Cloud != "" && t.Stack.Infra.Cloud != "none" {
		if mod, ok := infraCloudModuleMap[t.Stack.Infra.Cloud]; ok && mod != "" {
			modules = append(modules, mod)
		}
		// Unknown cloud values are silently skipped (forward-compat; no mapping yet).
	}

	// 6b. Messaging queue module (TG-08).
	if t.Stack.Messaging.Queue != "" && t.Stack.Messaging.Queue != "none" {
		if mod, ok := messagingQueueModuleMap[t.Stack.Messaging.Queue]; ok && mod != "" {
			modules = append(modules, mod)
		}
		// Unmapped queue values are silently skipped (forward-compat).
	}

	// 6c. Infra CI/CD module (TG-32).
	if t.Stack.Infra.CICD != "" && t.Stack.Infra.CICD != "none" {
		if mod, ok := infraCICDModuleMap[t.Stack.Infra.CICD]; ok && mod != "" {
			modules = append(modules, mod)
		}
	}

	// 6d. Observability: tracing module (TG-32).
	if t.Stack.Observability.Tracing != "" && t.Stack.Observability.Tracing != "none" {
		if mod, ok := observabilityTracingModuleMap[t.Stack.Observability.Tracing]; ok && mod != "" {
			// Deduplicate if metrics already added the same module.
			if !containsStr(modules, mod) {
				modules = append(modules, mod)
			}
		}
	}

	// 6e. Observability: metrics module (TG-32).
	if t.Stack.Observability.Metrics != "" && t.Stack.Observability.Metrics != "none" {
		if mod, ok := observabilityMetricsModuleMap[t.Stack.Observability.Metrics]; ok && mod != "" {
			if !containsStr(modules, mod) {
				modules = append(modules, mod)
			}
		}
	}

	// 7. Observability is always appended last.
	modules = append(modules, "observability/structured-logging")

	return modules, nil
}

// containsStr reports whether s is in slice.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
