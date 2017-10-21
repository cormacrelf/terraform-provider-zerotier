provider "zerotier" {
  // api_key = "..."
}

variable "target_cidr" { default = "10.72.0.0/18" }
variable "cidr" { default = "10.5.0.0/20" }
variable "gateway" { default = "10.5.1.1" }

resource "zerotier_network" "bouncy_castle" {
  name = "bouncy_castle"
  rules_source = "${file("script.ztr")}"
  assignment_pool {
    cidr = "${var.cidr}"
  }
  assignment_pool {
    first = "10.5.2.2"
    last = "10.5.2.254"
  }
  route {
    target = "${var.cidr}"
  }
  route {
    target = "${var.target_cidr}"
    via = "${var.gateway}"
  }

}
