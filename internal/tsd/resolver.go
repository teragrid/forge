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
var paymentModuleMap = map[string]string{
	"stripe":   "backend/payments-stripe",
	"paypal":   "backend/payments-paypal",
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

	// 4. AI module.
	if t.Stack.AI.Orchestration != "" && t.Stack.AI.Orchestration != "none" {
		mod, ok := aiModuleMap[t.Stack.AI.Orchestration]
		if !ok {
			return nil, &UnknownMappingError{Key: "stack.ai.orchestration", Value: t.Stack.AI.Orchestration}
		}
		if mod != "" {
			modules = append(modules, mod)
		}
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

	// 6. Infra module.
	if t.Stack.Infra.Container != "" && t.Stack.Infra.Container != "none" {
		mod, ok := infraModuleMap[t.Stack.Infra.Container]
		if !ok {
			return nil, &UnknownMappingError{Key: "stack.infra.container", Value: t.Stack.Infra.Container}
		}
		if mod != "" {
			modules = append(modules, mod)
		}
	}

	// 7. Observability is always appended last.
	modules = append(modules, "observability/structured-logging")

	return modules, nil
}
