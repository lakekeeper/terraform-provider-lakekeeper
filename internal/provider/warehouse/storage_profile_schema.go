package warehouse

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// The storage_profile block is described once, below, and rendered in one of
// two modes:
//
//   - resource mode: Required/Optional, validators, plan modifiers and the
//     credential block;
//   - data-source mode: every attribute Computed, and no credential.

type fieldKind int

const (
	stringField fieldKind = iota
	boolField
	int64Field
)

// profileField describes one attribute of a storage profile.
type profileField struct {
	name        string
	kind        fieldKind
	description string
	// dataSourceDescription replaces description in data-source mode when set.
	dataSourceDescription string
	// required makes the attribute Required in resource mode; otherwise it is
	// Optional.
	required bool
	// computed makes the attribute also Computed in resource mode, keeping
	// the state value when the plan is unknown.
	computed         bool
	stringValidators []validator.String
	int64Validators  []validator.Int64
}

// profileDescription describes one storage profile (s3, adls or gcs).
type profileDescription struct {
	name        string
	description string
	fields      []profileField
	// credential returns the resource-only storage credential block.
	credential func() rschema.Attribute
}

func storageProfileDescriptions() []profileDescription {
	return []profileDescription{
		{
			name:        "s3",
			description: `S3 storage profile. Suitable for AWS or any service compatible with the S3 API.`,
			fields: []profileField{
				{name: "region", kind: stringField, required: true, description: "Region to use for S3 requests."},
				{
					name: "bucket", kind: stringField, required: true,
					description:      "The bucket name for the storage profile.",
					stringValidators: []validator.String{stringvalidator.LengthBetween(3, 64)},
				},
				{
					name: "sts_enabled", kind: boolField, required: true,
					description: "Whether to enable STS for S3 storage profile. Required if the storage type is `s3`. If enabled, the `sts_role_arn` or `assume_role_arn` must be provided.",
				},
				{name: "key_prefix", kind: stringField, description: "Subpath in the filesystem to use."},
				{
					name: "allow_alternative_protocols", kind: boolField, computed: true,
					description: "Allow `s3a://`, `s3n://`, `wasbs://` in locations. This is disabled by default. We do not recommend to use this setting except for migration.",
				},
				{name: "assume_role_arn", kind: stringField, description: "Optional ARN to assume when accessing the bucket from Lakekeeper for S3 storage profile"},
				{name: "aws_kms_key_arn", kind: stringField, description: "ARN of the KMS key used to encrypt the S3 bucket, if any."},
				{
					name: "endpoint", kind: stringField, computed: true,
					description: "Optional endpoint to use for S3 requests, if not provided the region will be used to determine the endpoint. If both region and endpoint are provided, the endpoint will be used. Example: `http://s3-de.my-domain.com:9000`",
					stringValidators: []validator.String{
						stringvalidator.RegexMatches(regexp.MustCompile("/$"), "Endpoint must ends with '/' character"),
					},
				},
				{
					name: "flavor", kind: stringField, computed: true,
					description:           fmt.Sprintf("S3 flavor to use. Defaults to `%s`. One of `%s` `%s`", api.S3FlavorAWS, api.S3FlavorS3Compat, api.S3FlavorAWS),
					dataSourceDescription: "S3 flavor to use. Defaults to `aws`.",
					stringValidators:      []validator.String{stringvalidator.OneOf(api.S3FlavorS3Compat, api.S3FlavorAWS)},
				},
				{
					name: "path_style_access", kind: boolField, computed: true,
					description: "Path style access for S3 requests. If the underlying S3 supports both, we recommend to not set path_style_access.",
				},
				{
					name: "push_s3_delete_disabled", kind: boolField, computed: true,
					description: "Controls whether the `s3.delete-enabled=false` flag is sent to clients.",
				},
				{
					name: "remote_signing_url_style", kind: stringField, computed: true,
					description:           fmt.Sprintf("S3 URL style detection mode for remote signing. One of `%s`, `%s`, `%s`. Default: `%s`.", api.RemoteSigningURLStyleAuto, api.RemoteSigningURLStylePath, api.RemoteSigningURLStyleVirtualHost, api.RemoteSigningURLStyleAuto),
					dataSourceDescription: "S3 URL style detection mode for remote signing. One of `auto`, `path-style`, `virtual-host`. Default: `auto`.",
					stringValidators: []validator.String{
						stringvalidator.OneOf(
							api.RemoteSigningURLStyleAuto,
							api.RemoteSigningURLStylePath,
							api.RemoteSigningURLStyleVirtualHost,
						),
					},
				},
				{name: "sts_role_arn", kind: stringField},
				{
					name: "sts_token_validity_seconds", kind: int64Field, computed: true,
					description:     "The validity of the STS tokens in seconds. Default is `3600`.",
					int64Validators: []validator.Int64{int64validator.AtLeast(0)},
				},
			},
			credential: s3CredentialSchema,
		},
		{
			name:        "adls",
			description: "ADLS storage profile. Suitable for Azure Data Lake Storage Gen2.",
			fields: []profileField{
				{name: "account_name", kind: stringField, required: true, description: "Name of the azure storage account."},
				{
					name: "filesystem", kind: stringField, required: true,
					description:      "Name of the adls filesystem, in blobstorage also known as container.",
					stringValidators: []validator.String{stringvalidator.LengthBetween(3, 64)},
				},
				{
					name: "allow_alternative_protocols", kind: boolField, computed: true,
					description: "Allow alternative protocols such as wasbs:// in locations. This is disabled by default. We do not recommend to use this setting except for migration.",
				},
				{
					name: "authority_host", kind: stringField, computed: true,
					description: "The authority host to use for authentication. Defaults to `https://login.microsoftonline.com`.",
				},
				{
					name: "host", kind: stringField, computed: true,
					description: "The host to use for the storage account. Defaults to `dfs.core.windows.net`.",
				},
				{name: "key_prefix", kind: stringField, description: "Subpath in the filesystem to use."},
				{
					name: "sas_token_validity_seconds", kind: int64Field, computed: true,
					description:     "The validity of the sas token in seconds. Default is `3600`.",
					int64Validators: []validator.Int64{int64validator.AtLeast(0)},
				},
			},
			credential: adlsCredentialSchema,
		},
		{
			name:        "gcs",
			description: "GCS storage profile. Designed for use with Google Cloud Storage",
			fields: []profileField{
				{
					name: "bucket", kind: stringField, required: true,
					description:      "The bucket name.",
					stringValidators: []validator.String{stringvalidator.LengthBetween(3, 64)},
				},
				{name: "key_prefix", kind: stringField, description: "Subpath in the filesystem to use."},
			},
			credential: gcsCredentialSchema,
		},
	}
}

