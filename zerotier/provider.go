package zerotier

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("ZEROTIER_API_KEY", nil),
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"zerotier_network": resourceZeroTierNetwork(),
			"zerotier_member":  resourceZeroTierMember(),
		},
		ConfigureFunc: configureProvider,
	}
}

func configureProvider(d *schema.ResourceData) (interface{}, error) {
	return &ZeroTierClient{ApiKey: d.Get("api_key").(string)}, nil
}
