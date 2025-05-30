package provider

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	ukc "sdk.kraft.cloud"
	"sdk.kraft.cloud/services"
)

func NewServiceResource() resource.Resource {
	return &ServiceResource{}
}

// ServiceResource defines the resource implementation.
type ServiceResource struct {
	client services.ServicesService
}

// Ensure ServiceResource satisfies various resource interfaces.
var (
	_ resource.Resource                = &ServiceResource{}
	_ resource.ResourceWithImportState = &ServiceResource{}
)

// ServiceResourceModel describes the resource data model.
type ServiceResourceModel struct {
	Domains   types.List   `tfsdk:"domains"`
	HardLimit types.Int32  `tfsdk:"hard_limit"`
	Name      types.String `tfsdk:"name"`
	Services  types.List   `tfsdk:"services"`
	SoftLimit types.Int32  `tfsdk:"soft_limit"`

	UUID types.String `tfsdk:"uuid"`
}

type ServiceDomainModel struct {
	FQDN types.String `tfsdk:"fqdn"`
	// Certificate types.Map    `tfsdk:"certificate"`
	Name types.String `tfsdk:"name"`
}

type ServiceServiceModel struct {
	Port            types.Int32 `tfsdk:"port"`
	DestinationPort types.Int32 `tfsdk:"destination_port"`
	Handlers        types.Set   `tfsdk:"handlers"`
}

