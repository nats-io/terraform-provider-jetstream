# Import existing JetStream resources

When migrating to the JetStream Terraform provider, it may be the case hat some resources have already been
created manually within the NATS JetStream cluster. It's possible to import these resources into the Terraform state
and start managing them using Terraform. 

## Example
Let's create some JetStream resources that will already exist in a NATS server: a stream called `ORDERS`,
a consumer for the `ORDERS` stream called `NEW`, a KV bucket called `TEST` and a KV entry called `FOO`.

Then, we need to define a matching HCL configuration for them. Alternatively, as of version 1.5, Terraform can 
generate the configuration for us. Refer to the
[Terraform documentation](https://developer.hashicorp.com/terraform/language/import) for more information.

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

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
  ttl = 60
  history = 10
  max_value_size = 1024
  max_bucket_size = 10240
}

resource "jetstream_kv_entry" "test_entry" {
  bucket = jetstream_kv_bucket.test.name
  key = "FOO"
  value = "bar"
}
```

Then, we need to run `terraform init`. 

If we do `terraform show`, we'll see there is currently no state

```zsh
➜  ✗ terraform show
No state.
```

And `terraform plan` would think all 3 resources need to be created: 

```zsh
➜  git:(main) ✗ terraform plan

Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # jetstream_consumer.ORDERS_NEW will be created
  + resource "jetstream_consumer" "ORDERS_NEW" {
      + ack_policy         = "explicit"
      + ack_wait           = 30
      + deliver_all        = true
      + durable_name       = "NEW"
      + filter_subject     = "ORDERS.received"
      + flow_control       = false
      + headers_only       = false
      + heartbeat          = 0
      + id                 = (known after apply)
      + inactive_threshold = 0
      + max_ack_pending    = 20000
      + max_batch          = 0
      + max_bytes          = 0
      + max_delivery       = -1
      + max_expires        = 0
      + max_waiting        = 512
      + memory             = false
      + ratelimit          = 0
      + replay_policy      = "instant"
      + replicas           = 0
      + sample_freq        = 100
      + stream_id          = (known after apply)
    }

  # jetstream_kv_bucket.test will be created
  + resource "jetstream_kv_bucket" "test" {
      + history         = 10
      + id              = (known after apply)
      + max_bucket_size = 10240
      + max_value_size  = 1024
      + name            = "TEST"
      + replicas        = 1
      + ttl             = 60
    }

  # jetstream_kv_entry.test_entry will be created
  + resource "jetstream_kv_entry" "test_entry" {
      + bucket   = "TEST"
      + id       = (known after apply)
      + key      = "FOO"
      + revision = (known after apply)
      + value    = "bar"
    }

  # jetstream_stream.ORDERS will be created
  + resource "jetstream_stream" "ORDERS" {
      + ack                     = true
      + allow_direct            = true
      + allow_rollup_hdrs       = false
      + deny_delete             = false
      + deny_purge              = false
      + discard                 = "old"
      + discard_new_per_subject = false
      + duplicate_window        = 120
      + id                      = (known after apply)
      + max_age                 = 31536000
      + max_bytes               = -1
      + max_consumers           = -1
      + max_msg_size            = -1
      + max_msgs                = -1
      + max_msgs_per_subject    = -1
      + name                    = "ORDERS"
      + replicas                = 1
      + retention               = "limits"
      + storage                 = "file"
      + subjects                = [
          + "ORDERS.*",
        ]
    }

Plan: 4 to add, 0 to change, 0 to destroy.

──────────────────────────────────────────────────────────────────────────────────────────────────────
```

### Importing the resources into state

```zsh
git:(main) ✗ terraform import jetstream_stream.ORDERS JETSTREAM_STREAM_ORDERS
jetstream_stream.ORDERS: Importing from ID "JETSTREAM_STREAM_ORDERS"...
jetstream_stream.ORDERS: Import prepared!
  Prepared jetstream_stream for import
jetstream_stream.ORDERS: Refreshing state... [id=JETSTREAM_STREAM_ORDERS]

Import successful!

The resources that were imported are shown above. These resources are now in
your Terraform state and will henceforth be managed by Terraform.
```

```zsh
➜  git:(main) ✗ terraform import jetstream_consumer.ORDERS_NEW JETSTREAM_STREAM_ORDERS_CONSUMER_NEW
jetstream_consumer.ORDERS_NEW: Importing from ID "JETSTREAM_STREAM_ORDERS_CONSUMER_NEW"...
jetstream_consumer.ORDERS_NEW: Import prepared!
  Prepared jetstream_consumer for import
jetstream_consumer.ORDERS_NEW: Refreshing state... [id=JETSTREAM_STREAM_ORDERS_CONSUMER_NEW]

Import successful!

The resources that were imported are shown above. These resources are now in
your Terraform state and will henceforth be managed by Terraform.
```

```zsh
➜  git:(main) ✗ terraform import jetstream_kv_bucket.test JETSTREAM_KV_TEST
jetstream_kv_bucket.test: Importing from ID "JETSTREAM_KV_TEST"...
jetstream_kv_bucket.test: Import prepared!
  Prepared jetstream_kv_bucket for import
jetstream_kv_bucket.test: Refreshing state... [id=JETSTREAM_KV_TEST]

Import successful!

The resources that were imported are shown above. These resources are now in
your Terraform state and will henceforth be managed by Terraform.
```

```zsh
➜  git:(main) ✗ terraform import jetstream_kv_entry.test_entry JETSTREAM_KV_TEST_ENTRY_FOO

Import successful!

The resources that were imported are shown above. These resources are now in
your Terraform state and will henceforth be managed by Terraform.
```

Now all the resources have been imported and can be managed by Terraform. 

## Terraform JetStream resource IDs 

When running `terraform import` normally the ID of the resource has to be specified. These IDs are provider specific. 
In the case of the JetStream provider, the IDs follow the following case sensitive format: 
* for streams: `JETSTREAM_STREAM_<stream-name>`
* for consumers: `JETSTREAM_STREAM_<stream-name>_CONSUMER_<consumer-name>`
* for kv buckets: `JETSTREAM_KV_<bucket-name>`
* for kv entries: `JETSTREAM_KV_<bucket-name>_ENTRY_<entry-key>`
