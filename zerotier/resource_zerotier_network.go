package zerotier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"

	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
)

func route() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"target": &schema.Schema{
				Type:             schema.TypeString,
				Required:         true,
				DiffSuppressFunc: diffSuppress,
			},
			"via": &schema.Schema{
				Type:             schema.TypeString,
				Optional:         true,
				DiffSuppressFunc: diffSuppress,
			},
		},
	}
}

func resourceZeroTierNetwork() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetworkCreate,
		Read:   resourceNetworkRead,
		Update: resourceNetworkUpdate,
		Delete: resourceNetworkDelete,
		Exists: resourceNetworkExists,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

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
			"rules_source": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				// pulled from ZT's default
				Default: "#\n# Allow only IPv4, IPv4 ARP, and IPv6 Ethernet frames.\n#\ndrop\n\tnot ethertype ipv4\n\tand not ethertype arp\n\tand not ethertype ipv6\n;\n\n#\n# Uncomment to drop non-ZeroTier issued and managed IP addresses.\n#\n# This prevents IP spoofing but also blocks manual IP management at the OS level and\n# bridging unless special rules to exempt certain hosts or traffic are added before\n# this rule.\n#\n#drop\n#\tnot chr ipauth\n#;\n\n# Accept anything else. This is required since default is 'drop'.\naccept;",
				Set:     stringHash,
			},
			"private": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"auto_assign_v4": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			//Warning: Undecoumented on the API, but that is how the UI manages it
			"broadcast": &schema.Schema{
				Type:        schema.TypeBool,
				Description: "Enable network broadcast (ff:ff:ff:ff:ff:ff)",
				Optional:    true,
				Default:     true,
			},
			"multicast_limit": &schema.Schema{
				Type:         schema.TypeInt,
				Description:  "The maximum number of recipients that can receive an Ethernet multicast or broadcast. Setting to 0 disables multicast, but be aware that only IPv6 with NDP emulation (RFC4193 or 6PLANE addressing modes) or other unicast-only protocols will work without multicast.",
				Optional:     true,
				Default:      32,
				ValidateFunc: validation.IntAtLeast(0),
			},
			"route": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem:     route(),
			},
			"assignment_pool": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"cidr": &schema.Schema{
							Type:          schema.TypeString,
							Optional:      true,
							ConflictsWith: []string{"assignment_pool.first", "assignment_pool.last"},
						},
						"first": &schema.Schema{
							Type:          schema.TypeString,
							Optional:      true,
							ConflictsWith: []string{"assignment_pool.cidr"},
						},
						"last": &schema.Schema{
							Type:          schema.TypeString,
							Optional:      true,
							ConflictsWith: []string{"assignment_pool.cidr"},
						},
					},
				},
				Set: resourceIpAssignmentHash,
			},
		},
	}
}

func diffSuppress(k, old, new string, d *schema.ResourceData) bool {
	return old == new
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

func fromResourceData(d *schema.ResourceData) (*Network, error) {
	routesRaw := d.Get("route").([]interface{})
	var routes []Route
	for _, raw := range routesRaw {
		r := raw.(map[string]interface{})
		via := r["via"].(string)
		routes = append(routes, Route{
			Target: r["target"].(string),
			Via:    &via,
		})
	}
	var pools []IpRange
	for _, raw := range d.Get("assignment_pool").(*schema.Set).List() {
		r := raw.(map[string]interface{})
		cidr := r["cidr"].(string)
		first, last, err := CIDRToRange(cidr)
		if err != nil {
			first = net.ParseIP(r["first"].(string))
			last = net.ParseIP(r["last"].(string))
		}
		pools = append(pools, IpRange{
			First: first.String(),
			Last:  last.String(),
		})
	}
	n := &Network{
		Id:          d.Id(),
		RulesSource: d.Get("rules_source").(string),
		Description: d.Get("description").(string),
		Config: &Config{
			Name:              d.Get("name").(string),
			Private:           d.Get("private").(bool),
			EnableBroadcast:   d.Get("broadcast").(bool),
			MulticastLimit:    d.Get("multicast_limit").(int),
			V4AssignMode:      V4AssignModeConfig{ZT: true},
			Routes:            routes,
			IpAssignmentPools: pools,
		},
	}
	return n, nil
}

func resourceNetworkCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)
	n, err := fromResourceData(d)
	if err != nil {
		return err
	}
	created, err := client.CreateNetwork(n)
	if err != nil {
		return err
	}
	d.SetId(created.Id)
	setAssignmentPools(d, created)
	return nil
}

func resourceNetworkRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)

	// Attempt to read from an upstream API
	net, err := client.GetNetwork(d.Id())

	// If the resource does not exist, inform Terraform. We want to immediately
	// return here to prevent further processing.
	if err != nil {
		return fmt.Errorf("unable to read network from API: %s", err)
	}
	if net == nil {
		d.SetId("")
		return nil
	}

	d.Set("name", net.Config.Name)
	d.Set("description", net.Description)
	d.Set("private", net.Config.Private)
	d.Set("broadcast", net.Config.EnableBroadcast)
	d.Set("multicast_limit", net.Config.MulticastLimit)
	d.Set("auto_assign_v4", net.Config.V4AssignMode.ZT)
	d.Set("rules_source", net.RulesSource)

	setRoutes(d, net)
	setAssignmentPools(d, net)

	return nil
}

func setAssignmentPools(d *schema.ResourceData, n *Network) {
	rawPools := &schema.Set{F: resourceIpAssignmentHash}
	for _, p := range n.Config.IpAssignmentPools {
		raw := make(map[string]interface{})
		// raw["cidr"] = SmallestCIDR(net.ParseIP(p.First), net.ParseIP(p.Last))
		raw["first"] = p.First
		raw["last"] = p.Last
		rawPools.Add(raw)
	}
	d.Set("assignment_pool", rawPools)
}

func setRoutes(d *schema.ResourceData, n *Network) {
	rawRoutes := make([]interface{}, len(n.Config.Routes))
	for i, r := range n.Config.Routes {
		raw := make(map[string]interface{})
		raw["target"] = r.Target
		if r.Via != nil {
			raw["via"] = *r.Via
		}
		rawRoutes[i] = raw
	}
	d.Set("route", rawRoutes)
}

func resourceNetworkUpdate(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)
	n, err := fromResourceData(d)
	if err != nil {
		return err
	}
	updated, err := client.UpdateNetwork(d.Id(), n)
	if err != nil {
		stringify, _ := json.Marshal(n)
		return fmt.Errorf("unable to update network using ZeroTier API: %s\n\n%s", err, stringify)
	}
	setAssignmentPools(d, updated)
	return nil
}

func resourceNetworkDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)
	err := client.DeleteNetwork(d.Id())
	return err
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

func resourceIpAssignmentState(v interface{}) string {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["cidr"]; ok && len(v.(string)) > 0 {
		if first, last, err := CIDRToRange(v.(string)); err == nil {
			buf.WriteString(fmt.Sprintf("%s-", first.String()))
			buf.WriteString(fmt.Sprintf("%s", last.String()))
		}
	} else {
		if v, ok := m["first"]; ok {
			buf.WriteString(fmt.Sprintf("%s-", v.(string)))
		}

		if v, ok := m["last"]; ok {
			buf.WriteString(fmt.Sprintf("%s", v.(string)))
		}
	}

	return buf.String()
}

func resourceIpAssignmentHash(v interface{}) int {
	return hashcode.String(resourceIpAssignmentState(v))
}

func stringHash(v interface{}) int {
	s := v.(string)
	return hashcode.String(s)
}
