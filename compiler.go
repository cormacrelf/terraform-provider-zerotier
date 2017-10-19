package main

///////////////////////////////
// This code isn't used
// it used to call the zt rule compiler via Node.js,
// but that turns out not to be necessary
// I just want one commit that has it for posterity
///////////////////////////////

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
)

type CompilerOutput struct {
	Config             CompilerConfig       `json:"config"`
	TagsByName         map[string]TagByName `json:"tagsByName"`
	CapabilitiesByName map[string]int       `json:"capabilitiesByName"`
}

type CompilerConfig struct {
	Rules        []IRule      `json:"rules"`
	Tags         []Tag        `json:"tags"`
	Capabilities []Capability `json:"capabilities"`
}

func CompileRulesSource(src []byte) (*CompilerOutput, error) {
	// check if we have node.js available
	if _, err := exec.LookPath("node"); err != nil {
		return nil, fmt.Errorf("node binary not found. terraform-provider-zerotier requires Node.js to be installed for compiling rule source")
	}
	tmpfile, err := ioutil.TempFile("", "zerotier-compiler-input")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpfile.Name()) // clean up

	cli := "./node_modules/zerotier-rule-compiler/cli.js"
	nodeCmd := exec.Command("node", cli, tmpfile.Name())

	if _, err := tmpfile.Write(src); err != nil {
		return nil, err
	}

	errPipe, _ := nodeCmd.StderrPipe()

	outputBytes, err := nodeCmd.Output()
	errBytes, _ := ioutil.ReadAll(errPipe)
	if err != nil || len(errBytes) > 0 || len(outputBytes) == 0 {
		return nil, fmt.Errorf("Error compiling zerotier rules: %s", string(errBytes))
	}

	if err := tmpfile.Close(); err != nil {
		return nil, err
	}

	var output CompilerOutput
	if err := json.Unmarshal(outputBytes, &output); err != nil {
		return nil, err
	}
	if output.Config.Rules == nil {
		return nil, fmt.Errorf("unable to parse compiled rules from JSON, %s", output.Config)
	}

	return &output, nil
}

// This stuff is sorta on hold
// There is no real reason for it to exist unless you also wanted to write a compiler
// If you want to test it, just uncomment and the code will run

// func (cap *Capability) UnmarshalJSON(b []byte) error {
// 	var capRaw struct {
// 		Id      int                `json:"id"`
// 		Default bool               `json:"default"`
// 		Rules   []*json.RawMessage `json:"rules"`
// 	}
// 	if err := json.Unmarshal(b, &capRaw); err != nil {
// 		return err
// 	}
// 	cap.Id = capRaw.Id
// 	cap.Default = capRaw.Default
// 	if rules, err := transformRules(capRaw.Rules); err == nil {
// 		cap.Rules = rules
// 	} else {
// 		return fmt.Errorf("bad rules in cap, %s", err)
// 	}
// 	return nil
// }
//
// func (cc *CompilerConfig) UnmarshalJSON(b []byte) error {
// 	var ccRaw struct {
// 		Tags         []Tag              `json:"tags"`
// 		Capabilities []Capability       `json:"capabilities"`
// 		Rules        []*json.RawMessage `json:"rules"`
// 	}
// 	if err := json.Unmarshal(b, &ccRaw); err != nil {
// 		return err
// 	}
// 	cc.Tags = ccRaw.Tags
// 	cc.Capabilities = ccRaw.Capabilities
// 	if rules, err := transformRules(ccRaw.Rules); err == nil {
// 		cc.Rules = rules
// 	} else {
// 		return fmt.Errorf("bad rules in compiler config, %s", err)
// 	}
// 	return nil
// }

func transformRules(raws []*json.RawMessage) ([]IRule, error) {
	rules := make([]IRule, len(raws))
	for i, raw := range raws {
		if rule, err := parseRule(raw); err == nil {
			rules[i] = rule
		} else {
			return nil, fmt.Errorf("errored on rule %s", string(*raw))
		}
	}
	return rules, nil
}

