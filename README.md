# Terraform provider for ZeroTier

This lets you create, modify and destroy [ZeroTier](https://zerotier.com) networks through Terraform.
Nothing fancy yet, like adding members, but the networks are the bulk of
terraform-able activity.

## Building and Installing

The easiest way to get going is to:

1. Install [Go](http://www.golang.org/) on your machine, 1.9+ required
2. Clone this repo into your `$GOPATH`, and `cd` into it. This would usually be
   `~/go/src/github.com/cormacrelf/terraform-provider-zerotier`
3. `go get -v` (will take a while to download `hashicorp/terraform`, so `-v` is to tell you what it's downloading).
4. `go build -o terraform-provider-zerotier_v0.0.1`
5. Copy the resulting executable to your terraform plugins path. See [the
   docs](https://www.terraform.io/docs/plugins/basics.html#installing-a-plugin)
   for where that is.

## Usage

There's only one resource, `"zerotier_network"`. To achieve a similar
configuration to the Zerotier default, do this:

```hcl
variable "zt_cidr" { default = "10.72.0.0/24" }

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

If you don't specify either an assignment pool or a managed route, your network
won't be very useful, so try to do both.

### Multiple routes

You can have more than one assignment pool, and more than one route. Multiple
routes are useful for connecting two networks together, like so:

```hcl
variable "zt_cidr" { default = "10.72.0.0/24" }
variable "other_network" { default = "10.41.23.0/24" }
variable "gateway" { default = "10.72.0.61" }

resource "zerotier_network" "your_network" {
    name = "your_network_name"
    assignment_pool {
        cidr = "${var.zt_cidr}"
    }
    route {
        target = "${var.zt_cidr}"
    }
    route {
        target = "${var.other_network}"
        via = "${gateway}"
    }
}
```

### Rules

Best of all, you can specify rules just like in the web interface.

```conf
# rules.ztr
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
    rules_source = "${file(rules.ztr)}"
}
```
