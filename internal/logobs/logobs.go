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
// Package logobs provides the structured logger used by every Forge verb
// (DEV-M0-04 / Arch §11). MVP scope: slog JSON or TTY formatter, level via
// flag/env, redaction of fields whose key starts with "secret_" (a placeholder
// for the full secrets pipeline in DEV-M0-09).
package logobs

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format selects the renderer.
type Format string

const (
	FormatAuto Format = "auto" // TTY-aware: text on terminals, JSON otherwise.
	FormatJSON Format = "json"
	FormatText Format = "text"
)

// Options configures the logger.
type Options struct {
	Level   slog.Level
	Format  Format
	Out     io.Writer // defaults to os.Stderr
	Explain bool      // when true, do not redact fields (DEV-M0-04 TC-04-05)
}

// New builds a *slog.Logger honoring Options. Always safe to call with the
// zero value (defaults: info / auto / stderr / no-explain).
func New(opt Options) *slog.Logger {
	if opt.Out == nil {
		opt.Out = os.Stderr
	}
	if opt.Format == "" {
		opt.Format = FormatAuto
	}
	format := opt.Format
	if format == FormatAuto {
		if isTerminal(opt.Out) {
			format = FormatText
		} else {
			format = FormatJSON
		}
	}

	hopts := &slog.HandlerOptions{
		Level: opt.Level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if !opt.Explain && shouldRedact(a.Key) {
				a.Value = slog.StringValue("[REDACTED]")
			}
			return a
		},
	}
	var h slog.Handler
	if format == FormatJSON {
		h = slog.NewJSONHandler(opt.Out, hopts)
	} else {
		h = slog.NewTextHandler(opt.Out, hopts)
	}
	return slog.New(h)
}

// shouldRedact returns true for keys conventionally holding sensitive material.
// The full pipeline (DEV-M0-09) replaces this with placeholder rewriting.
func shouldRedact(key string) bool {
	k := strings.ToLower(key)
	switch {
	case strings.HasPrefix(k, "secret_"),
		strings.HasPrefix(k, "token_"),
		strings.HasPrefix(k, "api_key"),
		k == "secret", k == "token", k == "password", k == "api_key":
		return true
	}
	return false
}

// isTerminal reports whether w is an *os.File attached to a character device.
// Pure-Go fallback: avoids the golang.org/x/term dep for the MVP.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// CtxWithLogger / FromCtx allow per-command logger threading without globals.
type ctxKey struct{}

func CtxWithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromCtx returns the context-bound logger, or slog.Default if none.
func FromCtx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