func parseRule(raw *json.RawMessage) (IRule, error) {
	if raw == nil {
		return nil, fmt.Errorf("cannot parse rule from nil")
	}
	var genericRule Rule
	if err := json.Unmarshal([]byte(*raw), &genericRule); err != nil {
		return nil, err
	}
	switch genericRule.Type {
	case "ACTION_DROP":
		var rule ActionDrop
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "ACTION_ACCEPT":
		var rule ActionAccept
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "ACTION_BREAK":
		var rule ActionBreak
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "ACTION_TEE":
		var rule ActionTee
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "ACTION_REDIRECT":
		var rule ActionRedirect
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "ACTION_DEBUG_LOG":
		var rule ActionDebugLog
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_SOURCE_ZEROTIER_ADDRESS":
		var rule MatchSourceZT
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_DEST_ZEROTIER_ADDRESS":
		var rule MatchDestZT
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_ETHERTYPE":
		var rule MatchEthertype
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_MAC_SOURCE":
		var rule MatchMacSource
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_MAC_DEST":
		var rule MatchMacDest
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_IPV4_SOURCE":
		var rule MatchIPV4Source
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_IPV4_DEST":
		var rule MatchIPV4Dest
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_IPV6_SOURCE":
		var rule MatchIPV6Source
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_IPV6_DEST":
		var rule MatchIPV6Dest
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_IP_TOS":
		var rule MatchIpTos
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_IP_PROTOCOL":
		var rule MatchIpProtocol
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_IP_SOURCE_PORT_RANGE":
		var rule MatchIpSourcePortRange
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_IP_DEST_PORT_RANGE":
		var rule MatchIpDestPortRange
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_CHARACTERISTICS":
		var rule MatchCharacteristics
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_FRAME_SIZE_RANGE":
		var rule MatchFrameSizeRange
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_TAGS_SAMENESS":
		var rule MatchTagsSameness
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_TAGS_BITWISE_AND":
		var rule MatchTagsBitwiseAnd
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_TAGS_BITWISE_OR":
		var rule MatchTagsBitwiseOr
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_TAGS_BITWISE_XOR":
		var rule MatchTagsBitwiseXor
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_RANDOM":
		var rule MatchRandom
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	case "MATCH_ICMP":
		var rule MatchIcmp
		if err := json.Unmarshal([]byte(*raw), &rule); err == nil {
			return rule, nil
		}
	}
	return nil, fmt.Errorf("unable to parse rule")
}

type Rule struct {
	Type string `json:"type"`
	Not  bool   `json:"not"`
}

type LengthField struct {
	Length int `json:"length"`
}
type AddressField struct {
	Address string `json:"address"`
}
type ZtAddressField struct {
	ZT string `json:"zt"`
}
type IpAddressField struct {
	IP string `json:"ip"`
}
type EtherTypeField struct {
	EtherType string `json:"etherType"`
}
type MacField struct {
	Mac string `json:"mac"`
}
type IpTosField struct {
	IpTos int `json:"ipTos"`
}
type IpProtocolField struct {
	IpProtocol int `json:"ipProtocol"`
}
type StartEndFields struct {
	Start int `json:"start"`
	End   int `json:"end"`
}
type IdValueFields struct {
	Id    int `json:"id"`
	Value int `json:"value"`
}

type ActionDrop struct{ Rule }
type ActionAccept struct{ Rule }
type ActionBreak struct{ Rule }
type ActionTee struct {
	Rule
	LengthField
	AddressField
}
type ActionRedirect struct{ Rule AddressField }
type ActionDebugLog struct{ Rule }
type MatchSourceZT struct{ Rule ZtAddressField }
type MatchDestZT struct{ Rule ZtAddressField }
type MatchEthertype struct{ Rule EtherTypeField }
type MatchMacSource struct{ Rule MacField }
type MatchMacDest struct{ Rule MacField }
type MatchIPV4Source struct{ Rule IpAddressField }
type MatchIPV4Dest struct{ Rule IpAddressField }
type MatchIPV6Source struct{ Rule IpAddressField }
type MatchIPV6Dest struct{ Rule IpAddressField }
type MatchIpTos struct {
	Rule
	Mask  int `json:"mask"`
	Start int `json:"start"`
	End   int `json:"end"`
}
type MatchRandom struct {
	Rule
	Probability uint32 `json:"probability"`
}
type MatchIcmp struct {
	Rule
	IcmpType int `json:"icmpType"` // 4 or 6
	IcmpCode int `json:"icmpCode"`
}
type MatchIpProtocol struct{ Rule IpProtocolField }

type MatchIpSourcePortRange struct{ Rule StartEndFields }
type MatchIpDestPortRange struct{ Rule StartEndFields }
type MatchFrameSizeRange struct{ Rule StartEndFields }

type MatchCharacteristics struct {
	Rule
	Mask  string `json:"mask"` // eg "0000000000000002"
	Value int    `json:"value"`
}

type MatchTagsSameness struct{ Rule IdValueFields }
type MatchTagsBitwiseAnd struct{ Rule IdValueFields }
type MatchTagsBitwiseOr struct{ Rule IdValueFields }
type MatchTagsBitwiseXor struct{ Rule IdValueFields }

// yeah
