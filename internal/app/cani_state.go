package app

import (
	"github.com/janosmiko/lfk/internal/model"
)

// canIState bundles all Can-I / Who-Can RBAC explorer state into a
// single embedded struct so the top-level Model doesn't grow past the
// 800-line file ceiling as the overlay gains more knobs. Fields keep
// their original names — the only change is where they are declared,
// so call sites that reference m.canIGroups, m.whoCan, etc. continue
// to work via Go's struct field promotion.
type canIState struct {
	canIGroups            []model.CanIGroup
	canIGroupCursor       int          // selected group in left column
	canIGroupScroll       int          // first visible row in the group list
	canIResourceScroll    int          // scroll offset for the resource column
	canISubject           string       // "" = current user, or "system:serviceaccount:ns:name"
	canISubjectName       string       // display name for the subject ("Current User" or "sa/name")
	canIServiceAccounts   []string     // cached SA list for the selector
	canISearchActive      bool         // true when typing in search bar
	canISearchInput       TextInput    // current search input
	canISearchQuery       string       // confirmed search query for filtering
	canISubjectFilterMode bool         // true when typing in subject filter bar
	canIAllowedOnly       bool         // true = show only allowed permissions
	canINamespaces        []string     // namespaces used for SelfSubjectRulesReview
	canIMode              canIViewMode // forward Can-I or reverse Who-Can (Tab toggle)
	whoCan                whoCanState  // reverse-RBAC overlay state; see update_whocan.go
}
