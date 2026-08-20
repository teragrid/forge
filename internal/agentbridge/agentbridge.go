// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package agentbridge implements the boundary between forge's two execution
// planes.
//
// # The two planes
//
// Forge's ship pipeline is two different kinds of work fused into one binary:
//
//   - The deterministic plane — checkpoint state machine, gate evaluation,
//     artefact validation, spec manifest, traceability, scan/lint, audit and
//     token ledgers. All of it runs locally in Go, needs no network, and is
//     the only thing that can truthfully say whether a checkpoint passed.
//   - The reasoning plane — writing a spec, debating an architecture,
//     decomposing a breakdown. Only this half needs a model.
//
// Before 1.8.0 the two were welded together: no API key meant no ship. That
// forced users who already pay for a model inside a chat window (Claude Code,
// Copilot Chat, Cursor) to buy a second, redundant subscription just to run
// the deterministic half.
//
// # What the bridge does
//
// A Bridge replaces the provider call at forge's single LLM choke point
// (LLMPipe.InvokeChecked). Instead of dispatching to Anthropic or OpenAI it:
//
//  1. Looks the prompt up in a content-addressed response store. On a hit the
//     recorded completion is returned and the pipeline continues as if a
//     provider had answered.
//  2. On a miss it persists the compiled prompt as a *pending turn* and puts
//     the bridge into the paused state. The caller (forge ship) then stops,
//     prints the prompt block, and exits with a distinct code so the host
//     agent knows a turn is owed.
//
// The host agent — the model the user is already talking to — reads the
// prompt, produces the artefact, submits it back with `forge agent submit`,
// and re-runs `forge ship --agent-mode`. The pipeline replays from the top,
// hits the recorded response instead of pausing, and advances to the next
// LLM step. Repeat until the run completes.
//
// # Why this is not "just prose in a skill file"
//
// The host agent never decides whether a gate passed. It only supplies text
// where a provider would have. Every artefact it submits still goes through
// the same validateArtefact / checkpoint / hook path as a provider response,
// because it *is* a provider response as far as the rest of forge can tell.
// A skill that merely describes the methodology gives up that enforcement;
// this gives it up nowhere.
//
// # Replay keying
//
// Responses are keyed primarily by a content address — sha256 over the
// operation name plus the system and user prompts — so an unchanged question
// always replays its recorded answer.
//
// A pure content address is not sufficient on its own. Prompts routinely
// embed repository state, and the host agent writes files between turns, so
// the prompt for turn N can legitimately change once turn N's own artefact
// lands on disk. Keying on the hash alone would re-ask that question forever.
// The store therefore also keeps an ordinal key — the Nth call to a given
// operation within a run — and falls back to it when the hash misses. A
// fallback hit is flagged as drift so the condition is visible rather than
// silent; StrictReplay turns the fallback off for callers that would rather
// re-ask than replay a stale answer.
package agentbridge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultDir is the bridge state directory relative to the project root.
const DefaultDir = ".forge/agent"

// DefaultSession is the session name used when the caller does not pick one.
const DefaultSession = "default"

// ErrTurnRequired is returned by Lookup when no recorded response satisfies
// the prompt and a host-agent turn is therefore owed. Callers detect it with
// errors.Is and must not treat it as a checkpoint failure — it is a pause,
// not an error about the code under test.
var ErrTurnRequired = errors.New("host-agent turn required")

// ErrNoPendingTurn is returned by Fulfil when there is nothing to answer.
var ErrNoPendingTurn = errors.New("no pending turn to fulfil")

// Turn is a single LLM request the host agent must answer.
type Turn struct {
	Seq        int       `json:"seq"`
	Operation  string    `json:"operation"`
	Checkpoint string    `json:"checkpoint,omitempty"`
	Model      string    `json:"model,omitempty"`
	MaxTokens  int       `json:"max_tokens"`
	Hash       string    `json:"hash"`
	Ordinal    string    `json:"ordinal"`
	System     string    `json:"system"`
	User       string    `json:"user"`
	CreatedAt  time.Time `json:"created_at"`
}

