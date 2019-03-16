package main

import (
	"terraform-provider-zerotier/zerotier"

	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/terraform"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() terraform.ResourceProvider {
			return zerotier.Provider()
		},
	})
}
