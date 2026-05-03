package k8s

import (
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

func appendIfUnseen(items *[]model.Item, seen map[string]bool, kind, name, namespace string, createdAt time.Time) {
	key := kind + "/" + name
	if seen[key] {
		return
	}
	seen[key] = true
	*items = append(*items, model.Item{
		Name:      name,
		Kind:      kind,
		Namespace: namespace,
		CreatedAt: createdAt,
		Age:       formatAge(time.Since(createdAt)),
	})
}
