package provider

import (
	"context"
	"fmt"
	"math"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
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
	Domains   []ServiceDomainModel  `tfsdk:"domains"`
	HardLimit types.Int32           `tfsdk:"hard_limit"`
	Name      types.String          `tfsdk:"name"`
	Services  []ServiceServiceModel `tfsdk:"services"`
	SoftLimit types.Int32           `tfsdk:"soft_limit"`
	UUID      types.String          `tfsdk:"uuid"`
}

type ServiceDomainModel struct {
	Certificate types.Map    `tfsdk:"certificate"`
	FQDN        types.String `tfsdk:"fqdn"`
	Name        types.String `tfsdk:"name"`
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
							WriteOnly:           true,
							MarkdownDescription: "Publicly accessible domain name",
						},
						"certificate": schema.MapNestedAttribute{
							Optional:            true,
							MarkdownDescription: "TLS certificate to use for the domain",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"uuid": schema.StringAttribute{
										Computed:            true,
										Optional:            true,
										MarkdownDescription: "UUID of the certificate",
									},
									"name": schema.StringAttribute{
										Computed:            true,
										Optional:            true,
										MarkdownDescription: "Name of the certificate",
									},

									// Return-only attributes
									"state": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "State of the certificate",
									},
								},
							},
						},

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
				Optional:            true,
				WriteOnly:           true,
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
				Optional:            true,
				WriteOnly:           true,
				MarkdownDescription: "Per-instance connection [soft limit](https://unikraft.cloud/docs/api/v1/services/#limits). Defaults to 1",
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplaceIfConfigured(),
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

	client, ok := req.ProviderData.(services.ServicesService)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected services.ServicesService, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create implements resource.Resource.
func (r *ServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := services.CreateRequest{
		Domains:   make([]services.CreateRequestDomain, len(data.Domains)),
		HardLimit: ptr(int(data.HardLimit.ValueInt32())),
		Name:      data.Name.ValueStringPointer(),
		Services:  make([]services.CreateRequestService, len(data.Services)),
		SoftLimit: ptr(int(data.SoftLimit.ValueInt32())),
	}

	for i, domain := range data.Domains {
		in.Domains[i] = services.CreateRequestDomain{
			Name: domain.Name.ValueString(),
		}
		// TODO: implement certificate
		// if domain.Certificate != nil {
		// 	in.Domains[i].Certificate = domain.Certificate
		// }
	}

	for i, service := range data.Services {
		in.Services[i].Port = int(service.Port.ValueInt32())
		in.Services[i].DestinationPort = ptr(int(service.DestinationPort.ValueInt32()))
		in.Services[i].Handlers = make([]services.Handler, 0, len(service.Handlers.Elements()))

		handlers := make([]types.String, 0, len(service.Handlers.Elements()))
		resp.Diagnostics.Append(service.Handlers.ElementsAs(ctx, &handlers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for j, handler := range handlers {
			in.Services[i].Handlers[j] = services.Handler(handler.ValueString())
		}
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
	var data InstanceResourceModel

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

	return diag
}
