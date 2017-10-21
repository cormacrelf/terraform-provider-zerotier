# Terraform provider for ZeroTier

This lets you create, modify and destroy [ZeroTier](https://zerotier.com) networks through Terraform.
Nothing fancy yet, like adding members, but the networks are the bulk of
terraform-able activity.

## Building and Installing

1. Install [Go](http://www.golang.org/) on your machine, 1.9+ required

2. Do the following

```sh
# it will take a while to download `hashicorp/terraform`,
# so `-v` is to tell you what it's downloading

go get -v github.com/cormacrelf/terraform-provider-zerotier
cd ~/go/src/github.com/cormacrelf/terraform-provider-zerotier
go build -o terraform-provider-zerotier_v0.0.1

# IMPORTANT: on Windows, append `.exe` to the output name.
```

Then, copy the resulting executable to your terraform plugins path. [The
docs](https://www.terraform.io/docs/configuration/providers.html#third-party-plugins)
don't fully describe where this is.

* On macOS 64-bit, it's `~/.terraform.d/plugins/darwin_amd64`
* On Windows 64-bit, it's `$APPDATA\terraform.d\plugins\windows_amd64`
* On Linux, I'm not sure.

You might have to run `terraform init` with a provider block to find out where
it's actually looking for plugins.

## Usage

### API Key

Use `export ZEROTIER_API_KEY="..."`, or define it in a provider block:

```hcl
provider "zerotier" {
  api_key = "..."
}
```

### Network resource

There's only one resource, `"zerotier_network"`. To achieve a similar
configuration to the Zerotier default, do this:

```hcl
variable "zt_cidr" { default = "10.0.96.0/24" }

resource "zerotier_network" "your_network" {
    name = "your_network_name"
    # auto-assign v4 addresses to devices
    assignment_pool {
        cidr = "${var.zt_cidr}"
    }
    # route requests to the cidr block on each device through zerotier
    route {
        target = "${var.zt_cidr}"
    }
}
```

If you don't specify either an assignment pool or a managed route, while it's
perfectly valid, your network won't be very useful, so try to do both.

### Multiple routes

You can have more than one assignment pool, and more than one route. Multiple
routes are useful for connecting two networks together, like so:

```hcl
variable "zt_cidr" { default = "10.0.96.0/24" }
variable "other_network" { default = "10.41.23.0/24" }
locals {
  # the first address is reserved for the gateway
  gateway_ip = "${cidrhost(var.zt_cidr, 1)}" # eg 10.0.96.1
}

resource "zerotier_network" "your_network" {
    name = "your_network_name"
    assignment_pool {
        first  = "${cidrhost(var.zt_cidr,  2)}" # eg 10.0.96.2
        last   = "${cidrhost(var.zt_cidr, -2)}" # eg 10.0.96.254
    }
    route {
        target = "${var.zt_cidr}"
    }
    route {
        target = "${var.other_network}"
        via    = "${local.gateway_ip}"
    }
}
```

Then go ahead and make an API call on your gateway's provisioner to set the IP
address manually. See below (auto-joining). 

### Rules

Best of all, you can specify rules just like in the web interface. Note, pending
[this zerotier issue](https://github.com/zerotier/ZeroTierOne/issues/608) you'll
have to taint the network to compile new rules.

```sh
# ztr.conf

# drop non-v4/v6/arp traffic
drop not ethertype ipv4 and not ethertype arp and not ethertype ipv6;

# disallow tcp connections except by specific grant in a capability
break chr tcp_syn and not chr tcp_ack;

# allow ssh from some devices
cap ssh
    id 1000
    accept ipprotocol tcp and dport 22;
;
accept;
```

```hcl
resource "zerotier_network" "your_network" {
    name = "your_network_name"
    assignment_pool {
        cidr = "${var.zt_cidr}"
    }
    route {
        target = "${var.zt_cidr}"
    }
    rules_source = "${file(ztr.conf)}"
}
```

### Auto-joining and auto-approving instances

Using `zerotier-cli join XXX` doesn't require an API key, but that member won't
be approved by default. The solution is to pass in the key to a provisioner and
use the ZeroTier API to do it from the instance itself.

If you're going to have instances auto-approve themselves with the same API key,
provide the environment variable `export TF_VAR_zerotier_api_key="..."` so you
can access the key outside the provider definition, and do something like this
(simplified and probably needs work):

```hcl
variable "zerotier_api_key" {}
provider "zerotier" {
  api_key = "${var.zerotier_api_key}"
}
resource "zerotier_network" "example" {
  # ...
}
resource "aws_instance" "web" {
  provisioner "file" {
    source = "join.sh"
    destination = "/tmp/join.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "sudo sh /tmp/join.sh ${var.zerotier_api_key} ${var.zerotier_network.example.id}"
    ]
  }
}
```

Note the `sudo`. `join.sh` is like the following:

```sh
ZT_API_KEY="$1"
ZT_NET="$2"
# maybe install zerotier here

zerotier-cli join $1
MEMBER_ID=$(zerotier-cli info | awk '{print $3}')
echo '{"config":{"authorized":true}}' | curl -X POST -H 'Authorization: Bearer $ZT_API_KEY' -d @- \
    "https://my.zerotier.com/api/network/$ZT_NET/member/$MEMBER_ID"
```

You could even set a static IP there, by POSTing the following instead. This is
useful if you want the instance to act as a gateway like in the multiple routes
example above.

```json
{
    "name": "a_single_tear",
    "config": {
        "authorized": true,
        "ipAssignments": ["10.0.96.1"]
    }
}
```

Note that if you're provisioning inside a VPC's _public_ subnet, you will likely
need to put the provisioner blocks on a `resource "aws_eip"` instead (with
`host` specified), so that the instance has Internet connectivity when it tries
to do all this. On a private subnet, it already has Internet connectivity at
boot, but on public, it can't do anything until the eip is ready, and running
a provisioner on the instance will delay creation of the eip.

### Replace your VPN Gateway

If you do the above auto-join/approve, static IP assignment, and add a route to
your VPC's CIDR via that static IP, you're almost ready to replace a VPN
gateway. This can be much cheaper and more flexible, and you can probably get by
on a `t2.nano`. The only missing pieces are packet forwarding from ZT to VPC,
and getting packets back out.

For packet forwarding, set `source_dest_check = false` on the instance, and use
your distro's version of `echo 1 > /proc/sys/net/ipv4/ip_forward` (make it
permanent by editing/appending to `/etc/sysctl.conf`).

To get them back out, it is preferable to set up your VPC route tables to route
the ZeroTier CIDR through your instance. It may be sufficient to use the
following `iptables` rules on the instance itself using MASQUERADE (again loaded
via the file provisioner), as long as you're not using any strange protocols not
supported by MASQUERADE. This can be simpler if you have lots of subnets.

```sh
# zt0 is the zerotier virtual interface, eth0 is connected to the VPC
iptables -t nat -F
# make it look like packets are coming from the gateway, not a zerotier IP
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
iptables -t nat -A POSTROUTING -j ACCEPT
```

The main difference of this approach from route table entries is that the source
IP is different for the security group rule evaluator. _With plain packet
forwarding and a route table return, you need an ingress rule for your zerotier
__CIDR__ on a service in your VPC._ Say `80, 10.0.96.0/24`. This is not more
powerful but a little more tedious. You can't control where ZT assigns members
within the assignment pools, and you would probably regulate that with your ZT
rules/capabilities anyway. With MASQUERADE, you allow ingress from the gateway's
security group.

It's probably a good idea to have some FORWARD rules either way you do the
routing, otherwise the gateway might be too useful as a nefarious pivot point
into, inside or outbound from your VPC.

```sh
iptables -F
# packets flow freely from zt to vpc
iptables -A FORWARD -i zt0 -o eth0 -s "$ZT_CIDR" -d "$VPC_CIDR" -j ACCEPT
# can't establish new outbound connection going the other way
iptables -A FORWARD -i eth0 -o zt0 -s "$VPC_CIDR" -d "$ZT_CIDR" -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A FORWARD -j REJECT
```


