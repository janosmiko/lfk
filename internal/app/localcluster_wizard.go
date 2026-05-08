// localcluster_wizard implements the Create-cluster sub-flow inside
// the manager overlay. The wizard is a stack of screens; Esc pops one
// step, Enter advances. State lives on Model.localClusterState.wizard.
package app

import (
	"regexp"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s/localcluster"
)

var (
	wizardNameRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,29}$`)
	wizardVersionRe = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?$`)
)

// startWizard opens the provider picker as wizard step 1. Resets all
// wizard state so a previous half-filled session never bleeds through.
func (m Model) startWizard() Model {
	provs := localcluster.Installed()
	names := make([]string, 0, len(provs))
	for _, p := range provs {
		names = append(names, p.Name())
	}
	m.localClusterState.screen = localClusterScreenWizardProvider
	m.localClusterState.wizard = localClusterWizard{
		installedSet: names,
		providerCur:  0,
		nodes:        1,
		nodesStr:     "1",
	}
	return m
}

func (m Model) updateWizardProviderKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		m.localClusterState.screen = localClusterScreenList
		return m, nil, true
	case tea.KeyEnter:
		set := m.localClusterState.wizard.installedSet
		if len(set) == 0 {
			return m, nil, true
		}
		m.localClusterState.wizard.provider = set[m.localClusterState.wizard.providerCur]
		m.localClusterState.screen = localClusterScreenWizardName
		return m, nil, true
	}
	switch msg.String() {
	case "j", "down":
		if m.localClusterState.wizard.providerCur < len(m.localClusterState.wizard.installedSet)-1 {
			m.localClusterState.wizard.providerCur++
		}
		return m, nil, true
	case "k", "up":
		if m.localClusterState.wizard.providerCur > 0 {
			m.localClusterState.wizard.providerCur--
		}
		return m, nil, true
	}
	return m, nil, true
}

func (m Model) updateWizardNameKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		m.localClusterState.screen = localClusterScreenWizardProvider
		return m, nil, true
	case tea.KeyEnter:
		if m.validateWizardName(m.localClusterState.wizard.name) != "" {
			return m, nil, true
		}
		m.localClusterState.screen = localClusterScreenWizardVersion
		return m, nil, true
	case tea.KeyBackspace:
		if n := len(m.localClusterState.wizard.name); n > 0 {
			m.localClusterState.wizard.name = m.localClusterState.wizard.name[:n-1]
		}
		m.localClusterState.wizard.nameErr = m.validateWizardName(m.localClusterState.wizard.name)
		return m, nil, true
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if isValidWizardNameRune(r) {
				m.localClusterState.wizard.name += string(r)
			}
		}
		m.localClusterState.wizard.nameErr = m.validateWizardName(m.localClusterState.wizard.name)
		return m, nil, true
	}
	return m, nil, true
}

func isValidWizardNameRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == '-':
		return true
	}
	return false
}

func (m Model) updateWizardVersionKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		m.localClusterState.screen = localClusterScreenWizardName
		return m, nil, true
	case tea.KeyEnter:
		if e := validateWizardVersion(m.localClusterState.wizard.version); e != "" {
			m.localClusterState.wizard.versionErr = e
			return m, nil, true
		}
		m.localClusterState.wizard.versionErr = ""
		m.localClusterState.screen = localClusterScreenWizardNodes
		return m, nil, true
	case tea.KeyBackspace:
		if n := len(m.localClusterState.wizard.version); n > 0 {
			m.localClusterState.wizard.version = m.localClusterState.wizard.version[:n-1]
		}
		m.localClusterState.wizard.versionErr = validateWizardVersion(m.localClusterState.wizard.version)
		return m, nil, true
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if isValidWizardVersionRune(r) {
				m.localClusterState.wizard.version += string(r)
			}
		}
		m.localClusterState.wizard.versionErr = validateWizardVersion(m.localClusterState.wizard.version)
		return m, nil, true
	}
	return m, nil, true
}

func isValidWizardVersionRune(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r == '.', r == 'v':
		return true
	}
	return false
}

