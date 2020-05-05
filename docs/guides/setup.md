# Setup and Installation

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

Here's a command to create this using `nsc`, note replace the `DemoAccount` and `1M` strings with your account name and desired expiry time:

```
$ nsc add user -a DemoAccount
--expiry 1M \
--name ngs_jetstream_admin \
--allow-pub '$JS.STREAM.*.CONSUMER.*.CREATE' \
--allow-pub '$JS.STREAM.*.CONSUMER.*.DELETE' \
--allow-pub '$JS.STREAM.*.CONSUMER.*.INFO' \
--allow-pub '$JS.STREAM.*.CONSUMERS' \
--allow-pub '$JS.STREAM.*.CREATE' \
--allow-pub '$JS.STREAM.*.DELETE' \
--allow-pub '$JS.STREAM.*.INFO' \
--allow-pub '$JS.STREAM.*.UPDATE' \
--allow-pub '$JS.STREAM.LIST' \
--allow-pub '$JS.TEMPLATE.>' \
--allow-sub '_INBOX.>'
```

This will create the credential in `~/.nkeys/creds/synadia/MyAccount/ngs_jetstream_admin.creds`
