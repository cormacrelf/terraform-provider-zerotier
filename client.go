package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
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
	Description string  `json:"description"`
	RulesSource string  `json:"rulesSource"`
	Config      *Config `json:"config"`
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

func (n *Network) Compile() error {
	return nil
	// compiled, err := CompileRulesSource([]byte(n.RulesSource))
	// if err != nil {
	// 	return err
	// }
	// n.Config.Rules = compiled.Config.Rules
	// n.Config.Tags = compiled.Config.Tags
	// n.Config.Capabilities = compiled.Config.Capabilities
	// n.TagsByName = compiled.TagsByName
	// n.CapabilitiesByName = compiled.CapabilitiesByName
	return nil
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

func (s *ZeroTierClient) doRequest(req *http.Request) ([]byte, error) {
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
	if 200 != resp.StatusCode {
		return nil, fmt.Errorf("%s", body)
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
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, err
	}
	resp, err := client.headRequest(req)
	if err != nil {
		return false, err
	}

	return resp.StatusCode == 200, nil
}

func (client *ZeroTierClient) GetNetwork(id string) (*Network, error) {
	url := fmt.Sprintf(baseUrl+"/network/%s", id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	bytes, err := client.doRequest(req)
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
	url := fmt.Sprintf(baseUrl+"/network/%s", id)
	j, err := json.Marshal(network)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
	if err != nil {
		return nil, err
	}
	bytes, err := client.doRequest(req)
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
	_, err = client.doRequest(req)
	return err
}
