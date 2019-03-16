package zerotier

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceZeroTierMember() *schema.Resource {
	return &schema.Resource{
		Create: resourceMemberCreate,
		Read:   resourceMemberRead,
		Update: resourceMemberUpdate,
		Delete: resourceMemberDelete,
		Exists: resourceMemberExists,

		Schema: map[string]*schema.Schema{
			"network_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"node_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Managed by Terraform",
			},
			"hidden": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"offline_notify_delay": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
			"authorized": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"allow_ethernet_bridging": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"no_auto_assign_ips": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"ip_assignments": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"capabilities": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
			"tags": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
		},
	}
}

func resourceMemberCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)
	stored, err := memberFromResourceData(d)
	if err != nil {
		return err
	}
	created, err := client.CreateMember(stored)
	if err != nil {
		return err
	}
	d.SetId(created.Id)
	setTags(d, created)
	return nil
}

func resourceMemberUpdate(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)
	stored, err := memberFromResourceData(d)
	if err != nil {
		return err
	}
	updated, err := client.UpdateMember(stored)
	if err != nil {
		return fmt.Errorf("unable to update member using ZeroTier API: %s", err)
	}
	setTags(d, updated)
	return nil
}

func setTags(d *schema.ResourceData, member *Member) {
	rawTags := map[string]int{}
	for _, tuple := range member.Config.Tags {
		key := fmt.Sprintf("%d", tuple[0])
		val := tuple[1]
		rawTags[key] = val
	}
}

func resourceMemberDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)
	member, err := memberFromResourceData(d)
	if err != nil {
		return err
	}
	err = client.DeleteMember(member)
	return err
}

func memberFromResourceData(d *schema.ResourceData) (*Member, error) {
	tags := d.Get("tags").(map[string]interface{})
	tagTuples := [][]int{}
	for key, val := range tags {
		i, err := strconv.Atoi(key)
		if err != nil {
			break
		}
		tagTuples = append(tagTuples, []int{i, val.(int)})
	}
	capsRaw := d.Get("capabilities").([]interface{})
	caps := make([]int, len(capsRaw))
	for i := range capsRaw {
		caps[i] = capsRaw[i].(int)
	}
	ipsRaw := d.Get("ip_assignments").([]interface{})
	ips := make([]string, len(ipsRaw))
	for i := range ipsRaw {
		ips[i] = ipsRaw[i].(string)
	}
	n := &Member{
		Id:                 d.Id(),
		NetworkId:          d.Get("network_id").(string),
		NodeId:             d.Get("node_id").(string),
		Hidden:             d.Get("hidden").(bool),
		OfflineNotifyDelay: d.Get("offline_notify_delay").(int),
		Name:               d.Get("name").(string),
		Description:        d.Get("description").(string),
		Config: &MemberConfig{
			Authorized:      d.Get("authorized").(bool),
			ActiveBridge:    d.Get("allow_ethernet_bridging").(bool),
			NoAutoAssignIps: d.Get("no_auto_assign_ips").(bool),
			Capabilities:    caps,
			Tags:            tagTuples,
			IpAssignments:   ips,
		},
	}
	return n, nil
}
func resourceMemberRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*ZeroTierClient)

	// Attempt to read from an upstream API
	nwid := d.Get("network_id").(string)
	nodeId := d.Get("node_id").(string)
	member, err := client.GetMember(nwid, nodeId)

	// If the resource does not exist, inform Terraform. We want to immediately
	// return here to prevent further processing.
	if err != nil {
		return fmt.Errorf("unable to read network from API: %s", err)
	}
	if member == nil {
		d.SetId("")
		return nil
	}

	d.SetId(member.Id)
	d.Set("name", member.Name)
	d.Set("description", member.Description)
	d.Set("hidden", member.Hidden)
	d.Set("offline_notify_delay", member.OfflineNotifyDelay)
	d.Set("authorized", member.Config.Authorized)
	d.Set("allow_ethernet_bridging", member.Config.ActiveBridge)
	d.Set("no_auto_assign_ips", member.Config.NoAutoAssignIps)
	d.Set("ip_assignments", member.Config.IpAssignments)
	d.Set("capabilities", member.Config.Capabilities)
	setTags(d, member)

	return nil
}

func resourceMemberExists(d *schema.ResourceData, m interface{}) (b bool, e error) {
	client := m.(*ZeroTierClient)
	nwid := d.Get("network_id").(string)
	nodeId := d.Get("node_id").(string)
	exists, err := client.CheckMemberExists(nwid, nodeId)
	if err != nil {
		return exists, err
	}

	if !exists {
		d.SetId("")
	}
	return exists, nil
}