func (m Model) updateWizardConfirmKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		m.localClusterState.screen = localClusterScreenWizardNodes
		return m, nil, true
	case tea.KeyEnter:
		w := m.localClusterState.wizard
		prov := localcluster.ByName(w.provider)
		if prov == nil {
			m.localClusterState.err = "unknown provider: " + w.provider
			m.localClusterState.screen = localClusterScreenList
			return m, nil, true
		}
		spec := localcluster.CreateSpec{
			Name: w.name, K8sVersion: w.version, Nodes: w.nodes,
		}
		// Surface the in-flight create as a synthetic InFlight row so
		// the user sees something is happening — kind/k3d/minikube
		// create is a 30-120s operation (image pulls, container boot).
		// Without this the manager looks frozen until the result
		// arrives. The row is keyed by provider+name;
		// updateLocalClusterCreated removes it once the CLI returns
		// and Detect re-fires.
		m.localClusterState.clusters = append(m.localClusterState.clusters, localClusterRow{
			Provider: w.provider, Name: w.name, ContextName: w.provider + "-" + w.name,
			Status: "creating", Mutating: "creating...", K8sVersion: w.version, Nodes: w.nodes,
			state: rowStateInFlight,
		})
		m.localClusterState.info = "Creating " + w.provider + "/" + w.name + "..."
		m.localClusterState.err = ""
		m.localClusterState.screen = localClusterScreenList
		m.localClusterState.wizard = localClusterWizard{}
		return m, m.dispatchCreateLocalCluster(prov, spec), true
	}
	return m, nil, true
}

func (m Model) updateWizardNodesKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		m.localClusterState.screen = localClusterScreenWizardVersion
		return m, nil, true
	case tea.KeyEnter:
		n, e := validateWizardNodes(m.localClusterState.wizard.nodesStr)
		if e != "" {
			m.localClusterState.wizard.nodesErr = e
			return m, nil, true
		}
		m.localClusterState.wizard.nodes = n
		m.localClusterState.wizard.nodesErr = ""
		m.localClusterState.screen = localClusterScreenWizardConfirm
		return m, nil, true
	case tea.KeyBackspace:
		if n := len(m.localClusterState.wizard.nodesStr); n > 0 {
			m.localClusterState.wizard.nodesStr = m.localClusterState.wizard.nodesStr[:n-1]
		}
		_, m.localClusterState.wizard.nodesErr = validateWizardNodes(m.localClusterState.wizard.nodesStr)
		return m, nil, true
	case tea.KeyRunes:
		// Single-digit field: a fresh keypress overwrites any prior value.
		for _, r := range msg.Runes {
			if r >= '0' && r <= '9' {
				m.localClusterState.wizard.nodesStr = string(r)
			}
		}
		_, m.localClusterState.wizard.nodesErr = validateWizardNodes(m.localClusterState.wizard.nodesStr)
		return m, nil, true
	}
	return m, nil, true
}

// validateWizardName returns "" when the name is valid, or a
// user-facing error otherwise. Conflict detection runs against the
// current cluster list so the validator can fire on every keystroke.
func (m Model) validateWizardName(name string) string {
	if name == "" {
		return "name is required"
	}
	if !wizardNameRe.MatchString(name) {
		return "lowercase letters, digits, dashes; <= 30 chars"
	}
	for _, c := range m.localClusterState.clusters {
		if c.Provider == m.localClusterState.wizard.provider && c.Name == name {
			return "name already in use"
		}
	}
	return ""
}

func validateWizardVersion(v string) string {
	if v == "" {
		return ""
	}
	if !wizardVersionRe.MatchString(v) {
		return "expected vX.Y or vX.Y.Z (e.g. v1.30.0)"
	}
	return ""
}

func validateWizardNodes(s string) (int, string) {
	if s == "" {
		return 0, "nodes is required"
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, "must be a number"
	}
	if n < 1 || n > 9 {
		return 0, "must be between 1 and 9"
	}
	return n, ""
}
