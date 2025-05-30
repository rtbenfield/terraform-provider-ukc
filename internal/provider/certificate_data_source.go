package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	ukc "sdk.kraft.cloud"
	"sdk.kraft.cloud/certificates"
)

func NewCertificateDataSource() datasource.DataSource {
	return &CertificateDataSource{}
}

// CertificateDataSource defines the data source implementation.
type CertificateDataSource struct {
	client certificates.CertificatesService
}

// Ensure CertificateDataSource satisfies various datasource interfaces.
var _ datasource.DataSource = &CertificateDataSource{}

// CertificateDataSourceModel describes the data source data model.
type CertificateDataSourceModel struct {
	UUID types.String `tfsdk:"uuid"`

	CN            types.String `tfsdk:"cn"`
	CreatedAt     types.String `tfsdk:"created_at"`
	Issuer        types.String `tfsdk:"issuer"`
	Name          types.String `tfsdk:"name"`
	NotAfter      types.String `tfsdk:"not_after"`
	NotBefore     types.String `tfsdk:"not_before"`
	SerialNumber  types.String `tfsdk:"serial_number"`
	ServiceGroups types.List   `tfsdk:"service_groups"`
	Status        types.String `tfsdk:"status"`
	Subject       types.String `tfsdk:"subject"`
}

// Metadata implements datasource.DataSource.
func (d *CertificateDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

// Schema implements datasource.DataSource.
func (d *CertificateDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Retrieve information about a Unikraft Cloud certificate.",

		Attributes: map[string]schema.Attribute{
			"uuid": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the certificate to retrieve",
			},

			"cn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Common Name of the certificate",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Date and time of creation in ISO8601",
			},
			"issuer": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Certificate issuer (usually Let's Encrypt)",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the certificate to retrieve",
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
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`success` on success, or `error` if the request failed",
			},
			"subject": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Certificate subject",
			},
		},
	}
}

// Configure implements datasource.DataSource.
func (d *CertificateDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(ukc.KraftCloud)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected KraftCloud, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client.Certificates()
}

// Read implements datasource.DataSource.
func (d *CertificateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CertificateDataSourceModel
	var diag diag.Diagnostics

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.UUID.IsNull() {
		resp.Diagnostics.AddError(
			"Missing UUID",
			"UUID is required to read a certificate.",
		)
		return
	}

	// Get certificate by UUID
	uuid := data.UUID.ValueString()
	certRaw, err := d.client.Get(ctx, uuid)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to read certificate with UUID %s, got error: %v", uuid, err),
		)
		return
	}

	// Check if certificate was found
	if len(certRaw.Data.Entries) == 0 {
		resp.Diagnostics.AddError(
			"Certificate Not Found",
			fmt.Sprintf("No certificate found with UUID: %s", uuid),
		)
		return
	}

	// Map response body to model
	cert := certRaw.Data.Entries[0]

	data.CN = types.StringValue(cert.CommonName)
	data.CreatedAt = types.StringValue(cert.CreatedAt)
	data.Issuer = types.StringValue(cert.Issuer)
	data.Name = types.StringValue(cert.Name)
	data.NotAfter = types.StringValue(cert.NotAfter)
	data.NotBefore = types.StringValue(cert.NotBefore)
	data.SerialNumber = types.StringValue(cert.SerialNumber)
	data.Status = types.StringValue(cert.Status)
	data.Subject = types.StringValue(cert.Subject)

	serviceGroups := make([]attr.Value, len(cert.ServiceGroups))
	for i, svcGrp := range cert.ServiceGroups {
		serviceGroups[i], diag = ukcRefModel(svcGrp.UUID, svcGrp.Name)
		resp.Diagnostics.Append(diag...)
	}
	data.ServiceGroups, diag = types.ListValue(ukcRefModelType, serviceGroups)
	resp.Diagnostics.Append(diag...)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
