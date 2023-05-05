# Setup and Installation

This is a work in progress Terraform Provider to manage NATS JetStream.

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
