package v2

import (
	"encoding/json"
	"time"
)

// UnmarshalJSON unmarshals a LoadBalancer resource into a temporary structure whose "CreatedAt" field of type
// string to be able to parse the original timestamp (ISO 8601) into a time.Time object, since json.Unmarshal()
// only supports RFC3339 format.
func (lb *LoadBalancer) UnmarshalJSON(data []byte) error {
	var raw = struct {
		CreatedAt   string                 `json:"created-at,omitempty"`
		Description *string                `json:"description,omitempty"`
		Id          *string                `json:"id,omitempty"` // nolint:golint
		Ip          *string                `json:"ip,omitempty"` // nolint:golint
		Name        *string                `json:"name,omitempty"`
		Services    *[]LoadBalancerService `json:"services,omitempty"`
		State       *string                `json:"state,omitempty"`
	}{}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	createdAt, err := time.Parse(iso8601Format, raw.CreatedAt)
	if err != nil {
		return err
	}

	lb.CreatedAt = &createdAt
	lb.Description = raw.Description
	lb.Id = raw.Id
	lb.Ip = raw.Ip
	lb.Name = raw.Name
	lb.Services = raw.Services
	lb.State = raw.State

	return nil
}
