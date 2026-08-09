package k8s

import (
	"os"
	"os/user"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"

	"github.com/janosmiko/lfk/internal/version"
)

// FieldManagerOverride replaces the derived manager name. Set from the
// field_manager config key; an empty value keeps the default "lfk:<user>".
var FieldManagerOverride string

// unknownUser stands in when the OS gives no username.
const unknownUser = "unknown"

// FieldManager returns the name lfk stamps on every write. The apiserver
// records it in metadata.managedFields, which is what the YAML blame view
// reads, so the name carries the person and not only the tool.
//
// The version stays out of the name on purpose: managedFields entries are
// keyed by manager, so a version here would leave a dead entry behind on
// every release. The version travels in the User-Agent instead.
func FieldManager() string {
	return buildFieldManager(FieldManagerOverride, currentUser())
}

// UserAgent returns the agent string lfk sends on every request. The
// apiserver writes it to the audit log.
func UserAgent() string {
	return buildUserAgent(version.Short(), runtime.GOOS, runtime.GOARCH, currentUser(), currentHost())
}

func buildFieldManager(override, username string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return cut(sanitizeManager(trimmed), metav1validation.FieldManagerMaxLength)
	}
	name := sanitizeManager(stripDomain(username))
	if name == "" {
		name = unknownUser
	}
	return cut("lfk:"+name, metav1validation.FieldManagerMaxLength)
}

func buildUserAgent(ver, goos, goarch, username, host string) string {
	agent := "lfk/" + sanitizeHeader(ver) + " (" + goos + "/" + goarch + ")"
	name := sanitizeHeader(stripDomain(username))
	if name == "" {
		return agent
	}
	agent += " " + name
	if h := sanitizeHeader(host); h != "" {
		agent += "@" + h
	}
	return agent
}

// sanitizeManager keeps the characters that read well in a blame view and
// replaces the rest, so a shell-hostile or multi-line username cannot reach
// the API field verbatim.
func sanitizeManager(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune("-_.:@", r):
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

// sanitizeHeader drops control characters. A newline in the agent string
// would split the HTTP header.
func sanitizeHeader(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// stripDomain removes a Windows domain prefix such as "CORP\jmiko".
func stripDomain(username string) string {
	if i := strings.LastIndex(username, `\`); i >= 0 {
		return username[i+1:]
	}
	return username
}

// cut trims s to at most limit bytes without splitting a rune.
func cut(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	for i := limit; i > 0; i-- {
		if utf8.RuneStart(s[i]) {
			return s[:i]
		}
	}
	return ""
}

var identity = sync.OnceValues(func() (string, string) {
	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	if name == "" {
		name = os.Getenv("USER")
	}
	if name == "" {
		name = os.Getenv("USERNAME")
	}
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	return name, host
})

func currentUser() string {
	name, _ := identity()
	return name
}

func currentHost() string {
	_, host := identity()
	return host
}
