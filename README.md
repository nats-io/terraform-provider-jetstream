This is a work in progress Terraform Provider to manage NATS JetStream.

## Installation

To install download the archive for your platform from the [Releases Page](https://github.com/nats-io/terraform-provider-jetstream/releases) and place the plugin in your local install.
 
You can install it for your user by creating `~/.terraform.d/plugins/terraform-provider-jetstream_v0.0.3`.  After that `terraform init` will find it. Or you can check it into your repository in `your-repo/terraform.d/plugins/linux_amd64/terraform-provider-jetstream_v0.0.3`.

## Credentials

The provider supports NATS 2.0 credentials either in a file or as a Terraform Variable or Data, for local use you would set the credentials as in the examples later, but if you use something like Terraform Cloud or wish to store the credentials in a credential store you can use Terraform Variables:

```terraform
variable "ngs_credential_data" {
  type = string
  description = "Text of the NATS credentials to use to manage JetStream"
}

provider "jetstream" {
  servers = "connect.ngs.global"
  credential_data = var.ngs_credential_data
}
```

This also lets you store the data in Vault, for example:

```terraform
data "vault_generic_secret" "jetstream_credential" {
  path = "secret/jetstream_credential"
}

provider "jetstream" {
  servers = "connect.ngs.global"
  credential_data = data.vault_generic_secret.jetstream_credential.data["ngs_jetstream_admin"]
}
```

A locked down user that can only admin JetStream but not retrieve any data stored in JetStream looks like this:

```
╭─────────────────────────────────────────────────────────────────────────────────╮
│                                      User                                       │
├──────────────────────┬──────────────────────────────────────────────────────────┤
│ Name                 │ ngs_jetstream_admin                                      │
│ User ID              │ UBI3V7PXXQHJ67C4G2W2D7ICX3NK4S2RJCBUPNLWGGV6W34HYFUSD57M │
│ Issuer ID            │ AARRII4JZYJB3WYECBTMU6WSST2SUZ7SN7PWNSDWQTYVPMXQHDA3WXZ7 │
│ Issued               │ 2020-03-11 10:37:29 UTC                                  │
│ Expires              │ 2020-04-11 10:37:29 UTC                                  │
├──────────────────────┼──────────────────────────────────────────────────────────┤
│ Pub Allow            │ $JS.STREAM.*.CONSUMER.*.CREATE                           │
│                      │ $JS.STREAM.*.CONSUMER.*.DELETE                           │
│                      │ $JS.STREAM.*.CONSUMER.*.INFO                             │
│                      │ $JS.STREAM.*.CONSUMERS                                   │
│                      │ $JS.STREAM.*.CREATE                                      │
│                      │ $JS.STREAM.*.DELETE                                      │
│                      │ $JS.STREAM.*.INFO                                        │
│                      │ $JS.STREAM.*.UPDATE                                      │
│                      │ $JS.STREAM.LIST                                          │
│                      │ $JS.TEMPLATE.>                                           │
│ Sub Allow            │ _INBOX.>                                                 │
├──────────────────────┼──────────────────────────────────────────────────────────┤
│ Response Permissions │ Not Set                                                  │
├──────────────────────┼──────────────────────────────────────────────────────────┤
│ Max Messages         │ Unlimited                                                │
│ Max Msg Payload      │ Unlimited                                                │
│ Network Src          │ Any                                                      │
│ Time                 │ Any                                                      │
╰──────────────────────┴──────────────────────────────────────────────────────────╯
```

## Provider

Terraform Provider that connects to any NATS JetStream server

### Example

```terraform
provider "jetstream" {
  servers     = "connect.ngs.global:4222"
  credentials = "/home/you/ngs_stream_admin.creds"
}
```

### Argument Reference

 * `servers` - Comma separated list of servers to connect to
 * `credentials` - (optional) path to the credentials file to use
 * `credential_data` - (optional) the credentials as a string

## jetstream_stream

Creates a JetStream Stream, supports editing resources in place.

### Example

```terraform
resource "jetstream_stream" "ORDERS" {
  name     = "ORDERS"
  subjects = ["ORDERS.*"]
  storage  = "file"
  max_age  = 60 * 60 * 24 * 365
}
```

### Attribute Reference

 * `ack` - (optional) If the Stream should support confirming receiving messages via acknowledgements (bool)
 * `max_age` - (optional) The maximum oldest message that can be kept in the stream, duration specified in seconds (number)
 * `max_bytes` - (optional) The maximum size of all messages that can be kept in the stream (number)
 * `max_consumers` - (optional) Number of consumers this stream allows (number)
 * `max_msg_size` - (optional) The maximum individual message size that the stream will accept (number)
 * `max_msgs` - (optional) The maximum amount of messages that can be kept in the stream (number)
 * `name` - The name of the stream (string)
 * `replicas` - (optional) How many replicas of the data to keep in a clustered environment (number)
 * `retention` - (optional) The retention policy to apply over and above max_msgs, max_bytes and max_age (string)
 * `storage` - (optional) The storage engine to use to back the stream (string)
 * `subjects` - The list of subjects that will be consumed by the Stream (["list", "string"])

## jetstream_stream_template

Creates a JetStream Stream Template. Does not support editing in place, when a template gets deleted all it's managed Streams also get deleted

### Example

```terraform
resource "jetstream_stream_template" "ONE_HOUR" {
  name        = "JS_1H"
  subjects    = ["JS.1H.*"]
  storage     = "file"
  max_age     = 60 * 60
  max_streams = 60
}
```

### Attribute Reference

 * `ack` - (optional) If the Stream should support confirming receiving messages via acknowledgements (bool)
 * `max_age` - (optional) The maximum oldest message that can be kept in the stream, duration specified in seconds (number)
 * `max_bytes` - (optional) The maximum size of all messages that can be kept in the stream (number)
 * `max_consumers` - (optional) Number of consumers this stream allows (number)
 * `max_msg_size` - (optional) The maximum individual message size that the stream will accept (number)
 * `max_msgs` - (optional) The maximum amount of messages that can be kept in the stream (number)
 * `max_streams` - Maximum number of Streams this Template can manage (number)
 * `name` - The name of the Stream Template (string)
 * `replicas` - (optional) How many replicas of the data to keep in a clustered environment (number)
 * `retention` - (optional) The retention policy to apply over and above max_msgs, max_bytes and max_age (string)
 * `storage` - (optional) The storage engine to use to back the stream (string)
 * `subjects` - The list of subjects that will be consumed by the Stream (["list", "string"])

## jetstream_consumer

Create or Delete Consumers on any Terraform managed Stream. Does not support editing consumers in place

### Example

```terraform
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
```

### Attribute Reference

 * `ack_policy` - (optional) The delivery acknowledgement policy to apply to the Consumer
 * `ack_wait` - (optional) Number of seconds to wait for acknowledgement
 * `deliver_all` - (optional) Starts at the first available message in the Stream
 * `deliver_last` - (optional) Starts at the latest available message in the Stream
 * `delivery_subject` - (optional) The subject where a Push-based consumer will deliver messages
 * `durable_name` - The durable name of the Consumer
 * `filter_subject` - (optional) Only receive a subset of messages from the Stream based on the subject they entered the Stream on
 * `max_delivery` - (optional) Maximum deliveries to attempt for each message
 * `replay_policy` - (optional) The rate at which messages will be replayed from the stream
 * `sample_freq` - (optional) The percentage of acknowledgements that will be sampled for observability purposes
 * `start_time` - (optional) The timestamp of the first message that will be delivered by this Consumer
 * `stream_id` - The name of the Stream that this consumer consumes
 * `stream_sequence` - (optional) The Stream Sequence that will be the first message delivered by this Consumer
