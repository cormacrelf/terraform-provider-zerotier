package zerotier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

const baseUrl string = "https://my.zerotier.com/api"

type ZeroTierClient struct {
	ApiKey string
}

type Route struct {
	// cidr
	Target string `json:"target"`
	// nil if handled by 's 'LAN'
	Via *string `json:"via"`
}

type IpRange struct {
	First string `json:"ipRangeStart"`
	Last  string `json:"ipRangeEnd"`
}

type V4AssignModeConfig struct {
	ZT bool `json:"zt"`
}

type Config struct {
	Name              string             `json:"name"`
	Private           bool               `json:"private"`
	EnableBroadcast   bool               `json:"enableBroadcast"`
	MulticastLimit    int                `json:"multicastLimit"`
	Routes            []Route            `json:"routes"`
	IpAssignmentPools []IpRange          `json:"ipAssignmentPools"`
	V4AssignMode      V4AssignModeConfig `json:"v4AssignMode"`
}

type ConfigReadOnly struct {
	Config

	// if you include these three in a POST request, Central won't compile RulesSource for you
	// so, we only want them when reading from the API
	// this struct lets you do that
	Tags         []Tag        `json:"tags"`
	Rules        []IRule      `json:"rules"`
	Capabilities []Capability `json:"capabilities"`

	CreationTime int64 `json:"creationTime"`
	LastModified int64 `json:"lastModified"`
	Revision     int   `json:"revision"`
}

type Network struct {
	Id          string  `json:"id"`
	Description string  `json:"description,omitempty"`
	RulesSource string  `json:"rulesSource,omitempty"`
	Config      *Config `json:"config,omitempty"`
}

type NetworkReadOnly struct {
	Id                 string               `json:"id"`
	Description        string               `json:"description"`
	RulesSource        string               `json:"rulesSource"`
	Config             *ConfigReadOnly      `json:"config"`
	TagsByName         map[string]TagByName `json:"tagsByName"`
	CapabilitiesByName map[string]int       `json:"capabilitiesByName"`
}

type Capability struct {
	Id      int     `json:"id"`
	Default bool    `json:"default"`
	Rules   []IRule `json:"rules"`
}

type Tag struct {
	Id      int  `json:"id"`
	Default *int `json:"default"`
}

type IRule interface {
	// default unmarshaljson just makes a
	// map[string]interface{} from { type: "ACTION_DROP" } etc
}

type TagByName struct {
	Tag
	Enums map[string]int `json:"enums"`
	Flags map[string]int `json:"flags"`
}

type Member struct {
	Id                 string        `json:"id"`
	NetworkId          string        `json:"networkId"`
	NodeId             string        `json:"nodeId"`
	OfflineNotifyDelay int           `json:"offlineNotifyDelay"` // milliseconds
	Name               string        `json:"name"`
	Description        string        `json:"description"`
	Hidden             bool          `json:"hidden"`
	Config             *MemberConfig `json:"config"`
}
type MemberConfig struct {
	Authorized      bool     `json:"authorized"`
	Capabilities    []int    `json:"capabilities"`
	Tags            [][]int  `json:"tags"` // array of [tag id, value] tuples
	ActiveBridge    bool     `json:"activeBridge"`
	NoAutoAssignIps bool     `json:"noAutoAssignIps"`
	IpAssignments   []string `json:"ipAssignments"`
}
type MemberConfigReadOnly struct {
	CreationTime       int `json:"creationTime"`
	LastAuthorizedTime int `json:"lastAuthorizedTime"`
	VMajor             int `json:"vMajor"`
	VMinor             int `json:"vMinor"`
	VRev               int `json:"vRev"`
	VProto             int `json:"vProto"`
}

func CIDRToRange(cidr string) (net.IP, net.IP, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, err
	}
	first := ip.Mask(ipnet.Mask)
	last := make(net.IP, 4)
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		copy(last, ip)
	}
	// mirror what ZT console does
	// there must be a reason
	if first[3] == 0 {
		first[3] = 1
	}
	if last[3] == 255 {
		last[3] = 254
	}
	return first, last, nil

}

// modifies existing net.IP
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// not perfect, but allocation ranges should probably always be cidrs
func SmallestCIDR(from net.IP, to net.IP) string {
	maxLen := 32
	for l := maxLen; l >= 0; l-- {
		mask := net.CIDRMask(l, maxLen)
		na := from.Mask(mask)
		n := net.IPNet{IP: na, Mask: mask}

		if n.Contains(to) {
			return fmt.Sprintf("%v/%v", na, l)
		}
	}
	// return a string so it shows up in any CLI diffs
	return "unable to figure out CIDR from range"
}

