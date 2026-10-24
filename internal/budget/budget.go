// Package budget holds each project's cap (doctrine module "budgets"). The cap lives in
// the instance database, not the settings file, so the user can raise it from the UI.
package budget

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Chandler-Thompson/trireme/internal/journal"
)

// ErrNoCap is returned for a project that has no cap set.
var ErrNoCap = errors.New("budget: project has no cap")

// Caps reads and changes project caps.
type Caps struct {
	db      *sql.DB
	journal *journal.Journal
}

// New returns Caps.
func New(db *sql.DB, j *journal.Journal) *Caps { return &Caps{db: db, journal: j} }

// Seed sets a project's cap only if it has none, so a cap raised in the UI survives a
// restart with the settings file's original value.
func (c *Caps) Seed(projectID string, usd float64) error {
	if usd <= 0 {
		return fmt.Errorf("budget: cap for %s must be positive", projectID)
	}
	if _, err := c.db.Exec(`INSERT OR IGNORE INTO caps (project_id, cap_usd) VALUES (?, ?)`, projectID, usd); err != nil {
		return fmt.Errorf("budget: seed %s: %w", projectID, err)
	}
	return nil
}

// Get returns a project's cap.
func (c *Caps) Get(projectID string) (float64, error) {
	var usd float64
	err := c.db.QueryRow(`SELECT cap_usd FROM caps WHERE project_id = ?`, projectID).Scan(&usd)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNoCap
	}
	if err != nil {
		return 0, fmt.Errorf("budget: get %s: %w", projectID, err)
	}
	return usd, nil
}

// Raise sets a higher cap. A cap only ever goes up from the UI, and never to a value at
// or below what the project has already spent, which would pause it again at once.
func (c *Caps) Raise(projectID string, usd float64) error {
	current, err := c.Get(projectID)
	if err != nil {
		return err
	}
	spent, err := c.journal.Spent(projectID)
	if err != nil {
		return err
	}
	if usd <= current || usd <= spent {
		return fmt.Errorf("budget: new cap $%.2f must be above the current cap $%.2f and the $%.2f spent", usd, current, spent)
	}
	if _, err := c.db.Exec(`UPDATE caps SET cap_usd = ? WHERE project_id = ?`, usd, projectID); err != nil {
		return fmt.Errorf("budget: raise %s: %w", projectID, err)
	}
	_, err = c.journal.Append(journal.Event{ProjectID: projectID, Seat: "user", Type: "project_settings.changed",
		Payload: map[string]any{"setting": "budgets.project_cap_usd", "from": current, "to": usd}})
	return err
}

// Over reports whether a project has spent its cap.
func (c *Caps) Over(projectID string) (bool, float64, float64, error) {
	usd, err := c.Get(projectID)
	if err != nil {
		return false, 0, 0, err
	}
	spent, err := c.journal.Spent(projectID)
	if err != nil {
		return false, 0, 0, err
	}
	return spent >= usd, spent, usd, nil
}
