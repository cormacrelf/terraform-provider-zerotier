package zerotier

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func isValidControllerURL(i interface{}, k string) ([]string, []error) {
	v, ok := i.(string)
	if !ok {
		return nil, []error{fmt.Errorf("expected type of %q to be string", k)}
	}
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil, []error{fmt.Errorf("%q must not be empty", k)}
	}

	if strings.HasSuffix(trimmed, "/") {
		return nil, []error{fmt.Errorf("%q should not have trailing slash", k)}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, []error{fmt.Errorf("%q must be a valid url", k)}
	}

	if parsed.Scheme == "" {
		return nil, []error{fmt.Errorf("%q should have an scheme, such as http:// or https://", k)}
	}

	return nil, nil
}

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("ZEROTIER_API_KEY", nil),
			},
			"controller_url": &schema.Schema{
				Type:         schema.TypeString,
				Required:     true,
				DefaultFunc:  schema.EnvDefaultFunc("ZEROTIER_CONTROLLER_URL", "https://my.zerotier.com/api"),
				ValidateFunc: isValidControllerURL,
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
	return &ZeroTierClient{
		ApiKey:     d.Get("api_key").(string),
		Controller: d.Get("controller_url").(string)}, nil
}
