package patchedclient

import (
	ukc "sdk.kraft.cloud"
	"sdk.kraft.cloud/certificates"
	"sdk.kraft.cloud/images"
	"sdk.kraft.cloud/instances"
	"sdk.kraft.cloud/metros"
	"sdk.kraft.cloud/services"
	"sdk.kraft.cloud/services/autoscale"
	"sdk.kraft.cloud/users"
	"sdk.kraft.cloud/volumes"
)

// NewPatchedClientWithMetro returns a new KraftCloud client with the given metro.
// This client exists because of a bug in the KraftCloud SDK where the client
// default metro does not acknowledge the FQDN as it does with individual services.
func NewPatchedClientWithMetro(client ukc.KraftCloud, metro string) ukc.KraftCloud {
	return &patchedClientWithMetro{client: client, metro: metro}
}

type patchedClientWithMetro struct {
	client ukc.KraftCloud
	metro  string
}

func (c *patchedClientWithMetro) Autoscale() autoscale.AutoscaleService {
	return c.client.Autoscale().WithMetro(c.metro)
}

func (c *patchedClientWithMetro) Certificates() certificates.CertificatesService {
	return c.client.Certificates().WithMetro(c.metro)
}

func (c *patchedClientWithMetro) Images() images.ImagesService {
	return c.client.Images().WithMetro(c.metro)
}

func (c *patchedClientWithMetro) Instances() instances.InstancesService {
	return c.client.Instances().WithMetro(c.metro)
}

func (c *patchedClientWithMetro) Metros() metros.MetrosService {
	return c.client.Metros().WithMetro(c.metro)
}

func (c *patchedClientWithMetro) Services() services.ServicesService {
	return c.client.Services().WithMetro(c.metro)
}

func (c *patchedClientWithMetro) Users() users.UsersService {
	return c.client.Users().WithMetro(c.metro)
}

func (c *patchedClientWithMetro) Volumes() volumes.VolumesService {
	return c.client.Volumes().WithMetro(c.metro)
}
