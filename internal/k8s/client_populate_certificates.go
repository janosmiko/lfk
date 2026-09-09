package k8s

import (
	"encoding/pem"
	"fmt"

	"github.com/janosmiko/lfk/internal/model"
)

// notBefore/notAfter are copied verbatim, matching cert-manager Certificate's
// "Expires" column (no relative-time conversion).
func populatePodCertificateRequest(ti *model.Item, spec, status map[string]any) {
	if spec != nil {
		if podName, ok := spec["podName"].(string); ok && podName != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Pod", Value: podName})
		}
		if signerName, ok := spec["signerName"].(string); ok && signerName != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Signer", Value: signerName})
		}
	}
	if status != nil {
		if notBefore, ok := status["notBefore"].(string); ok && notBefore != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Issued", Value: notBefore})
		}
		if notAfter, ok := status["notAfter"].(string); ok && notAfter != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Expires", Value: notAfter})
		}
	}
}

// Signer is always shown, even empty, since an anonymous bundle is valid.
func populateClusterTrustBundle(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	signerName, _ := spec["signerName"].(string)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Signer", Value: signerName})

	trustBundle, _ := spec["trustBundle"].(string)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Certificates", Value: fmt.Sprintf("%d", countPEMCertificates(trustBundle))})
}

// countPEMCertificates counts CERTIFICATE-type PEM blocks in data. Malformed
// or non-PEM input decodes to zero blocks.
func countPEMCertificates(data string) int {
	count := 0
	rest := []byte(data)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			count++
		}
	}
	return count
}