func (s *ZeroTierClient) doRequest(reqName string, req *http.Request) ([]byte, error) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.ApiKey))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("%s received a %s response. Check your ZEROTIER_API_KEY.", reqName, resp.Status)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s received response: %s", reqName, body)
	}
	return body, nil
}

func (s *ZeroTierClient) headRequest(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.ApiKey))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (client *ZeroTierClient) CheckNetworkExists(id string) (bool, error) {
	url := fmt.Sprintf(baseUrl+"/network/%s", id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	resp, err := client.headRequest(req)
	if resp.StatusCode == 404 {
		return false, nil
	}
	if resp.StatusCode == 403 {
		return false, fmt.Errorf("CheckNetworkExists received a %s response. Check your ZEROTIER_API_KEY.", resp.Status)
	}
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("CheckNetworkExists received response: %s", resp.Status)
	}
	return true, err
}

func (client *ZeroTierClient) GetNetwork(id string) (*Network, error) {
	url := fmt.Sprintf(baseUrl+"/network/%s", id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	bytes, err := client.doRequest("GetNetwork", req)
	if err != nil {
		return nil, err
	}
	var data Network
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (client *ZeroTierClient) postNetwork(id string, network *Network) (*Network, error) {
	url := strings.TrimSuffix(fmt.Sprintf(baseUrl+"/network/%s", id), "/")
	// strip carriage returns?
	// network.RulesSource = strings.Replace(network.RulesSource, "\r", "", -1)
	j, err := json.Marshal(network)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
	if err != nil {
		return nil, err
	}
	var reqName string
	if id == "" {
		reqName = "CreateNetwork"
	} else {
		reqName = "UpdateNetwork"
	}
	bytes, err := client.doRequest(reqName, req)
	if err != nil {
		return nil, err
	}
	var data Network
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (client *ZeroTierClient) CreateNetwork(network *Network) (*Network, error) {
	return client.postNetwork("", network)
}

func (client *ZeroTierClient) UpdateNetwork(id string, network *Network) (*Network, error) {
	return client.postNetwork(id, network)
}

func (client *ZeroTierClient) DeleteNetwork(id string) error {
	url := fmt.Sprintf(baseUrl+"/network/%s", id)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	_, err = client.doRequest("DeleteNetwork", req)
	return err
}

/////////////
// members //
/////////////

func (client *ZeroTierClient) GetMember(nwid string, nodeId string) (*Member, error) {
	url := fmt.Sprintf(baseUrl+"/network/%s/member/%s", nwid, nodeId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	bytes, err := client.doRequest("GetMember", req)
	if err != nil {
		return nil, err
	}
	var data Member
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (client *ZeroTierClient) postMember(member *Member, reqName string) (*Member, error) {
	url := fmt.Sprintf(baseUrl+"/network/%s/member/%s", member.NetworkId, member.NodeId)
	j, err := json.Marshal(member)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
	if err != nil {
		return nil, err
	}
	bytes, err := client.doRequest(reqName, req)
	if err != nil {
		return nil, err
	}
	var data Member
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (client *ZeroTierClient) CreateMember(member *Member) (*Member, error) {
	return client.postMember(member, "CreateMember")
}

func (client *ZeroTierClient) UpdateMember(member *Member) (*Member, error) {
	return client.postMember(member, "UpdateMember")
}

// Careful: this one isn't documented in the Zt API,
// but this is what the Central web client does.
func (client *ZeroTierClient) DeleteMember(member *Member) error {
	url := fmt.Sprintf(baseUrl+"/network/%s/member/%s", member.NetworkId, member.NodeId)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	_, err = client.doRequest("DeleteMember", req)
	return err
}

func (client *ZeroTierClient) CheckMemberExists(nwid string, nodeId string) (bool, error) {
	url := fmt.Sprintf(baseUrl+"/network/%s/member/%s", nwid, nodeId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	resp, err := client.headRequest(req)
	if resp.StatusCode == 404 {
		return false, nil
	}
	if resp.StatusCode == 403 {
		return false, fmt.Errorf("CheckMemberExists received a %s response. Check your ZEROTIER_API_KEY.", resp.Status)
	}
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("CheckMemberExists received response: %s", resp.Status)
	}
	return true, err
}