// storageProfileResourceSchema renders the storage_profile block in resource mode.
func storageProfileResourceSchema() rschema.SingleNestedAttribute {
	profiles := map[string]rschema.Attribute{}
	for _, p := range storageProfileDescriptions() {
		attrs := map[string]rschema.Attribute{"credential": p.credential()}
		for _, f := range p.fields {
			attrs[f.name] = f.resourceAttribute()
		}
		profiles[p.name] = rschema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: p.description,
			Attributes:          attrs,
		}
	}

	return rschema.SingleNestedAttribute{
		Required:            true,
		MarkdownDescription: "Configure the storage profile. Only one Of `s3`, `adls` or `gcs` must be provided.",
		Attributes:          profiles,
	}
}

// storageProfileDataSourceSchema renders the storage_profile block in
// data-source mode.
func storageProfileDataSourceSchema() dschema.SingleNestedAttribute {
	profiles := map[string]dschema.Attribute{}
	for _, p := range storageProfileDescriptions() {
		attrs := map[string]dschema.Attribute{}
		for _, f := range p.fields {
			attrs[f.name] = f.dataSourceAttribute()
		}
		profiles[p.name] = dschema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: p.description,
			Attributes:          attrs,
		}
	}

	return dschema.SingleNestedAttribute{
		MarkdownDescription: "The storage profile of the warehouse. One of `s3`, `adls` or `gcs`.",
		Computed:            true,
		Attributes:          profiles,
	}
}

func (f profileField) resourceAttribute() rschema.Attribute {
	optional := !f.required
	switch f.kind {
	case boolField:
		a := rschema.BoolAttribute{Required: f.required, Optional: optional, Computed: f.computed, MarkdownDescription: f.description}
		if f.computed {
			a.PlanModifiers = []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}
		}
		return a
	case int64Field:
		a := rschema.Int64Attribute{Required: f.required, Optional: optional, Computed: f.computed, MarkdownDescription: f.description, Validators: f.int64Validators}
		if f.computed {
			a.PlanModifiers = []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
		}
		return a
	default:
		a := rschema.StringAttribute{Required: f.required, Optional: optional, Computed: f.computed, MarkdownDescription: f.description, Validators: f.stringValidators}
		if f.computed {
			a.PlanModifiers = []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
		}
		return a
	}
}

