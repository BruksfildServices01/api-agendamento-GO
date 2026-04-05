// Package shared exposes canonical SQL fragments reused across read-model
// query packages (crm, daypanel, financial, etc.).
// Centralizing these snippets ensures that every query applies the same
// business rule; a change here propagates everywhere automatically.
package shared

// ActiveSubscriptionSQL is the canonical WHERE condition for an active,
// non-expired subscription row.  The snippet references the table alias "s"
// so every query that includes it must alias the subscriptions table as "s".
//
// Rule:
//   - status must be 'active'
//   - the current period must have started (current_period_start <= NOW())
//   - the current period must not have ended (current_period_end > NOW())
//
// This mirrors the condition in infra/repository/subscription_gorm.go
// (GetActiveSubscription / IncrementCutsUsed) and is the single source of
// truth for "what counts as an active subscription".
const ActiveSubscriptionSQL = `s.status = 'active'
	  AND s.current_period_start <= NOW()
	  AND s.current_period_end   >  NOW()`
