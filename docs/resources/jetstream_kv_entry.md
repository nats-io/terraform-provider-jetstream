## jetstream_kv_entry Resource

The `jetstream_kv_entry` Resource manages entries in JetStream Based Key-Value buckets and supports editing entries in place.

### Example

Using a string value:

```hcl
resource "jetstream_kv_entry" "myservice_cfg" {
  bucket = "CFG"
  key = "config.myservice.timeout"
  value = "10"
}
```

Using json:

```hcl
resource "jetstream_kv_entry" "myservice_cfg" {
  bucket = "CFG"
  key = "config.myservice"
  value = jsonencode({timeout: 10, retries: 3})
}
```

### Attribute Reference

 * `bucket` - (required) The name of the KV bucket
 * `key` - (required) The entry key
 * `value` - (required) The entry value
