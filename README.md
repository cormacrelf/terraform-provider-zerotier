# Terraform provider for ZeroTier

This lets you create, modify and destroy [ZeroTier](https://zerotier.com) networks through Terraform.
Nothing fancy yet, like adding members, but the networks are the bulk of
terraform-able activity.

## Building and Installing

Since this isn't part of the terraform-providers organisation (yet), you have to
install manually.

Install [Go](https://www.golang.org/) on your machine, 1.9+ required. Then do
the following:

```sh
# it will take a while to download `hashicorp/terraform`. be patient.

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

There is only one resource, `"zerotier_network"`. To achieve a similar
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

### Rules

Best of all, you can specify rules just like in the web interface. You could even use a Terraform `template_file` to insert variables.

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
    rules_source = "${file(ztr.conf)}"
}
```

### Auto-joining and auto-approving instances

Using `zerotier-cli join XXX` doesn't require an API key, but that member won't
be approved by default. The solution is to pass in the key to a provisioner and
use the ZeroTier API to do it from the instance itself. This is the basic
pattern, and applies whether you're using Terraform provisioners, running Docker
entrypoint scripts with environment variables, running a container on
Kubernetes.

Any way you do it, you will need to have your ZT API key accessible to Terraform.
Provide the environment variable `export TF_VAR_zerotier_api_key="..."` so you
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
```

You might then insert `"${var.zerotier_api_key}"` into a
[`kubernetes_secret`][k8s_secret] resource, or an
[`aws_ssm_parameter`][ssm_param]. To use a standard Terraform provisioner, do
this:

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
# maybe install zerotier here

zerotier-cli join $1
sleep 5
MEMBER_ID=$(zerotier-cli info | awk '{print $3}')
echo '{"config":{"authorized":true}}' | curl -X POST -H 'Authorization: Bearer $ZT_API_KEY' -d @- \
    "https://my.zerotier.com/api/network/$ZT_NET/member/$MEMBER_ID"
```

You could even set a static IP there, by POSTing the following instead. This
is useful if you want the instance to act as a gateway with a known IP, like
in the multiple routes example above.

```json
{
    "name": "a-single-tear",
    "config": {
        "authorized": true,
        "ipAssignments": ["10.96.0.1"]
    }
}
```

Often, and especially for joining/approving your own machine automatically, you might want to add some capabilities or tags. Refer to the [ZeroTier API Reference][zt-api] for more details on `POST /api/network/{networkId}/member/{nodeId}`.

[zt-api]: https://my.zerotier.com/help/api

```json
{
    "name": "dev-machine",
    "config": {
        "authorized": true,
        "capabilities": [ 1000, 2000 ],
        "tags": [ 1000 ]
    }
}
```

#### Joining your local machine automatically

The same principle of supplying an API key and calling the `my.zerotier.com`
API applies even if you're running a `local-exec` provisioner to have your
developer machine auto-connect. You will have to run `terraform apply` as
root/admin. This is a flaw; you don't really want to be running an elevated
shell all the time. So, don't try fancy `data "external"` tricks to
automatically re-join your machine if not already, because that would require
root on every `terraform plan`.

Instead, the provisioner should be defined on the network resource or on a
`null_resource` that depends on it. That way, you only need to run as admin
the first time. The script to run is essentially the same as above for a
cloud instance.

```hcl
resource "zerotier_network" "net" { ... }
resource "null_resource" "joiner" {
  triggers {
    network_id = "${zerotier_network.net.id}"
  }
  provisioner "local-exec" {
    command = "sudo sh ./join.sh ${var.zt_api_key} ${zerotier_network.net.id} ${var.zt_computer_name}"
    # windows
    # command = "powershell -c .\\join.ps1 -apikey ${var.zt_api_key} -nwid ${zerotier_network.net.id} -name ${var.zt_computer_name}"
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