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

package llmresponse

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/teragrid/forge/internal/tokenledger"
)

// FORGE-2001 error code (llm/budget range 2400–2499). Registered here so the
// remedy is available in the JSON envelope without importing errcode.
const forge2001Code = "FORGE-2001"
const forge2001Remedy = "set FORGE_BUDGET_USD=<amount>  # raise or clear the per-invocation LLM spend cap"

// BudgetExceededError is returned when the invocation LLM spend cap is hit.
// It implements the forgeErr interface consumed by errorDetailFromErr.
type BudgetExceededError struct {
	SpentUSD  float64
	LimitUSD  float64
	Timestamp time.Time
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("FORGE-2001 llm/budget: LLM spend $%.4f exceeds cap $%.4f; set FORGE_BUDGET_USD to raise or clear",
		e.SpentUSD, e.LimitUSD)
}

// ForgeCode implements the forgeErr interface.
func (e *BudgetExceededError) ForgeCode() string { return forge2001Code }

// ForgeRemedy implements the forgeErr interface.
func (e *BudgetExceededError) ForgeRemedy() string { return forge2001Remedy }

// CheckBudget reads the token ledger and the FORGE_BUDGET_USD env var. If the
// total spend in the current session exceeds the cap it returns a
// *BudgetExceededError (never nil). If under budget or no cap is set it returns nil.
//
// root is the project root directory (where .forge/ lives).
func CheckBudget(root string) error {
	capStr := os.Getenv("FORGE_BUDGET_USD")
	if capStr == "" || capStr == "0" {
		return nil // no cap configured
	}
	limitUSD, err := strconv.ParseFloat(capStr, 64)
	if err != nil || limitUSD <= 0 {
		return nil // malformed or zero → treat as unlimited
	}

	ledgerPath := filepath.Join(root, tokenledger.DefaultPath)
	l := tokenledger.New(ledgerPath)
	spent, err := l.TotalCost()
	if err != nil || spent <= limitUSD {
		return nil // read error → don't block; under cap → fine
	}
	return &BudgetExceededError{
		SpentUSD:  spent,
		LimitUSD:  limitUSD,
		Timestamp: time.Now().UTC(),
	}
}

// SessionCost reads the token ledger and returns total tokens used and total
// cost in USD for the current project. Returns zeros on any error (non-fatal).
func SessionCost(root string) (tokensUsed int, costUSD float64) {
	ledgerPath := filepath.Join(root, tokenledger.DefaultPath)
	l := tokenledger.New(ledgerPath)
	summary, err := l.Summary()
	if err != nil || summary == nil {
		return 0, 0
	}
	for _, ms := range summary.ByModel {
		tokensUsed += ms.InputTokens + ms.OutputTokens
	}
	return tokensUsed, summary.TotalCostUSD
}
