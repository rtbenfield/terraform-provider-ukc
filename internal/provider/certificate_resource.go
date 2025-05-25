// Copyright (c) Unikraft GmbH
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"sdk.kraft.cloud/certificates"
)

func NewCertificateResource() resource.Resource {
	return &CertificateResource{}
}

type CertificateResource struct {
	client certificates.CertificatesService
}

// Ensure CertificateResource satisfies various resource interfaces.
var (
	_ resource.Resource                = &CertificateResource{}
	_ resource.ResourceWithImportState = &CertificateResource{}
)

// CertificateResourceModel describes the resource data model.
type CertificateResourceModel struct {
	CN    types.String `tfsdk:"cn"`
	Chain types.String `tfsdk:"chain"`
	Name  types.String `tfsdk:"name"`
	PKey  types.String `tfsdk:"pkey"`

	CreatedAt     types.String         `tfsdk:"created_at"`
	Issuer        types.String         `tfsdk:"issuer"`
	NotAfter      types.String         `tfsdk:"not_after"`
	NotBefore     types.String         `tfsdk:"not_before"`
	SerialNumber  types.String         `tfsdk:"serial_number"`
	ServiceGroups []ukcRefModel        `tfsdk:"service_groups"`
	Status        types.String         `tfsdk:"status"`
	Subject       types.String         `tfsdk:"subject"`
	UUID          types.String         `tfsdk:"uuid"`
	Validation    *certValidationModel `tfsdk:"validation"`
}

type certValidationModel struct {
	Attempt types.Int32  `tfsdk:"attempt"`
	Next    types.String `tfsdk:"next"`
}

// Metadata implements resource.Resource.
func (r *CertificateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

// Schema implements resource.Resource.
func (r *CertificateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Allows the creation of Unikraft Cloud certificates.",

		Attributes: map[string]schema.Attribute{
			// input fields
			"cn": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Common Name of the certificate",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"chain": schema.StringAttribute{
				Required:            true,
				WriteOnly:           true,
				MarkdownDescription: "Chain of the certificate in PEM format",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the certificate",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pkey": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				WriteOnly:           true,
				MarkdownDescription: "Private key of the certificate in PEM format",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// computed fields
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Date and time of creation in ISO8601",
			},
			"issuer": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Certificate issuer (usually Let's Encrypt)",
			},
			"not_before": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Date and time of beginning of validity in ISO8601",
			},
			"not_after": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Expiration date and time in ISO8601",
			},
			"serial_number": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Certificate serial number",
			},
			"service_groups": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Services using this certificate",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "UUID of the service",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Name of the service",
						},
					},
				},
			},
			"subject": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Certificate subject",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`success` on success, or `error` if the request failed",
			},
			"uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID of the certificate",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"validation": schema.SingleNestedAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Validation status (only while `pending`)",
				Attributes: map[string]schema.Attribute{
					"attempt": schema.Int32Attribute{
						Computed:            true,
						MarkdownDescription: "Number of validation attempts made",
					},
					"next": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Date and time of next validation attempt in ISO8601",
					},
				},
			},
		},
	}
}

// Configure implements resource.Resource.
func (r *CertificateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(certificates.CertificatesService)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected certificates.CertificatesService, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create implements resource.Resource.
func (r *CertificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CertificateResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	certReq := certificates.CreateRequest{
		CN:    data.CN.ValueString(),
		Chain: data.Chain.ValueString(),
		Name:  data.Name.ValueString(),
		PKey:  data.PKey.ValueString(),
	}

	certRaw, err := r.client.Create(ctx, &certReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Failed to create certificate, got error: %v", err),
		)
		return
	}
	cert := certRaw.Data.Entries[0]

	// Not all attributes are returned by CreateCertificate
	certRawFull, err := r.client.Get(ctx, cert.UUID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Failed to get certificate state, got error: %v", err),
		)
		return
	}
	certFull := certRawFull.Data.Entries[0]

	r.inflate(&data, &certFull)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read implements resource.Resource.
func (r *CertificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CertificateResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get certificate by UUID
	uuid := data.UUID.ValueString()
	certRaw, err := r.client.Get(ctx, uuid)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Failed to get certificate state with UUID %s, got error: %v", uuid, err),
		)
		return
	}
	cert := certRaw.Data.Entries[0]

	r.inflate(&data, &cert)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource.
func (r *CertificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Certificate Update Not Supported",
		"Certificates cannot be updated. To change certificate properties, the resource must be recreated.",
	)
}

// Delete implements resource.Resource.
func (r *CertificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CertificateResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call the Delete API
	uuid := data.UUID.ValueString()
	_, err := r.client.Delete(ctx, uuid)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Failed to delete certificate with UUID %s, got error: %v", uuid, err),
		)
		return
	}

}

// ImportState implements resource.Resource.
func (r *CertificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("uuid"), req, resp)
}

func (r *CertificateResource) inflate(model *CertificateResourceModel, cert *certificates.GetResponseItem) {
	model.CN = types.StringValue(cert.CommonName)
	model.CreatedAt = types.StringValue(cert.CreatedAt)
	model.Issuer = types.StringValue(cert.Issuer)
	model.Name = types.StringValue(cert.Name)
	model.NotAfter = types.StringValue(cert.NotAfter)
	model.NotBefore = types.StringValue(cert.NotBefore)
	model.SerialNumber = types.StringValue(cert.SerialNumber)
	model.Status = types.StringValue(cert.Status)
	model.Subject = types.StringValue(cert.Subject)
	model.UUID = types.StringValue(cert.UUID)

	model.ServiceGroups = make([]ukcRefModel, len(cert.ServiceGroups))
	for i, svcGrp := range cert.ServiceGroups {
		model.ServiceGroups[i] = ukcRefModel{
			UUID: types.StringValue(svcGrp.UUID),
			Name: types.StringValue(svcGrp.Name),
		}
	}

	if cert.Validation != nil {
		model.Validation = &certValidationModel{
			Attempt: types.Int32Value(int32(cert.Validation.Attempt)),
			Next:    types.StringValue(cert.Validation.Next),
		}
	}
}
