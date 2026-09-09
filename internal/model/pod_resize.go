package model

// ContainerResources holds one container's CPU/memory request and limit
// values for the pods/resize subresource patch. Empty fields are omitted
// from the patch so a blank form field never clears an unset value.
type ContainerResources struct {
	Name       string
	CPURequest string
	CPULimit   string
	MemRequest string
	MemLimit   string
}