// Response is a fulfilled turn, recorded so later replays never re-ask.
type Response struct {
	Hash      string    `json:"hash"`
	Ordinal   string    `json:"ordinal"`
	Operation string    `json:"operation"`
	Content   string    `json:"content"`
	Chars     int       `json:"chars"`
	FilledAt  time.Time `json:"filled_at"`
}

// Session is the bridge's run-level metadata, persisted so a chat window that
// has lost its own context can still discover what run it is driving.
type Session struct {
	Name        string    `json:"name"`
	Feature     string    `json:"feature,omitempty"`
	Slug        string    `json:"slug,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TurnsFilled int       `json:"turns_filled"`
}

// Stats summarises bridge state for `forge agent status`.
type Stats struct {
	Session   Session `json:"session"`
	Responses int     `json:"responses"`
	Pending   *Turn   `json:"pending,omitempty"`
	Drifted   int     `json:"drifted"`
}

// Bridge is the plane boundary. It is not safe for concurrent use; forge ship
// drives it from a single goroutine.
type Bridge struct {
	root    string
	dir     string
	session Session

	// byHash is the primary, content-addressed index.
	byHash map[string]Response
	// byOrdinal is the fallback index, keyed operation#N.
	byOrdinal map[string]Response
	// seen counts calls per operation within this process run, producing the
	// ordinal key. Reset on every Open so replay is deterministic.
	seen map[string]int

	pending *Turn
	// paused latches once a turn has been requested. Every later Lookup in the
	// same process returns ErrTurnRequired without overwriting the pending
	// turn, so one `forge ship --agent-mode` invocation always yields exactly
	// one question for the host agent to answer.
	paused bool

	// restored records that pending was loaded from disk rather than created
	// in this process — i.e. a previous run already showed the host agent a
	// question it has not answered yet. See Lookup for why that matters.
	restored bool

	// StrictReplay disables the ordinal fallback: a prompt whose hash is not
	// recorded is always re-asked, even when an ordinal-keyed answer exists.
	StrictReplay bool

	drifted int
	nextSeq int
}

// Open loads (or initialises) the bridge state for a session under root.
func Open(root, session string) (*Bridge, error) {
	if session == "" {
		session = DefaultSession
	}
	if !validSessionName(session) {
		return nil, fmt.Errorf("invalid session name %q: use letters, digits, '-' or '_'", session)
	}
	b := &Bridge{
		root:      root,
		dir:       filepath.Join(root, filepath.FromSlash(DefaultDir), session),
		byHash:    map[string]Response{},
		byOrdinal: map[string]Response{},
		seen:      map[string]int{},
		session:   Session{Name: session, CreatedAt: time.Now().UTC()},
	}
	if err := os.MkdirAll(b.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create bridge dir: %w", err)
	}
	if err := b.loadSession(); err != nil {
		return nil, err
	}
	if err := b.loadResponses(); err != nil {
		return nil, err
	}
	if err := b.loadPending(); err != nil {
		return nil, err
	}
	return b, nil
}

// Root returns the project root the bridge was opened against.
func (b *Bridge) Root() string { return b.root }

// Dir returns the bridge state directory.
func (b *Bridge) Dir() string { return b.dir }

// SessionName returns the active session name.
func (b *Bridge) SessionName() string { return b.session.Name }

// SetFeature records what feature this session is driving. Best-effort: a
// persistence failure never blocks the pipeline.
//
// If the session already has recorded responses from a *different* feature
// (its persisted Slug/Feature does not match), those responses are discarded
// first. Otherwise the ordinal fallback in Lookup — keyed only on
// "operation#N", with no feature scoping — would happily replay another
// feature's answer into this run merely because both features' Nth call to
// the same operation (e.g. "ship:qa-verify:generate") landed in the same
// position. That is silent cross-feature state contamination: the host agent
// never sees a prompt for the new feature, it just gets served the old
// feature's stale artefact under the new feature's name. Reusing the default
// session across unrelated features (rather than passing --session per
// feature) is exactly the case this guards.
func (b *Bridge) SetFeature(feature, slug string) (switched bool) {
	if b == nil {
		return false
	}
	prevSlug, prevFeature := b.session.Slug, b.session.Feature
	hadPrior := prevSlug != "" || prevFeature != ""
	newIdentity := slug != "" || feature != ""
	if hadPrior && newIdentity && prevSlug != slug && prevFeature != feature {
		_ = b.Reset()
		switched = true
	}
	b.session.Feature = feature
	b.session.Slug = slug
	_ = b.saveSession()
	return switched
}

// Paused reports whether a turn has been requested during this process run.
func (b *Bridge) Paused() bool { return b != nil && b.paused }

// Pending returns the turn awaiting a host-agent answer, if any.
func (b *Bridge) Pending() (Turn, bool) {
	if b == nil || b.pending == nil {
		return Turn{}, false
	}
	return *b.pending, true
}

// Lookup resolves a prompt against the recorded responses.
//
// On a hit it returns the recorded completion. On a miss it records the
// prompt as the pending turn, latches the paused state, and returns
// ErrTurnRequired. Once paused, later calls return ErrTurnRequired
// immediately without disturbing the already-pending turn.
func (b *Bridge) Lookup(operation, checkpoint, model, system, user string, maxTokens int) (string, error) {
	if b == nil {
		return "", ErrTurnRequired
	}
	hash := Hash(operation, system, user)
	// The ordinal is consumed even when the hash hits, so that the Nth call to
	// an operation keeps the same ordinal across runs regardless of which
	// index actually served it.
	b.seen[operation]++
	ordinal := fmt.Sprintf("%s#%d", operation, b.seen[operation])

	if resp, ok := b.byHash[hash]; ok {
		return resp.Content, nil
	}
	if !b.StrictReplay {
		if resp, ok := b.byOrdinal[ordinal]; ok {
			// The question changed but its position in the run did not — the
			// host agent's own writes moved the prompt out from under us.
			// Replay rather than loop, and count it so status can show it.
			b.drifted++
			return resp.Content, nil
		}
	}
	if b.paused {
		return "", ErrTurnRequired
	}
	// An unanswered turn from a previous run outranks whatever this run would
	// have asked. Re-running while paused is normal — a driver that loses its
	// place just runs `forge ship --agent-mode` again — and the pipeline can
	// legitimately arrive at a *different* prompt the second time, because a
	// checkpoint that could not generate its artefact may still have
	// scaffolded a stub, moving the next run onto the review path instead of
	// the generate path.
	//
	// Letting that overwrite pending.json would misfile the answer: the host
	// agent is holding a prompt it was shown, and `forge agent submit`
	// records against whatever is pending at submit time. It would file a
	// generated spec as if it were a review. So the original question stands
	// until it is answered or the session is reset.
	if b.restored && b.pending != nil {
		b.paused = true
		return "", ErrTurnRequired
	}
	b.pending = &Turn{
		Seq:        b.nextSeq,
		Operation:  operation,
		Checkpoint: checkpoint,
		Model:      model,
		MaxTokens:  maxTokens,
		Hash:       hash,
		Ordinal:    ordinal,
		System:     system,
		User:       user,
		CreatedAt:  time.Now().UTC(),
	}
	b.paused = true
	if err := b.savePending(); err != nil {
		return "", err
	}
	return "", ErrTurnRequired
}

// Fulfil records the host agent's answer to the pending turn and clears it.
// The recorded response is indexed under both keys so the next replay hits.
func (b *Bridge) Fulfil(content string) (Turn, error) {
	if b == nil || b.pending == nil {
		return Turn{}, ErrNoPendingTurn
	}
	if strings.TrimSpace(content) == "" {
		return Turn{}, errors.New("empty response: the host agent must supply the artefact content")
	}
	t := *b.pending
	resp := Response{
		Hash:      t.Hash,
		Ordinal:   t.Ordinal,
		Operation: t.Operation,
		Content:   content,
		Chars:     len(content),
		FilledAt:  time.Now().UTC(),
	}
	if err := b.appendResponse(resp); err != nil {
		return Turn{}, err
	}
	b.byHash[resp.Hash] = resp
	b.byOrdinal[resp.Ordinal] = resp
	b.pending = nil
	b.paused = false
	b.restored = false
	b.nextSeq = t.Seq + 1
	b.session.TurnsFilled++
	if err := b.clearPending(); err != nil {
		return Turn{}, err
	}
	_ = b.saveSession()
	return t, nil
}

// Reset discards every recorded response and any pending turn, so the next
// run re-asks from the first LLM step.
func (b *Bridge) Reset() error {
	if b == nil {
		return nil
	}
	for _, name := range []string{pendingFile, responsesFile, sessionFile} {
		if err := os.Remove(filepath.Join(b.dir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("reset %s: %w", name, err)
		}
	}
	b.byHash = map[string]Response{}
	b.byOrdinal = map[string]Response{}
	b.seen = map[string]int{}
	b.pending = nil
	b.paused = false
	b.restored = false
	b.drifted = 0
	b.nextSeq = 0
	b.session = Session{Name: b.session.Name, CreatedAt: time.Now().UTC()}
	return nil
}

// Stats returns a snapshot for `forge agent status`.
func (b *Bridge) Stats() Stats {
	if b == nil {
		return Stats{}
	}
	s := Stats{Session: b.session, Responses: len(b.byHash), Drifted: b.drifted}
	if b.pending != nil {
		p := *b.pending
		s.Pending = &p
	}
	return s
}

// Hash is the content address of a prompt: sha256 over the operation name and
// both prompt halves, with NUL separators so no concatenation can collide.
func Hash(operation, system, user string) string {
	h := sha256.New()
	h.Write([]byte(operation))
	h.Write([]byte{0})
	h.Write([]byte(system))
	h.Write([]byte{0})
	h.Write([]byte(user))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// ── persistence ───────────────────────────────────────────────────────────────

// Bridge state files are written 0600, not 0644. They hold the compiled
// prompts, and a prompt carries whatever repository context the checkpoint
// decided to include — source, specs, config. That is the same material an
// API request would carry, and it does not become less sensitive for having
// been routed through a chat window instead.
const (
	pendingFile   = "pending.json"
	responsesFile = "responses.jsonl"
	sessionFile   = "session.json"
)

func (b *Bridge) loadSession() error {
	data, err := os.ReadFile(filepath.Join(b.dir, sessionFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read session: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		// A corrupt session file is metadata-only; losing it must not strand
		// a run whose responses are still intact.
		return nil
	}
	s.Name = b.session.Name
	b.session = s
	return nil
}

func (b *Bridge) saveSession() error {
	b.session.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(b.session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.dir, sessionFile), append(data, '\n'), 0o600)
}

func (b *Bridge) loadResponses() error {
	path := filepath.Join(b.dir, responsesFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read responses: %w", err)
	}
	maxSeq := -1
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r Response
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue // skip a torn trailing line rather than fail the run
		}
		b.byHash[r.Hash] = r
		b.byOrdinal[r.Ordinal] = r
		maxSeq++
	}
	b.nextSeq = maxSeq + 1
	return nil
}

func (b *Bridge) appendResponse(r Response) error {
	f, err := os.OpenFile(filepath.Join(b.dir, responsesFile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open responses: %w", err)
	}
	defer f.Close()
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append response: %w", err)
	}
	return nil
}

func (b *Bridge) loadPending() error {
	data, err := os.ReadFile(filepath.Join(b.dir, pendingFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pending: %w", err)
	}
	var t Turn
	if err := json.Unmarshal(data, &t); err != nil {
		return fmt.Errorf("parse pending turn: %w", err)
	}
	b.pending = &t
	// Loading does not latch paused on its own: replay must still work, so
	// the run has to be allowed to proceed through every already-answered
	// prompt before it reaches the unanswered one. The latch happens in
	// Lookup, on the first miss, and preserves this turn rather than
	// replacing it.
	b.restored = true
	return nil
}

func (b *Bridge) savePending() error {
	data, err := json.MarshalIndent(b.pending, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(b.dir, pendingFile), append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write pending turn: %w", err)
	}
	return b.saveSession()
}

func (b *Bridge) clearPending() error {
	if err := os.Remove(filepath.Join(b.dir, pendingFile)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear pending turn: %w", err)
	}
	return nil
}

// ListSessions returns the bridge session names present under root, sorted.
func ListSessions(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(DefaultDir)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func validSessionName(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