func (f profileField) dataSourceAttribute() dschema.Attribute {
	description := f.description
	if f.dataSourceDescription != "" {
		description = f.dataSourceDescription
	}
	switch f.kind {
	case boolField:
		return dschema.BoolAttribute{Computed: true, MarkdownDescription: description}
	case int64Field:
		return dschema.Int64Attribute{Computed: true, MarkdownDescription: description}
	default:
		return dschema.StringAttribute{Computed: true, MarkdownDescription: description}
	}
}

func s3CredentialSchema() rschema.Attribute {
	return rschema.SingleNestedAttribute{
		Required:            true,
		MarkdownDescription: "Configure the credentials to access the S3 storage. Only one of `access_key`, `cloudflare_r2` or `aws_system_identity` must be provided. This is not available for imported resources.",
		Attributes: map[string]rschema.Attribute{
			"access_key": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Authenticate to the S3 bucket with an Access Key",
				Attributes: map[string]rschema.Attribute{
					"access_key_id": rschema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "The access key ID. Required for `aws-access-key` credentials.",
					},
					"secret_access_key": rschema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "The secret access key. Required for `aws-access-key` credentials.",
					},
					"external_id": rschema.StringAttribute{
						Optional:            true,
						Sensitive:           true,
						MarkdownDescription: "The external ID.",
					},
				},
			},
			"cloudflare_r2": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Authenticate to a Cloudflare R2 Bucket",
				Attributes: map[string]rschema.Attribute{
					"access_key_id": rschema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "Access key ID used for IO operations of Lakekeeper",
					},
					"secret_access_key": rschema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "Secret key associated with the access key ID",
					},
					"account_id": rschema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "Cloudflare account ID, used to determine the temporary credentials",
					},
					"token": rschema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "Token associated with the access key ID",
					},
				},
			},
			"aws_system_identity": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Authenticate to the S3 bucket with AWS System Identity",
				Attributes: map[string]rschema.Attribute{
					"external_id": rschema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "Required for `aws-system-identity` credentials",
					},
				},
			},
		},
	}
}

func adlsCredentialSchema() rschema.Attribute {
	return rschema.SingleNestedAttribute{
		Required:            true,
		MarkdownDescription: "Configure the credentials to access the ADLS storage. One of `shared_access_key`, `client_credentials` or `azure_system_identity` must be provided. This is not available for imported resources.",
		Attributes: map[string]rschema.Attribute{
			"shared_access_key": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Authenticate to ADLS with Shared Access Key",
				Attributes: map[string]rschema.Attribute{
					"key": rschema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "The shared access key. Required for `azure-shared-access-key` credentials.",
					},
				},
			},
			"client_credentials": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Authenticate to ADLS with Client Credentials",
				Attributes: map[string]rschema.Attribute{
					"client_id": rschema.StringAttribute{
						Required:  true,
						Sensitive: true,
					},
					"client_secret": rschema.StringAttribute{
						Required:  true,
						Sensitive: true,
					},
					"tenant_id": rschema.StringAttribute{
						Required:  true,
						Sensitive: true,
					},
				},
			},
			"azure_system_identity": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Authenticate to ADLS with Azure System Identity",
				Attributes: map[string]rschema.Attribute{
					"enabled": rschema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "This is just an helper to check if the Azure System Identity is activated for this storage profile.",
						Default:             booldefault.StaticBool(true),
					},
				},
			},
		},
	}
}

func gcsCredentialSchema() rschema.Attribute {
	return rschema.SingleNestedAttribute{
		Required:            true,
		MarkdownDescription: "Configure the credentials to access the GCS storage. One of `service_account_key` or `gcp_system_identity` must be provided. This is not available for imported resources.",
		Attributes: map[string]rschema.Attribute{
			"service_account_key": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Authenticate to the GCS bucket with a Service Account Key",
				Attributes: map[string]rschema.Attribute{
					"key": rschema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "Required for `service-account-key` credentials.",
					},
				},
			},
			"gcp_system_identity": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Authenticate to the GCS bucket with GCP System Identity",
				Attributes: map[string]rschema.Attribute{
					"enabled": rschema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "This is just an helper to check if the GCP System Identity is activated for this storage profile.",
						Default:             booldefault.StaticBool(true),
					},
				},
			},
		},
	}
}
