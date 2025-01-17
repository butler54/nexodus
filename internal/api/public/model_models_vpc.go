/*
Nexodus API

This is the Nexodus API Server.

API version: 1.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package public

// ModelsVPC struct for ModelsVPC
type ModelsVPC struct {
	Description    string `json:"description,omitempty"`
	Id             string `json:"id,omitempty"`
	Ipv4Cidr       string `json:"ipv4_cidr,omitempty"`
	Ipv6Cidr       string `json:"ipv6_cidr,omitempty"`
	OrganizationId string `json:"organization_id,omitempty"`
	PrivateCidr    bool   `json:"private_cidr,omitempty"`
}
