# jetstream_stream Resource

The `jetstream_stream` Resource creates a JetStream Stream, supports editing resources in place.

## Example Usage

```hcl
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
 * `duplicate_window` - (optional) The time window size for duplicate tracking, duration specified in seconds (number)
 * `replicas` - (optional) How many replicas of the data to keep in a clustered environment
 * `placement_cluster` - (optional) Place the stream in a specific cluster, influenced by placement_tags
 * `placement_tags` - (optional) Place the stream only on servers with these tags
