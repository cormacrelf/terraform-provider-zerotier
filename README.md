# Terraform provider for ZeroTier

[![Build Status](https://travis-ci.org/cormacrelf/terraform-provider-zerotier.svg?branch=master)](https://travis-ci.org/cormacrelf/terraform-provider-zerotier)

This lets you create, modify and destroy [ZeroTier](https://zerotier.com)
networks and members through Terraform.

## Building and Installing

Since this isn't maintained by Hashicorp, you have to install it manually. There
are two main ways:

### Download a release

Download and unzip the [latest
release](https://github.com/cormacrelf/terraform-provider-zerotier/releases/latest).

Then, move the binary to your terraform plugins directory. [The
docs](https://www.terraform.io/docs/configuration/providers.html#third-party-plugins)
don't fully describe where this is.

* On Mac, it's `~/.terraform.d/plugins/darwin_amd64`
* On Linux, it's `~/.terraform.d/plugins/linux_amd64`
* On Windows, it's `$APPDATA\terraform.d\plugins\windows_amd64`

### Build using the Makefile

Install [Go](https://www.golang.org/) v1.9+ on your machine, and
[dep](https://golang.github.io/dep/docs/installation.html); clone the source,
and let `make install` do the rest.

#### Mac

```sh
brew install go  # or upgrade
brew install dep # or upgrade
mkdir -p $GOPATH/src/github.com/cormacrelf; cd $GOPATH/src/github.com/cormacrelf
git clone https://github.com/cormacrelf/terraform-provider-zerotier 
cd terraform-provider-zerotier
make install
# it may take a while to download `hashicorp/terraform`. be patient.
```

#### Linux

Install go and dep from your favourite package manager or from source. Then:

```sh
mkdir -p $GOPATH/src/github.com/cormacrelf; cd $GOPATH/src/github.com/cormacrelf
git clone https://github.com/cormacrelf/terraform-provider-zerotier 
cd terraform-provider-zerotier
make install
# it may take a while to download `hashicorp/terraform`. be patient.
```

#### Windows

In PowerShell, running as Administrator:

```powershell
choco install golang
choco install dep
# if you don't have these already
choco install zip
choco install git # for git-bash
```

In a shell that has Make, like Git-Bash:

```sh
mkdir -p $GOPATH/src/github.com/cormacrelf; cd $GOPATH/src/github.com/cormacrelf
git clone https://github.com/cormacrelf/terraform-provider-zerotier 
cd terraform-provider-zerotier
make install
# it may take a while to download `hashicorp/terraform`. be patient.
```

## Usage

Before you can use a new provider, you must run `terraform init` in your
project, where the root `.tf` file is.

### API Key

Use `export ZEROTIER_API_KEY="..."`, or define it in a provider block:

```hcl
provider "zerotier" {
  api_key = "..."
}
```

### Networks

#### Network resource

To achieve a similar configuration to the ZeroTier default, do this:

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

#### Multiple routes

You can have more than one assignment pool, and more than one route. Multiple
routes are useful for connecting two networks together, like so:

```hcl
variable "zt_cidr" { default = "10.96.0.0/24" }
variable "other_network" { default = "10.41.0.0/24" }
locals {
  # the first address is reserved for the gateway
  gateway_ip = "${cidrhost(var.zt_cidr, 1)}" # eg 10.96.0.1
}

resource "zerotier_network" "your_network" {
    name = "your_network_name"
    assignment_pool {
        first  = "${cidrhost(var.zt_cidr,  2)}" # eg 10.96.0.2
        last   = "${cidrhost(var.zt_cidr, -2)}" # eg 10.96.0.254
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

#### Rules

Best of all, you can specify rules just like in the web interface. You could even use a Terraform `template_file` to insert variables.

```sh
# ztr.conf

# drop non-v4/v6/arp traffic
drop not ethertype ipv4 and not ethertype arp and not ethertype ipv6;
# disallow tcp connections except by specific grant in a capability break chr tcp_syn and not chr tcp_ack; 
# allow ssh from some devices
cap ssh
    id 1000
    accept ipprotocol tcp and dport 22;
;

# allow everything else
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
    rules_source = "${file("ztr.conf")}"
}
```

### Members and joining

Unfortunately, it is not possible for a machine to be added to a network without
the machine itself reaching out to ZeroTier.

However, you can pre-approve a machine if you already know its Node ID. This is
**not** the case for dynamically created machines like cloud instances. It is
more useful for your developer machine, which you might want to give a bunch of
capabilities and pre-approve so that when you do paste a network ID in, you
don't have to use the web UI to do the rest.

#### Member resource

Basic example to pre-approve:

```hcl
resource "zerotier_member" "dev_machine" {
  node_id = "..."
  network_id = "${zerotier_network.net.id}"
  name = "dev machine"
}
```

Full list of properties:

```hcl
resource "zerotier_member" "hector" {
  # required: the known id that a particular machine shows
  # (e.g. in the Mac menu bar app, or the Windows tray, Linux CLI output)
  node_id                 = "a1511e5bf5"
  # required: the network id
  network_id              = "${zerotier_network.net.id}"

  # the rest are optional

  name                    = "hector"
  description             = "..."
  authorized              = true
  # whether to show it in the list in the Web UI
  hidden                  = false


  # e.g.
  # cap administrator
  #   id 1000
  #   accept;
  # ;
  capabilities = [ 1000 ]

  # e.g.
  # tag department
  #   id 2000
  #   enum 100 marketing
  #   enum 200 accounting
  # ;
  tags = {
    "2000" = 100 # marketing
  }

  # default (false) means this member has a managed IP address automatically assigned.
  # without ip_assignments being configured, the member won't have any managed IPs.
  no_auto_assign_ips      = false
  # will happily override any auto-assigned v4 addresses (and v6 in some configurations)
  ip_assignments = [
    "10.0.96.15"
  ]

  # not known whether this does anything or not
  offline_notify_delay    = 0
  # see ZeroTier Manual section on L2/ethernet bridging
  allow_ethernet_bridging = true

}
```

#### Joining your development machine automatically

Things are simple when you already know your Node ID. A `local-exec` provisioner
can be used to execute `sudo zerotier-cli join [nwid]` when a network is
created, which will be auto-approved using a `zerotier_member` resource. You
will have to type your password (once) during `terraform apply`, or you will
have to apply as root already.

The provisioner should be defined on a `null_resource` that is triggered when
the network ID changes. That way you can re-join by marking the null resource as
deleted, without deleting the entire network.

If you had another machine nearby (like a CI box), you could also run `join` on
it using SSH or similar. Or just accept the one-off menial task.

```hcl
resource "zerotier_network" "net" { ... }
resource "zerotier_member" "dev_machine" {
  network_id = "${zerotier_network.net.id}"
  node_id = "... (see above)"
  name = "dev machine"
  capabilities = [ 1000, 2000 ]
}
resource "null_resource" "joiner" {
  triggers {
    network_id = "${zerotier_network.net.id}"
  }
  provisioner "local-exec" {
    command = "sudo zerotier-cli join ${zerotier_network.net.id}"
  }
}
```

#### Auto-joining and auto-approving dynamic instances

Using `zerotier-cli join XXX` doesn't require an API key, but that member won't
be approved by default. On the other hand, the `zerotier_member` resource cannot
force a machine to join, it can only (pre-)approve and (pre-)configure membership of
a machine whose Node ID is already known. This is not true of a dynamically
created instance on a cloud provider.

The solution is to pass in the key to a provisioner and use the ZeroTier REST
API directly to do it from the instance itself. This is the basic pattern, and
applies whether you're using Terraform provisioners, running Docker entrypoint
scripts with environment variables, or running Ansible scripts (etc).

Any way you do it, you will need to have your ZT API key accessible to Terraform.
Provide the environment variable `export TF_VAR_zerotier_api_key="..."` so you
can access the key outside the provider definition, and do something like this:

```hcl
variable "zerotier_api_key" {}
provider "zerotier" {
  api_key = "${var.zerotier_api_key}"
}
resource "zerotier_network" "example" {
  # ...
}
```

You might then insert `"${var.zerotier_api_key}"` into
a [`kubernetes_secret`][k8s_secret] resource, or an
[`aws_ssm_parameter`][ssm_param], or directly into a provisioner as a script
argument. To use a standard Terraform provisioner, do this:

[k8s_secret]: https://www.terraform.io/docs/providers/kubernetes/r/secret.html
[ssm_param]:  https://www.terraform.io/docs/providers/aws/r/ssm_parameter.html

```hcl
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

# use gpg2 instead on Ubuntu, basically follow this guide
# https://www.zerotier.com/download.shtml
curl -s 'https://pgp.mit.edu/pks/lookup?op=get&search=0x1657198823E52A61' | gpg --import && \
if z=$(curl -s 'https://install.zerotier.com/' | gpg); then echo "$z" | sudo bash; fi

zerotier-cli join "$ZT_NET"
sleep 5
NODE_ID=$(zerotier-cli info | awk '{print $3}')
echo '{"config":{"authorized":true}}' | curl -X POST -H 'Authorization: Bearer $ZT_API_KEY' -d @- \
    "https://my.zerotier.com/api/network/$ZT_NET/member/$NODE_ID"
```

You could even set a static IP there, by POSTing the following instead. This is
useful if you want the instance to act as a gateway with a known IP, like in the
multiple routes example above. Or any field from the [ZeroTier API
Reference][zt-api] listing for `POST /api/network/{networkId}/member/{nodeId}`.

[zt-api]: https://my.zerotier.com/help/api

```json
{
    "name": "a-single-tear",
    "config": {
        "authorized": true,
        "ipAssignments": ["10.96.0.1"],
        "capabilities": [ 1000, 2000 ],
        "tags": [ [2000, 100] ]
    }
}
```

### Replace your VPN Gateway in an Amazon VPC

If you:

* define a 'gateway' ec2 instance in a VPC
* make it auto-join/approve with a static IP address, as above
* add a managed ZT route to your VPC's CIDR via that static IP, using a `route` block

... then you're almost ready to replace a VPN gateway. This can be cheaper
and more flexible, and you can probably get by on a `t2.nano`. The only
missing pieces are packet forwarding from ZT to VPC, and getting packets back
out.

It is preferable to set up your VPC route tables to route the ZeroTier CIDR
through your instance. If you have zero NAT, this means you will never have
any trouble with strange protocols, and you squeeze more performance out of
the `t2.nano` you set up. To be fair, on a `t2.nano` you are limited much
more by its limited link speed than anything else, and protocols that don't
support NAT are rare in primarily TCP/HTTP/ environments. NAT can be simpler
to set up if you have a lot of dynamically created subnets.

The main configuration difference of this approach from route table entries is that the
source IP is different for the security group rule evaluator. _With plain
packet forwarding and a route table return, you need an ingress rule for your
zerotier __CIDR__ on a service in your VPC._ Say `ingress tcp/80, 10.96.0.0/24`. This is
not more powerful, but equally as easy with Terraform. You can't control
where ZT assigns members within the assignment pools, and you would probably
regulate that with your ZT rules/capabilities anyway. With MASQUERADE, you
instead allow ingress from the gateway's security group.

Assuming the following:

```
networks:
    zerotier = 10.96.0.0/24
    aws vpc  = 10.41.0.0/16
interfaces:
    you        = { zt0: 10.96.0.37                   }
    gateway    = { zt0: 10.96.0.1,  eth0: 10.41.1.15 }
    ec2 in vpc = {                  eth0: 10.41.2.67 }
```

#### Required for both methods

You'll need to enable Linux kernel IPV4 forwarding. Use your distro's version
of `echo 1 > /proc/sys/net/ipv4/ip_forward`, and make it permanent by
editing/appending to `/etc/sysctl.conf`. On Ubuntu, that's:

```sh
# requires sudo
# set up packet forwarding now
echo 1 > /proc/sys/net/ipv4/ip_forward
# make it permanent
echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
sysctl -p /etc/sysctl.conf
```

It's a very good idea to have some FORWARD rules either way you do the
routing, otherwise the gateway might be too useful as a nefarious pivot point
into, inside or outbound from your VPC.

```sh
# requires sudo
iptables -F
# packets flow freely from zt to vpc
iptables -A FORWARD -i zt0 -o eth0 -s "$ZT_CIDR" -d "$VPC_CIDR" -j ACCEPT
# only allow stateful return in the other direction
# i.e. can't establish new outbound connections going the other way
iptables -A FORWARD -i eth0 -o zt0 -s "$VPC_CIDR" -d "$ZT_CIDR" -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A FORWARD -j REJECT
```

Load both of these scripts with your provisioner, whatever that may be, and run them as root.

#### Route table entries method

You want packets to move like so:

```
in:
1. you.zt0(src=10.96.0.37, dest=10.41.2.67) => ZT => gateway.zt0
2.     -> gateway.eth0(src=10.96.0.37, dest=10.41.2.67) => VPC normal => ec2.eth0
out:
3. ec2.eth0(src=10.41.2.67, dest=10.96.0.37) => VPC (through route table entry) => gateway.eth0
4.     -> gateway.zt0(src=10.41.2.67, dest=10.96.0.37) => ZT => you.zt0
```

* #1 is satisfied because of the managed route, if your ZT flow rules allow it.
* #2 and #3 need an **AWS security group** on ec2 with inbound/outbound allowed to the ZeroTier CIDR
* #2 and #3 need EC2 IP **source/dest check disabled** on the gateway instance.
* #3 needs a a **route table entry** out of each subnet you want return traffic from, to the gateway instance.
* #4 is satisfied if your ZT flow rules allow it.

##### Source/dest check

For packet forwarding, set `source_dest_check = false` on the instance.

##### Add a route table entry to a subnet

```hcl
data "aws_route_table" "private" {
  subnet_id = "..."
}

resource "aws_route" "zt_route" {
  route_table_id = "${data.aws_route_table.private.id}"

  # route all packets destined for zt network, send them through the gateway
  destination_cidr_block = "${var.zt_cidr}"
  instance_id = "${aws_instance.zt_gateway.id}"
}
```

##### Security group additions

You'll need a gateway security group with:

* All ingress UDP traffic on port 9993 allowed
* All egress allowed
* SSH ingress for provisioning

Any other ec2 instances you want to access from your ZT network will need:

* Ingress from your **ZT CIDR** for whatever ports you want
* Egress either everywhere or to ZT CIDR

#### NAT method

The gateway behaves like a standard router, using `iptables` MASQUERADE rules. 'You' sees exactly the same src,dest information on the packets; it looks like you are communicating directly with 10.41.2.67, but the 'ec2.eth0' interface sees packets coming from the gateway.

```
in:
1. you.zt0(src=10.96.0.37, dest=10.41.2.67) => ZT => gateway.zt0
2.     -> gateway.eth0(src=10.41.1.15, dest=10.41.2.67) => VPC => ec2.eth0
out:
3. ec2.eth0(src=10.41.2.67, dest=10.41.1.15) => VPC => gateway.eth0
4.     -> gateway.zt0(src=10.41.2.67, dest=10.96.0.37) => ZT => you.zt0
```

* #1 is satisfied because of the managed route, if your ZT flow rules allow it.
* #2 and #4 (NAT) need an **`iptables` MASQUERADE rule** on the gateway
* #2 and #3 need an **AWS security group** on ec2 with inbound/outbound to allow the **gateway's security group**
* #4 is satisfied if your ZT flow rules allow it.

##### iptables rule

Append this to your FORWARD rules script above:

```sh
# zt0 is the zerotier virtual interface, eth0 is connected to the VPC
iptables -t nat -F
# make it look like packets are coming from the gateway, not a zerotier IP
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
iptables -t nat -A POSTROUTING -j ACCEPT
```

##### Security group

You'll need a gateway security group with:

* All ingress UDP traffic on port 9993 allowed
* All egress allowed
* SSH ingress for provisioning

Any other ec2 instances you want to access from your ZT network will need:

* Ingress from your **gateway's security group** for whatever ports you want
* Egress either everywhere or to gateway
