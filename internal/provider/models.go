// Copyright (c) Unikraft GmbH
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// svcGrpModel describes the data model for an instance's service group.
type svcGrpModel struct {
	UUID     types.String  `tfsdk:"uuid"`
	Name     types.String  `tfsdk:"name"`
	Services []svcModel    `tfsdk:"services"`
	Domains  []domainModel `tfsdk:"domains"`
}

// svcModel describes the data model for a service group's service.
type svcModel struct {
	Port            types.Int64 `tfsdk:"port"`
	DestinationPort types.Int64 `tfsdk:"destination_port"`
	Handlers        types.Set   `tfsdk:"handlers"`
}

type domainModel struct {
	// Name types.String `tfsdk:"name"`
	FQDN types.String `tfsdk:"fqdn"`
}

// netwIfaceModel describes the data model for an instance's network interface.
type netwIfaceModel struct {
	UUID      types.String `tfsdk:"uuid"`
	Name      types.String `tfsdk:"name"`
	PrivateIP types.String `tfsdk:"private_ip"`
	MAC       types.String `tfsdk:"mac"`
}

var netwIfaceModelType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"uuid":       types.StringType,
		"name":       types.StringType,
		"private_ip": types.StringType,
		"mac":        types.StringType,
	},
}

var ukcRefModelType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"uuid": types.StringType,
		"name": types.StringType,
	},
}

// ukcRefModel describes the data model for a generic resource reference.
// This is used when a relationship returns a UUID and Name only.
func ukcRefModel(uuid, name string) (attr.Value, diag.Diagnostics) {
	return types.ObjectValue(ukcRefModelType.AttrTypes, map[string]attr.Value{
		"uuid": types.StringValue(uuid),
		"name": types.StringValue(name),
	})
}

// int32ToInt converts an *int32 to an *int while preserving nil values.
func int32ToInt(i *int32) *int {
	if i == nil {
		return nil
	}
	v := int(*i)
	return &v
}
