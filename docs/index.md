# NATS JetStream Provider

The JetStream Provider manages [NATS](https://nats.io) JetStream that enables persistence and Stream
Processing for the NATS eco system.

For background information please review the [Technical Preview](https://github.com/nats-io/jetstream#readme) notes.

## Example Usage

```terraform
provider "jetstream" {
  servers     = "connect.ngs.global:4222"
  credentials = "/home/you/ngs_stream_admin.creds"
  tls {
    ca_file_data = "<Root CA PEM data>"
  }
}

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

## Argument Reference

 * `servers` - The list of servers to connect to in a comma seperated list
 * `credentials` - (optional) Fully Qualified Path to a file holding NATS credentials.
 * `credential_data` - (optional) The NATS credentials as a string, intended to use with data providers.
 * `user` - (optional) Connects using a username, when no password is set this is assumed to be a Token.
 * `password` - (optional) Connects using a password
 * `nkey` - (optional) Connects using an nkey stored in a file
 * `tls.ca_file` - (optional) Fully Qualified Path to a file containing Root CA (PEM format). Use when the server has certs signed by an unknown authority.
 * `tls.ca_file_data` - (optional) The Root CA PEM as a string, intended to use with data providers. Use when the server has certs signed by an unknown authority.

## Resources

 * `jetstream_stream` - Manage a Stream that persistently stores messages
 * `jetstream_consumer` - Creates a Consumer that defines how Stream messages can be consumed by clients
 * `jetstream_kv_bucket` - Creates a Key-Value store
