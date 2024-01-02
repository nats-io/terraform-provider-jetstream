This is a work in progress Terraform Provider to manage NATS JetStream.

**DEPRECATION NOTICE: This project will be deprecated after release 0.1.1 and will receive only bug fixes and security updates going forward**

## Installation

When using Terraform 0.13 or newer adding the `provider` and running `terraform init` will download the provider from the [Terraform Registry](https://registry.terraform.io/providers/nats-io/jetstream/latest/docs).

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
+----------------------+----------------------------------------------------------+
| Pub Allow            | $JS.API.CONSUMER.DELETE.*.*                              |
|                      | $JS.API.CONSUMER.DURABLE.CREATE.*.*                      |
|                      | $JS.API.CONSUMER.INFO.*.*                                |
|                      | $JS.API.CONSUMER.LIST.*                                  |
|                      | $JS.API.CONSUMER.NAMES.*                                 |
|                      | $JS.API.INFO                                             |
|                      | $JS.API.STREAM.CREATE.*                                  |
|                      | $JS.API.STREAM.DELETE.*                                  |
|                      | $JS.API.STREAM.INFO.*                                    |
|                      | $JS.API.STREAM.LIST                                      |
|                      | $JS.API.STREAM.NAMES                                     |
|                      | $JS.API.STREAM.TEMPLATE.>                                |
|                      | $JS.API.STREAM.UPDATE.*                                  |
| Sub Allow            | _INBOX.>                                                 |
| Response Permissions | Not Set                                                  |
+----------------------+----------------------------------------------------------+
| Max Msg Payload      | Unlimited                                                |
| Max Data             | Unlimited                                                |
| Max Subs             | Unlimited                                                |
| Network Src          | Any                                                      |
| Time                 | Any                                                      |
+----------------------+----------------------------------------------------------+
```

Here's a command to create this using `nsc`, note replace the `DemoAccount` and `1M` strings with your account name and desired expiry time:

```
$ nsc add user -a DemoAccount
--expiry 1M \
--name ngs_jetstream_admin \
--allow-pub '$JS.API.CONSUMER.DELETE.*.*' \
--allow-pub '$JS.API.CONSUMER.DURABLE.CREATE.*.*' \
--allow-pub '$JS.API.CONSUMER.INFO.*.*' \
--allow-pub '$JS.API.CONSUMER.LIST.*' \
--allow-pub '$JS.API.CONSUMER.NAMES.*' \
--allow-pub '$JS.API.INFO' \
--allow-pub '$JS.API.STREAM.CREATE.*' \
--allow-pub '$JS.API.STREAM.DELETE.*' \
--allow-pub '$JS.API.STREAM.INFO.*' \
--allow-pub '$JS.API.STREAM.LIST' \
--allow-pub '$JS.API.STREAM.NAMES' \
--allow-pub '$JS.API.STREAM.TEMPLATE.>' \
--allow-pub '$JS.API.STREAM.UPDATE.*' \
--allow-sub '_INBOX.>'
```

This will create the credential in `~/.nkeys/creds/synadia/MyAccount/ngs_jetstream_admin.creds`

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

 * `servers` - The list of servers to connect to in a comma seperated list.
 * `credentials` - (optional) Fully Qualified Path to a file holding NATS credentials.
 * `credential_data` - (optional) The NATS credentials as a string, intended to use with data providers.
 * `user` - (optional) Connects using a username, when no password is set this is assumed to be a Token.
 * `password` - (optional) Connects using a password.
 * `nkey` - (optional) Connects using an nkey stored in a file.
 * `tls.ca_file` - (optional) Fully Qualified Path to a file containing Root CA (PEM format). Use when the server has certs signed by an unknown authority.
 * `tls.ca_file_data` - (optional) The Root CA PEM as a string, intended to use with data providers. Use when the server has certs signed by an unknown authority.
 * `tls.cert_file` - (optional) The certificate to authenticate with.
 * `tls.cert_file_data` - (optional) The certificate to authenticate with, intended to use with data providers.
 * `tls.key_file` - (optional) The private key to authenticate with.
 * `tls.key_file_data` - (optional) The private key to authenticate with, intended to use with data providers.

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

 * `description` - (optional) Contains additional information about this stream (string)
 * `metadata` - (optional) A map of strings with arbitrary metadata for the stream
 * `ack` - (optional) If the Stream should support confirming receiving messages via acknowledgements (bool)
 * `max_age` - (optional) The maximum oldest message that can be kept in the stream, duration specified in seconds (number)
 * `max_bytes` - (optional) The maximum size of all messages that can be kept in the stream (number)
 * `compression` - (optional) Enable stream compression by setting the value to `s2`
 * `max_consumers` - (optional) Number of consumers this stream allows (number)
 * `max_msg_size` - (optional) The maximum individual message size that the stream will accept (number)
 * `max_msgs` - (optional) The maximum amount of messages that can be kept in the stream (number)
 * `max_msgs_per_subject` (optional) The maximum amount of messages that can be kept in the stream on a per-subject basis (number)
 * `name` - The name of the stream (string)
 * `replicas` - (optional) How many replicas of the data to keep in a clustered environment (number)
 * `retention` - (optional) The retention policy to apply over and above max_msgs, max_bytes and max_age (string)
 * `storage` - (optional) The storage engine to use to back the stream (string)
 * `subjects` - The list of subjects that will be consumed by the Stream (["list", "string"])
 * `duplicate_window` - (optional) The time window size for duplicate tracking, duration specified in seconds (number)
 * `placement_cluster` - (optional) Place the stream in a specific cluster, influenced by placement_tags
 * `placement_tags` - (optional) Place the stream only on servers with these tags
 * `source` - (optional) List of streams to source
 * `mirror` - (optional) Stream to mirror
 * `deny_delete` - (optional) Restricts the ability to delete messages from a stream via the API. Cannot be changed once set to true (bool)
 * `deny_purge` - (optional) Restricts the ability to purge messages from a stream via the API. Cannot be change once set to true (bool)
 * `allow_rollup_hdrs` - (optional) Allows the use of the Nats-Rollup header to replace all contents of a stream, or subject in a stream, with a single new message (bool)
 * `allow_direct` - (optional) Allow higher performance, direct access to get individual messages via the $JS.DS.GET API (bool)

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

 * `description` - (optional) Contains additional information about this consumer
 * `metadata` - (optional) A map of strings with arbitrary metadata for the consumer
 * `discard` - (optional) When a Stream reach it's limits either old messages are deleted or new ones are denied (`new` or `old`)
 * `discard_new_per_subject` - (optional) When discard policy is new and the stream is one with max messages per subject set, this will apply the new behavior to every subject. Essentially turning discard new from maximum number of subjects into maximum number of messages in a subject (bool)
 * `ack_policy` - (optional) The delivery acknowledgement policy to apply to the Consumer
 * `ack_wait` - (optional) Number of seconds to wait for acknowledgement
 * `deliver_all` - (optional) Starts at the first available message in the Stream
 * `deliver_last` - (optional) Starts at the latest available message in the Stream
 * `delivery_subject` - (optional) The subject where a Push-based consumer will deliver messages
 * `delivery_group` - (optional) When set Push consumers will only deliver messages to subscriptions with this group set
 * `durable_name` - The durable name of the Consumer
 * `filter_subject` - (optional) Only receive a subset of messages from the Stream based on the subject they entered the Stream on
 * `filter_subjects` - (optional) Only receive a subset tof messages from the Stream based on subjects they entered the Stream on. This is exclusive to `filter_subject`. Only works with v2.10 or better.
 * `max_delivery` - (optional) Maximum deliveries to attempt for each message
 * `replay_policy` - (optional) The rate at which messages will be replayed from the stream
 * `sample_freq` - (optional) The percentage of acknowledgements that will be sampled for observability purposes
 * `start_time` - (optional) The timestamp of the first message that will be delivered by this Consumer
 * `stream_id` - The name of the Stream that this consumer consumes
 * `stream_sequence` - (optional) The Stream Sequence that will be the first message delivered by this Consumer
 * `ratelimit` - (optional) The rate limit for delivering messages to push consumers, expressed in bits per second
 * `heartbeat` - (optional) Enable heartbeat messages for push consumers, duration specified in seconds
 * `flow_control` - (optional) Enable flow control for push consumers
 * `max_waiting` - (optional) The number of pulls that can be outstanding on a pull consumer, pulls received after this is reached are ignored
 * `headers_only` - (optional) When true no message bodies will be delivered only headers
 * `max_batch` - (optional) Limits Pull Batch sizes to this maximum
 * `max_bytes` - (optional)The maximum bytes value that maybe set when dong a pull on a Pull Consumer
 * `max_expires` - (optional) Limits the Pull Expires duration to this maximum in seconds
 * `inactive_threshold` - (optional) Removes the consumer after a idle period, specified as a duration in seconds
 * `replicas` - (optional) How many replicas of the data to keep in a clustered environment
 * `memory` - (optional) Force the consumer state to be kept in memory rather than inherit the setting from the stream
 * `backoff` - (optional) List of durations in Go format that represents a retry time scale for NaK'd messages. A list of durations in seconds
 * `republish_source` - (optional) Republish matching messages to `republish_destination`
 * `republish_destination` - (optional) The destination to publish messages to
 * `republish_headers_only` - (optional) Republish only message headers, no bodies

## jetstream_kv_bucket

Creates a JetStream based KV bucket

### Example

```terraform
resource "jetstream_kv_bucket" "test" {
  name = "TEST"
  ttl = 60
  history = 10
  max_value_size = 1024
  max_bucket_size = 10240
}
```

### Attribute Reference

 * `name` - (required) The unique name of the KV bucket, must match `\A[a-zA-Z0-9_-]+\z`
 * `description` - (optional) Contains additional information about this bucket
 * `history` - (optional) Number of historic values to keep
 * `ttl` - (optional) How many seconds to keep values for, keeps forever when not set
 * `placement_cluster` - (optional) Place the bucket in a specific cluster, influenced by placement_tags
 * `placement_tags` - (optional) Place the bucket only on servers with these tags
 * `max_value_size` - (optional) Maximum size of any value
 * `max_bucket_size` - (optional) The maximum size of all data in the bucket
 * `replicas` - (optional) How many replicas to keep on a JetStream cluster

## jetstream_kv_entry

Creates a JetStream based KV bucket entry

### Example

```terraform
resource "jetstream_kv_entry" "test_entry" {
  bucket = "TEST"
  key = "foo"
  value = "bar"
}
```

### Attribute Reference

 * `bucket` - (required) The name of the KV bucket
 * `key` - (required) The entry key
 * `value` - (required) The entry value

# Import existing JetStream resources 

See [docs/guides/import.md](docs/guides/import.md) 
