# JetStream Provider

Terraform Provider that connects to any NATS JetStream server

## Example Usage

```terraform
provider "jetstream" {
  servers     = "connect.ngs.global:4222"
  credentials = "/home/you/ngs_stream_admin.creds"
}
```

## Argument Reference

 * `servers` - The list of servers to connect to in a comma seperated list
 * `credentials` - (optional) Fully Qualified Path to a file holding NATS credentials.
 * `credential_data` - (optional) The NATS credentials as a string, intended to use with data providers.
