package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) GetContainerPorts(ctx context.Context, contextName, namespace, podName string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s: %w", podName, err)
	}

	var ports []ContainerPort
	for _, container := range pod.Spec.Containers {
		for _, p := range container.Ports {
			ports = append(ports, ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}
	}
	return ports, nil
}

func (c *Client) GetServicePorts(ctx context.Context, contextName, namespace, svcName string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	svc, err := cs.CoreV1().Services(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting service %s: %w", svcName, err)
	}

	ports := make([]ContainerPort, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		ports = append(ports, ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      string(p.Protocol),
		})
	}
	return ports, nil
}

func (c *Client) GetDeploymentPorts(ctx context.Context, contextName, namespace, name string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	dep, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment %s: %w", name, err)
	}

	var ports []ContainerPort
	for _, container := range dep.Spec.Template.Spec.Containers {
		for _, p := range container.Ports {
			ports = append(ports, ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}
	}
	return ports, nil
}

func (c *Client) GetStatefulSetPorts(ctx context.Context, contextName, namespace, name string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	sts, err := cs.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting statefulset %s: %w", name, err)
	}

	var ports []ContainerPort
	for _, container := range sts.Spec.Template.Spec.Containers {
		for _, p := range container.Ports {
			ports = append(ports, ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}
	}
	return ports, nil
}

func (c *Client) GetDaemonSetPorts(ctx context.Context, contextName, namespace, name string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	ds, err := cs.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting daemonset %s: %w", name, err)
	}

	var ports []ContainerPort
	for _, container := range ds.Spec.Template.Spec.Containers {
		for _, p := range container.Ports {
			ports = append(ports, ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}
	}
	return ports, nil
}
