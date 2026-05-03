package k8s

import (
	"fmt"
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
)

func populateServiceDetails(ti *model.Item, status, spec map[string]any) {
	if spec == nil {
		return
	}
	if svcType, ok := spec["type"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Type", Value: svcType})
	}
	if clusterIP, ok := spec["clusterIP"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Cluster IP", Value: clusterIP})
	}
	populateServicePorts(ti, spec)
	populateServiceExternalIPs(ti, spec)
	populateLoadBalancerAddresses(ti, status, "External Address")
	populateServiceSelector(ti, spec)
	if spec["sessionAffinity"] != nil {
		if sa, ok := spec["sessionAffinity"].(string); ok && sa != "None" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Session Affinity", Value: sa})
		}
	}
}

func populateServicePorts(ti *model.Item, spec map[string]any) {
	ports, ok := spec["ports"].([]any)
	if !ok {
		return
	}
	var portStrs []string
	for _, p := range ports {
		if pMap, ok := p.(map[string]any); ok {
			port := getInt(pMap, "port")
			nodePort := getInt(pMap, "nodePort")
			targetPort := getInt(pMap, "targetPort")
			proto, _ := pMap["protocol"].(string)
			head := fmt.Sprintf("%d", port)
			if nodePort > 0 {
				head = fmt.Sprintf("%d:%d", port, nodePort)
			}
			s := fmt.Sprintf("%s/%s", head, proto)
			if targetPort > 0 && targetPort != port {
				s = fmt.Sprintf("%s→%d/%s", head, targetPort, proto)
			}
			portStrs = append(portStrs, s)
		}
	}
	if len(portStrs) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ports", Value: strings.Join(portStrs, ", ")})
	}
}

func populateServiceExternalIPs(ti *model.Item, spec map[string]any) {
	extIPs, ok := spec["externalIPs"].([]any)
	if !ok || len(extIPs) == 0 {
		return
	}
	var ips []string
	for _, ip := range extIPs {
		if s, ok := ip.(string); ok {
			ips = append(ips, s)
		}
	}
	if len(ips) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "External IPs", Value: strings.Join(ips, ", ")})
	}
}

func populateLoadBalancerAddresses(ti *model.Item, status map[string]any, columnKey string) {
	if status == nil {
		return
	}
	lb, ok := status["loadBalancer"].(map[string]any)
	if !ok {
		return
	}
	ingress, ok := lb["ingress"].([]any)
	if !ok {
		return
	}
	var addrs []string
	for _, i := range ingress {
		if iMap, ok := i.(map[string]any); ok {
			if ip, ok := iMap["ip"].(string); ok {
				addrs = append(addrs, ip)
			} else if host, ok := iMap["hostname"].(string); ok {
				addrs = append(addrs, host)
			}
		}
	}
	if len(addrs) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: columnKey, Value: strings.Join(addrs, ", ")})
	}
}

func populateServiceSelector(ti *model.Item, spec map[string]any) {
	selector, ok := spec["selector"].(map[string]any)
	if !ok {
		return
	}
	var parts []string
	for k, v := range selector {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(parts)
	if len(parts) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Selector", Value: strings.Join(parts, ", ")})
	}
}

func populateIngressDetails(ti *model.Item, status, spec map[string]any) {
	if spec == nil {
		return
	}
	if ic, ok := spec["ingressClassName"].(string); ok && ic != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ingress Class", Value: ic})
	}
	populateIngressRulesAndHosts(ti, spec)
	populateIngressDefaultBackend(ti, spec)
	tlsHostSet := populateIngressTLSHosts(ti, spec)
	populateIngressURL(ti, spec, tlsHostSet)
	populateLoadBalancerAddresses(ti, status, "Address")
}

func populateIngressRulesAndHosts(ti *model.Item, spec map[string]any) {
	rules, ok := spec["rules"].([]any)
	if !ok {
		return
	}
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Rules", Value: fmt.Sprintf("%d", len(rules))})
	var hosts []string
	for _, r := range rules {
		if rMap, ok := r.(map[string]any); ok {
			if host, ok := rMap["host"].(string); ok {
				hosts = append(hosts, host)
			}
		}
	}
	if len(hosts) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Hosts", Value: strings.Join(hosts, ", ")})
	}
}

func populateIngressDefaultBackend(ti *model.Item, spec map[string]any) {
	defBackend, ok := spec["defaultBackend"].(map[string]any)
	if !ok {
		return
	}
	svc, ok := defBackend["service"].(map[string]any)
	if !ok {
		return
	}
	svcName, _ := svc["name"].(string)
	if port, ok := svc["port"].(map[string]any); ok {
		if num, ok := port["number"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Default Backend", Value: fmt.Sprintf("%s:%d", svcName, int64(num))})
		} else if name, ok := port["name"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Default Backend", Value: fmt.Sprintf("%s:%s", svcName, name)})
		}
	} else if svcName != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Default Backend", Value: svcName})
	}
}

func populateIngressTLSHosts(ti *model.Item, spec map[string]any) map[string]bool {
	tls, ok := spec["tls"].([]any)
	if !ok || len(tls) == 0 {
		return nil
	}
	tlsHostSet := make(map[string]bool)
	var tlsHosts []string
	for _, t := range tls {
		if tMap, ok := t.(map[string]any); ok {
			if hosts, ok := tMap["hosts"].([]any); ok {
				for _, h := range hosts {
					if s, ok := h.(string); ok {
						tlsHosts = append(tlsHosts, s)
						tlsHostSet[s] = true
					}
				}
			}
		}
	}
	if len(tlsHosts) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "TLS Hosts", Value: strings.Join(tlsHosts, ", ")})
	}
	return tlsHostSet
}

func populateIngressURL(ti *model.Item, spec map[string]any, tlsHostSet map[string]bool) {
	rules, ok := spec["rules"].([]any)
	if !ok || len(rules) == 0 {
		return
	}
	firstRule, ok := rules[0].(map[string]any)
	if !ok {
		return
	}
	host, ok := firstRule["host"].(string)
	if !ok || host == "" {
		return
	}
	scheme := "http"
	if tlsHostSet[host] {
		scheme = "https"
	}
	path := ""
	if httpBlock, ok := firstRule["http"].(map[string]any); ok {
		if paths, ok := httpBlock["paths"].([]any); ok && len(paths) > 0 {
			if firstPath, ok := paths[0].(map[string]any); ok {
				if p, ok := firstPath["path"].(string); ok && p != "" && p != "/" {
					path = p
				}
			}
		}
	}
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "__ingress_url", Value: scheme + "://" + host + path})
}
