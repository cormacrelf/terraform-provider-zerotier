package zerotier

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceZeroTierNetwork() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetworkCreate,
		Read:   resourceNetworkRead,
		Update: resourceNetworkUpdate,
		Delete: resourceNetworkDelete,
		Exists: resourceNetworkExists,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"description": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Managed by Terraform",
			},
			"cidr": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"ip_assignment_pools": &schema.Schema{
				Type:     schema.TypeList,
				Computed: true,
			},
			"auto_assign_v4": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"use_default_route": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"route": &schema.Schema{
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"target": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"via": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
				Set: resourceNetworkRouteHash,
			},
			"all_routes": &schema.Schema{
				Type:     schema.TypeList,
				Computed: true,
			},
			"rules_source": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				// pulled from ZT's default
				Default: "#\n# Allow only IPv4, IPv4 ARP, and IPv6 Ethernet frames.\n#\ndrop\n\tnot ethertype ipv4\n\tand not ethertype arp\n\tand not ethertype ipv6\n;\n\n#\n# Uncomment to drop non-ZeroTier issued and managed IP addresses.\n#\n# This prevents IP spoofing but also blocks manual IP management at the OS level and\n# bridging unless special rules to exempt certain hosts or traffic are added before\n# this rule.\n#\n#drop\n#\tnot chr ipauth\n#;\n\n# Accept anything else. This is required since default is 'drop'.\naccept;",
				Set:     stringHash,
			},
		},
	}
}

func resourceNetworkExists(d *schema.ResourceData, m interface{}) (b bool, e error) {
	client := m.(*ZeroTierClient)
	exists, err := client.CheckNetworkExists(d.Id())
	if err != nil {
		return exists, err
	}

	if !exists {
		d.SetId("")
	}
	return exists, nil
}

func resourceNetworkCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)
	net := NetworkDefault(d.Get("name").(string))
	client.CreateNetwork(net)
	d.SetId(net.Id)
	return nil
}

func resourceNetworkRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)

	// Attempt to read from an upstream API
	net, err := client.GetNetwork(d.Id())

	// If the resource does not exist, inform Terraform. We want to immediately
	// return here to prevent further processing.
	if err != nil {
		d.SetId("")
		return nil
	}

	d.Set("name", net.Config.Name)
	d.Set("description", net.Config.Description)
	d.Set("all_routes", net.Config.Routes)
	d.Set("auto_assign_v4", true)
	d.Set("ip_assignment_pools", net.Config.IpAssignmentPools)
	return nil
}

func resourceNetworkUpdate(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceNetworkDelete(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceNetworkRouteHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["target"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	if v, ok := m["via"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	return hashcode.String(buf.String())
}

func stringHash(v interface{}) int {
	s := v.(string)
	return hashcode.String(s)
}