// Metadata implements resource.Resource.
func (r *ServiceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

// Schema implements resource.Resource.
func (r *ServiceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Allows the creation of Unikraft Cloud services.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"domains": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Publicly accessible domain name",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplaceIfConfigured(),
							},
						},
						// "certificate": schema.MapNestedAttribute{
						// 	NestedObject: schema.NestedAttributeObject{
						// 	MarkdownDescription: "TLS certificate to use for the domain",
						// 		Attributes: map[string]schema.Attribute{
						// 			"uuid": schema.StringAttribute{
						// 				Computed:            true,
						// 				Optional:            true,
						// 			},
						// 				MarkdownDescription: "UUID of the certificate",
						// 			"name": schema.StringAttribute{
						// 				Computed:            true,
						// 				Optional:            true,
						// 			},
						// 				MarkdownDescription: "Name of the certificate",

						// 			// Return-only attributes
						// 			"state": schema.StringAttribute{
						// 				Computed:            true,
						// 			},
						// 				MarkdownDescription: "State of the certificate",
						// 		},
						// 	},
						// },

						// Return-only attributes
						"fqdn": schema.StringAttribute{
							Computed:            true,
							Optional:            true,
							MarkdownDescription: "Public fully-qualified domain name under which the service is accessible from the Internet",
						},
					},
				},
			},
			"hard_limit": schema.Int32Attribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Per-instance connection [hard limit](https://unikraft.cloud/docs/api/v1/services/#limits). Defaults to 65535",
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"services": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"port": schema.Int32Attribute{
							Required:            true,
							MarkdownDescription: "Public-facing port",
							Validators: []validator.Int32{
								int32validator.Between(1, math.MaxUint16),
							},
							PlanModifiers: []planmodifier.Int32{
								int32planmodifier.RequiresReplace(),
							},
						},
						"destination_port": schema.Int32Attribute{
							Computed:            true,
							Optional:            true,
							MarkdownDescription: "Port that the application listens on. Default is the same as `port`",
							Validators: []validator.Int32{
								int32validator.Between(1, math.MaxUint16),
							},
							PlanModifiers: []planmodifier.Int32{
								int32planmodifier.RequiresReplaceIfConfigured(),
								int32planmodifier.UseStateForUnknown(),
							},
						},
						"handlers": schema.SetAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "See [Unikraft Cloud docs](https://unikraft.cloud/docs/api/v1/services/#handlers)",
							Optional:            true,
							PlanModifiers: []planmodifier.Set{
								setplanmodifier.RequiresReplaceIfConfigured(),
								setplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"soft_limit": schema.Int32Attribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Per-instance connection [soft limit](https://unikraft.cloud/docs/api/v1/services/#limits). Defaults to 1",
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplaceIfConfigured(),
				},
			},

			"uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID of the service",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure implements resource.Resource.
func (r *ServiceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(ukc.KraftCloud)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected ukc.KraftCloud, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client.Services()
}

// Create implements resource.Resource.
func (r *ServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var domains []ServiceDomainModel
	resp.Diagnostics.Append(data.Domains.ElementsAs(ctx, &domains, false)...)
	var svcs []ServiceServiceModel
	resp.Diagnostics.Append(data.Services.ElementsAs(ctx, &svcs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := services.CreateRequest{
		Domains: make([]services.CreateRequestDomain, len(domains)),
		// HardLimit: int32ToInt(data.HardLimit.ValueInt32Pointer()),
		Name:     data.Name.ValueStringPointer(),
		Services: make([]services.CreateRequestService, len(svcs)),
		// SoftLimit: int32ToInt(data.SoftLimit.ValueInt32Pointer()),
	}

	for i, domain := range domains {
		in.Domains[i] = services.CreateRequestDomain{
			Name: domain.Name.ValueString(),
		}
	}

	for i, service := range svcs {
		var handlers []services.Handler
		resp.Diagnostics.Append(service.Handlers.ElementsAs(ctx, &handlers, false)...)
		in.Services[i] = services.CreateRequestService{
			Port:            int(service.Port.ValueInt32()),
			DestinationPort: ptr(int(service.DestinationPort.ValueInt32())),
			Handlers:        handlers,
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.Create(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Failed to create service, got error: %v", err),
		)
		return
	}
	service := response.Data.Entries[0]

	// Fetch the latest resource state
	// Not all values are returned by the create request
	resp.Diagnostics.Append(r.read(ctx, service.UUID, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read implements resource.Resource.
func (r *ServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch the latest resource state
	resp.Diagnostics.Append(r.read(ctx, data.UUID.ValueString(), &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource.
func (r *ServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Unsupported",
		"This resource does not support updates. Configuration changes were expected to have triggered a replacement "+
			"of the resource. Please report this issue to the provider developers.",
	)
}

// Delete implements resource.Resource.
func (r *ServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := data.UUID.ValueString()
	_, err := r.client.Delete(ctx, uuid)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Failed to delete service with UUID %s, got error: %v", uuid, err),
		)
		return
	}
}

// ImportState implements resource.ResourceWithImportState.
func (r *ServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("uuid"), req, resp)
}

func (r *ServiceResource) read(ctx context.Context, uuid string, data *ServiceResourceModel) diag.Diagnostics {
	var diag diag.Diagnostics

	response, err := r.client.Get(ctx, uuid)
	if err != nil {
		diag.AddError(
			"Client Error",
			fmt.Sprintf("Failed to get service state, got error: %v", err),
		)
		return diag
	}
	service := response.Data.Entries[0]

	// Map scalar values
	data.Name = types.StringValue(service.Name)
	data.HardLimit = types.Int32Value(int32(service.HardLimit))
	data.SoftLimit = types.Int32Value(int32(service.SoftLimit))
	data.UUID = types.StringValue(uuid)

	// Map domains
	// We need to handle the special case where 'name' is required for input but not returned in the response
	// Instead, we get 'fqdn' in the response
	// First, get the existing domains from state to preserve the 'name' values
	var domainsFromState []ServiceDomainModel
	diag.Append(data.Domains.ElementsAs(ctx, &domainsFromState, false)...)

	// Create domain objects for the response
	domains := make([]attr.Value, len(service.Domains))
	for i, domain := range service.Domains {
		// Find if we have a matching domain in the state
		var nameValue attr.Value
		for _, stateDomain := range domainsFromState {
			// Try to match based on FQDN if it exists in state
			if !stateDomain.FQDN.IsNull() && stateDomain.FQDN.ValueString() == domain.FQDN {
				// FQDN == FQDN
				nameValue = types.StringValue(stateDomain.Name.ValueString())
				break
			} else if !stateDomain.Name.IsNull() && strings.HasSuffix(stateDomain.Name.ValueString(), ".") && strings.TrimRight(stateDomain.Name.ValueString(), ".") == domain.FQDN {
				// Name is an FQDN with a trailing `.` and should be an exact match
				nameValue = types.StringValue(stateDomain.Name.ValueString())
				break
			} else if !stateDomain.Name.IsNull() && strings.HasPrefix(domain.FQDN, stateDomain.Name.ValueString()) {
				// Name is a prefix of the FQDN
				nameValue = types.StringValue(stateDomain.Name.ValueString())
				break
			} else {
				// Must be a new domain that is not in the state
				nameValue = types.StringNull()
			}
		}

		// Create the domain object
		domainObj, diags := types.ObjectValue(
			map[string]attr.Type{
				"fqdn": types.StringType,
				"name": types.StringType,
			},
			map[string]attr.Value{
				"fqdn": types.StringValue(domain.FQDN),
				"name": nameValue,
			},
		)
		diag.Append(diags...)
		domains[i] = domainObj
	}

	// Set the domains list
	domainsList, diags := types.ListValue(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"fqdn": types.StringType,
				"name": types.StringType,
			},
		},
		domains,
	)
	diag.Append(diags...)
	data.Domains = domainsList

	// Map services
	services := make([]attr.Value, len(service.Services))
	for i, svc := range service.Services {
		// Convert handlers to a Set
		handlers, diags := types.SetValueFrom(ctx, types.StringType, svc.Handlers)
		diag.Append(diags...)

		// Create the service object
		serviceObj, diags := types.ObjectValue(
			map[string]attr.Type{
				"port":             types.Int32Type,
				"destination_port": types.Int32Type,
				"handlers":         types.SetType{ElemType: types.StringType},
			},
			map[string]attr.Value{
				"port":             types.Int32Value(int32(svc.Port)),
				"destination_port": types.Int32Value(int32(svc.DestinationPort)),
				"handlers":         handlers,
			},
		)
		diag.Append(diags...)
		services[i] = serviceObj
	}

	// Set the services list
	servicesList, diags := types.ListValue(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"port":             types.Int32Type,
				"destination_port": types.Int32Type,
				"handlers":         types.SetType{ElemType: types.StringType},
			},
		},
		services,
	)
	diag.Append(diags...)
	data.Services = servicesList

	return diag
}
