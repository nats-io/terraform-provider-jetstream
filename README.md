This is a work in progress Terraform Provider to manage NATS JetStream.

## Installation

To install download the archive for your platform from the [Releases Page](https://github.com/nats-io/terraform-provider-jetstream/releases) and place the plugin in your plugin directory, for example in `~/.terraform.d/plugins/terraform-provider-jetstream_v0.0.1`.  After that `terraform init` will find it.
 
## Usage

To recreate the `ORDERS` example from the [JetStream README](https://github.com/nats-io/jetstream#configuration) you can use the code below:

```terraform
provider "jetstream" {
  servers = "connect.ngs.global:4222"
  credentials = "ngs_stream_admin.creds"
}

// ORDERS setup from the JetStream README
resource "jetstream_stream" "ORDERS" {
  name     = "ORDERS"
  subjects = ["ORDERS.*"]
  storage  = "file"
  max_age  = 60 * 60 * 24 * 365
}

resource "jetstream_consumer" "ORDERS_NEW" {
  stream_id      = jetstream_stream.ORDERS.id
  durable_name   = "NEW"
  deliver_all    = true
  filter_subject = "ORDERS.received"
  sample_freq    = 100
}

resource "jetstream_consumer" "ORDERS_DISPATCH" {
  stream_id      = jetstream_stream.ORDERS.id
  durable_name   = "DISPATCH"
  deliver_all    = true
  filter_subject = "ORDERS.processed"
  sample_freq    = 100
}

resource "jetstream_consumer" "ORDERS_MONITOR" {
  stream_id        = jetstream_stream.ORDERS.id
  durable_name     = "MONITOR"
  deliver_last     = true
  ack_policy       = "none"
  delivery_subject = "monitor.ORDERS"
}

// Stream Template that creates JS_1H_x streams based on activity in the JS.1H.* subjects
resource "jetstream_stream_template" "ONE_HOUR" {
  name        = "JS_1H"
  subjects    = ["JS.1H.*"]
  storage     = "file"
  max_age     = 60 * 60
  max_streams = 60
}

output "ORDERS_SUBJECTS" {
  value = jetstream_stream.ORDERS.subjects
}
```