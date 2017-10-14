package zerotier

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

type IPRange struct {
	IpRangeStart string `json:"ipRangeStart"`
	IpRangeEnd   string `json:"ipRangeEnd"`
}

type Config struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`

	DefaultRoute *Route  `json:"-"`
	OtherRoutes  []Route `json:"-"`
	Routes       []Route `json:"routes,omitempty"`

	IpAssignmentPools []IPRange `json:"ipAssignmentPools,omitempty"`

	CreationTime int64 `json:"creationTime"`
	LastModified int64 `json:"lastModified"`
	Revision     int   `json:"revision"`
}

type Network struct {
	Id          string  `json:"id,omitempty"`
	Config      *Config `json:"config"`
	RulesSource *string `json:"rulesSource,omitempty"`
}

func NetworkDefault(name string) *Network {
	return &Network{
		Id: "",
		Config: &Config{
			Name:    name,
			Private: true,
			Routes:  []Route{},
		},
	}
}

func (n *Network) SetCIDR(cidr string) error {
	first, last, err := CIDRToRange(cidr)
	if err != nil {
		return err
	}
	n.Config.DefaultRoute = &Route{
		Target: cidr,
		Via:    nil,
	}
	n.Config.IpAssignmentPools = []IPRange{
		IPRange{
			IpRangeStart: first.String(),
			IpRangeEnd:   last.String(),
		},
	}
	return nil
}

func CIDRToRange(cidr string) (net.IP, net.IP, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return ip, ip, err
	}
	first := ip.Mask(ipnet.Mask)
	last := make(net.IP, 4)
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		copy(last, ip)
	}
	return first, last, nil

}

// modified existing net.IP
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
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

func (client *ZeroTierClient) postNetwork(id string, network *Network) error {
	url := fmt.Sprintf(baseUrl+"/network/%s", id)
	j, err := json.Marshal(network)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
	if err != nil {
		return err
	}
	_, err = client.doRequest(req)
	return err
}

func (client *ZeroTierClient) CreateNetwork(network *Network) error {
	return client.postNetwork("", network)
}

func (client *ZeroTierClient) UpdateNetwork(network *Network) error {
	return client.postNetwork(network.Id, network)
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
