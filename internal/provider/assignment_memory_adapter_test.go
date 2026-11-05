package provider

import (
	"context"
	"fmt"
	"slices"
)

// memoryScopeAdapter is an in-memory scope adapter for unit tests of the
// assignment module. Like Lakekeeper, it applies each request atomically and
// rejects duplicate grants (409) and revokes of missing grants (404).
type memoryScopeAdapter struct {
	grants   []assignmentGrant
	listErr  error // returned by every ListGrants
	applyErr error // returned by every ApplyGrants, which then changes nothing
	lists    int
	applies  []memoryApply
}

type memoryApply struct {
	scope           assignmentScope
	revokes, grants []assignmentGrant
}

func newMemoryScopeAdapter(grants ...assignmentGrant) *memoryScopeAdapter {
	return &memoryScopeAdapter{grants: grants}
}

func (m *memoryScopeAdapter) ListGrants(_ context.Context, _ assignmentScope) ([]assignmentGrant, error) {
	m.lists++
	if m.listErr != nil {
		return nil, m.listErr
	}
	return slices.Clone(m.grants), nil
}

func (m *memoryScopeAdapter) ApplyGrants(_ context.Context, scope assignmentScope, revokes, grants []assignmentGrant) error {
	m.applies = append(m.applies, memoryApply{scope: scope, revokes: revokes, grants: grants})
	if m.applyErr != nil {
		return m.applyErr
	}
	next := slices.Clone(m.grants)
	for _, g := range revokes {
		i := slices.Index(next, g)
		if i < 0 {
			return fmt.Errorf("404: revoke of missing grant %+v", g)
		}
		next = slices.Delete(next, i, i+1)
	}
	for _, g := range grants {
		if slices.Contains(next, g) {
			return fmt.Errorf("409: duplicate grant %+v", g)
		}
		next = append(next, g)
	}
	m.grants = next
	return nil
}

// grantsOf returns the grant types the adapter holds for one assignee, sorted.
func (m *memoryScopeAdapter) grantsOf(kind assigneeKind, id string) []string {
	out := []string{}
	for _, g := range m.grants {
		if g.AssigneeKind == kind && g.AssigneeID == id {
			out = append(out, g.Type)
		}
	}
	slices.Sort(out)
	return out
}
