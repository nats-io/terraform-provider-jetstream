# jetstream_kv_bucket Resource

The `jetstream_kv_bucket` Resource manages JetStream Based Key-Value buckets, supports editing Buckets in place.

## Example Usage

```hcl
resource "jetstream_kv_bucket" "CFG" {
  name    = "CFG"
  history = 10
}
```

### Attribute Reference

* `name` - (required) The unique name of the KV bucket, must match `\A[a-zA-Z0-9_-]+\z`
* `history` - (optional) Number of historic values to keep
* `ttl` - (optional) How many seconds to keep values for, keeps forever when not set
* `placement` - (optional) JetStream cluster name to place the KV bucket in
* `max_value_size` - (optional) Maximum size of any value
* `max_bucket_size` - (optional) The maximum size of all data in the bucket
* `replicas` - (optional) How many replicas to keep on a JetStream cluster
