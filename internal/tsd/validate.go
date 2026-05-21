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
	"strings"

	"github.com/teragrid/forge/internal/errcode"
)

// Error codes for validation failures (range 6401..6449).
var (
	ErrValidation = errcode.Register(errcode.Code(6401), "TSD validation failed")
)

// ValidationError describes a single schema violation.
type ValidationError struct {
	Field   string
	Code    errcode.Code
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code.Format(), e.Field, e.Message)
}

// Validate checks all fields of t against allowed enum values and required
// constraints. It returns all violations found; an empty slice means valid.
// Validate never panics.
func Validate(t *TSD) []ValidationError {
	if t == nil {
		return []ValidationError{{Field: "(tsd)", Code: ErrValidation, Message: "nil TSD"}}
	}
	var errs []ValidationError

	// project.name is required.
	if strings.TrimSpace(t.Project.Name) == "" {
		errs = append(errs, ValidationError{
			Field:   "project.name",
			Code:    errcode.Code(6400),
			Message: "project name is required",
		})
	}

	// project.type enum.
	validProjectTypes := enumSet("saas", "service", "library", "data-platform", "marketplace",
		"api-product", "mobile-backend")
	if t.Project.Type != "" && !validProjectTypes[t.Project.Type] {
		errs = append(errs, enumErr("project.type", t.Project.Type, validProjectTypes))
	}

	// stack.frontend.framework enum.
	validFEFrameworks := enumSet("nextjs-15", "nuxt-3", "remix", "sveltekit", "vue-3", "react-spa", "none")
	if t.Stack.Frontend.Framework != "" && !validFEFrameworks[t.Stack.Frontend.Framework] {
		errs = append(errs, enumErr("stack.frontend.framework", t.Stack.Frontend.Framework, validFEFrameworks))
	}

	// stack.frontend.ui_library enum.
	validUILibs := enumSet("shadcn", "radix-ui", "chakra", "tailwind-only", "none")
	if t.Stack.Frontend.UILibrary != "" && !validUILibs[t.Stack.Frontend.UILibrary] {
		errs = append(errs, enumErr("stack.frontend.ui_library", t.Stack.Frontend.UILibrary, validUILibs))
	}

	// stack.backend.language enum.
	validBELangs := enumSet("python", "go", "typescript", "java", "none")
	if t.Stack.Backend.Language != "" && !validBELangs[t.Stack.Backend.Language] {
		errs = append(errs, enumErr("stack.backend.language", t.Stack.Backend.Language, validBELangs))
	}

	// stack.backend.framework enum.
	validBEFrameworks := enumSet("fastapi", "chi", "nestjs", "express", "gin", "fiber",
		"go-chi", "go-fiber", "spring-boot", "none")
	if t.Stack.Backend.Framework != "" && !validBEFrameworks[t.Stack.Backend.Framework] {
		errs = append(errs, enumErr("stack.backend.framework", t.Stack.Backend.Framework, validBEFrameworks))
	}

	// stack.backend.api_style enum.
	validAPIStyles := enumSet("rest", "graphql", "grpc", "trpc", "none")
	if t.Stack.Backend.APIStyle != "" && !validAPIStyles[t.Stack.Backend.APIStyle] {
		errs = append(errs, enumErr("stack.backend.api_style", t.Stack.Backend.APIStyle, validAPIStyles))
	}

	// stack.backend.auth enum.
	validAuths := enumSet("supabase-auth", "auth0", "clerk", "cognito", "keycloak", "custom-jwt", "none")
	if t.Stack.Backend.Auth != "" && !validAuths[t.Stack.Backend.Auth] {
		errs = append(errs, enumErr("stack.backend.auth", t.Stack.Backend.Auth, validAuths))
	}

	// stack.database.primary enum.
	validDBPrimary := enumSet("supabase-pg", "neon", "planetscale", "mysql", "sqlite", "none")
	if t.Stack.Database.Primary != "" && !validDBPrimary[t.Stack.Database.Primary] {
		errs = append(errs, enumErr("stack.database.primary", t.Stack.Database.Primary, validDBPrimary))
	}

	// stack.database.cache enum.
	validCache := enumSet("redis", "memcached", "none")
	if t.Stack.Database.Cache != "" && !validCache[t.Stack.Database.Cache] {
		errs = append(errs, enumErr("stack.database.cache", t.Stack.Database.Cache, validCache))
	}

	// stack.ai.orchestration enum (optional section).
	validOrch := enumSet("langgraph", "langchain", "autogen", "none")
	if t.Stack.AI.Orchestration != "" && !validOrch[t.Stack.AI.Orchestration] {
		errs = append(errs, enumErr("stack.ai.orchestration", t.Stack.AI.Orchestration, validOrch))
	}

	// stack.payments.providers enum values.
	validPayProviders := enumSet("stripe", "paypal", "adyen", "square", "razorpay")
	for _, p := range t.Stack.Payments.Providers {
		if !validPayProviders[p] {
			errs = append(errs, enumErr("stack.payments.providers[]", p, validPayProviders))
		}
	}

	// stack.payments.model enum.
	validPayModels := enumSet("subscription", "one-time", "usage-based", "marketplace")
	if t.Stack.Payments.Model != "" && !validPayModels[t.Stack.Payments.Model] {
		errs = append(errs, enumErr("stack.payments.model", t.Stack.Payments.Model, validPayModels))
	}

	// stack.infra.cloud enum.
	validClouds := enumSet("digitalocean", "aws", "gcp", "azure", "fly-io", "none")
	if t.Stack.Infra.Cloud != "" && !validClouds[t.Stack.Infra.Cloud] {
		errs = append(errs, enumErr("stack.infra.cloud", t.Stack.Infra.Cloud, validClouds))
	}

	// stack.infra.container enum.
	validContainers := enumSet("docker-compose", "kubernetes", "fly-io", "railway", "none")
	if t.Stack.Infra.Container != "" && !validContainers[t.Stack.Infra.Container] {
		errs = append(errs, enumErr("stack.infra.container", t.Stack.Infra.Container, validContainers))
	}

	// stack.infra.ci_cd enum.
	validCICD := enumSet("github-actions", "gitlab-ci", "circleci", "none")
	if t.Stack.Infra.CICD != "" && !validCICD[t.Stack.Infra.CICD] {
		errs = append(errs, enumErr("stack.infra.ci_cd", t.Stack.Infra.CICD, validCICD))
	}

	// stack.compliance.standards enum values.
	validStandards := enumSet("pci-dss-saq-a", "gdpr", "soc2", "hipaa", "soc2-type2", "iso27001")
	for _, s := range t.Stack.Compliance.Standards {
		if !validStandards[s] {
			errs = append(errs, enumErr("stack.compliance.standards[]", s, validStandards))
		}
	}

	// stack.compliance.secret_scanning enum (TG-04).
	validSecretScan := enumSet("gitleaks", "trufflehog", "none")
	if t.Stack.Compliance.SecretScanning != "" && !validSecretScan[t.Stack.Compliance.SecretScanning] {
		errs = append(errs, enumErr("stack.compliance.secret_scanning", t.Stack.Compliance.SecretScanning, validSecretScan))
	}

	// stack.messaging field enums (TG-01).
	validMsgQueue := enumSet("redis-bullmq", "celery-redis", "sqs", "pubsub", "none")
	if t.Stack.Messaging.Queue != "" && !validMsgQueue[t.Stack.Messaging.Queue] {
		errs = append(errs, enumErr("stack.messaging.queue", t.Stack.Messaging.Queue, validMsgQueue))
	}

	validMsgRealtime := enumSet("supabase-realtime", "pusher", "ably", "websocket", "none")
	if t.Stack.Messaging.Realtime != "" && !validMsgRealtime[t.Stack.Messaging.Realtime] {
		errs = append(errs, enumErr("stack.messaging.realtime", t.Stack.Messaging.Realtime, validMsgRealtime))
	}

	validMsgEmail := enumSet("resend", "sendgrid", "ses", "none")
	if t.Stack.Messaging.Email != "" && !validMsgEmail[t.Stack.Messaging.Email] {
		errs = append(errs, enumErr("stack.messaging.email", t.Stack.Messaging.Email, validMsgEmail))
	}

	validMsgSMS := enumSet("twilio", "vonage", "none")
	if t.Stack.Messaging.SMS != "" && !validMsgSMS[t.Stack.Messaging.SMS] {
		errs = append(errs, enumErr("stack.messaging.sms", t.Stack.Messaging.SMS, validMsgSMS))
	}

	// stack.ai sub-field enums (TG-02).
	validLLMProviders := enumSet("openai", "anthropic", "google-gemini", "mistral", "groq")
	for _, p := range t.Stack.AI.LLMProviders {
		if !validLLMProviders[p] {
			errs = append(errs, enumErr("stack.ai.llm_providers[]", p, validLLMProviders))
		}
	}

	validEmbedding := enumSet("openai-ada", "cohere", "local-onnx", "none")
	if t.Stack.AI.Embedding != "" && !validEmbedding[t.Stack.AI.Embedding] {
		errs = append(errs, enumErr("stack.ai.embedding", t.Stack.AI.Embedding, validEmbedding))
	}

	validVectorStore := enumSet("supabase-pgvector", "pinecone", "weaviate", "none")
	if t.Stack.AI.VectorStore != "" && !validVectorStore[t.Stack.AI.VectorStore] {
		errs = append(errs, enumErr("stack.ai.vector_store", t.Stack.AI.VectorStore, validVectorStore))
	}

	validAIObservability := enumSet("langsmith", "phoenix", "none")
	if t.Stack.AI.Observability != "" && !validAIObservability[t.Stack.AI.Observability] {
		errs = append(errs, enumErr("stack.ai.observability", t.Stack.AI.Observability, validAIObservability))
	}

	// stack.observability sub-field enums (TG-03).
	validOBSMetrics := enumSet("prometheus-grafana", "datadog", "newrelic", "none")
	if t.Stack.Observability.Metrics != "" && !validOBSMetrics[t.Stack.Observability.Metrics] {
		errs = append(errs, enumErr("stack.observability.metrics", t.Stack.Observability.Metrics, validOBSMetrics))
	}

	validOBSTracing := enumSet("opentelemetry", "sentry", "none")
	if t.Stack.Observability.Tracing != "" && !validOBSTracing[t.Stack.Observability.Tracing] {
		errs = append(errs, enumErr("stack.observability.tracing", t.Stack.Observability.Tracing, validOBSTracing))
	}

	validOBSLogging := enumSet("json-stdout", "structlog-loki", "cloudwatch", "datadog", "stdout")
	if t.Stack.Observability.Logging != "" && !validOBSLogging[t.Stack.Observability.Logging] {
		errs = append(errs, enumErr("stack.observability.logging", t.Stack.Observability.Logging, validOBSLogging))
	}

	validOBSAlerting := enumSet("pagerduty", "opsgenie", "slack-webhook", "none")
	if t.Stack.Observability.Alerting != "" && !validOBSAlerting[t.Stack.Observability.Alerting] {
		errs = append(errs, enumErr("stack.observability.alerting", t.Stack.Observability.Alerting, validOBSAlerting))
	}

	return errs
}

// enumSet returns a set (map) of the given values for O(1) lookup.
func enumSet(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

// enumErr constructs a ValidationError for an invalid enum value.
func enumErr(field, got string, allowed map[string]bool) ValidationError {
	keys := make([]string, 0, len(allowed))
	for k := range allowed {
		keys = append(keys, k)
	}
	// Sort for stable error messages.
	sortStrings(keys)
	return ValidationError{
		Field:   field,
		Code:    ErrValidation,
		Message: fmt.Sprintf("invalid value %q; allowed: [%s]", got, strings.Join(keys, ", ")),
	}
}

// sortStrings sorts a string slice in-place (avoids import of sort in test files).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
