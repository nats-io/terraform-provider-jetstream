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

```hcl
resource "jetstream_stream" "ORDERS_ARCHIVE" {
  name     = "ORDERS_ARCHIVE"
  storage  = "file"
  max_age  = 5 * 60 * 60 * 24 * 365

  mirror {
    name = "ORDERS"
  }
}
```

## Sources and Mirrors

Above the `ORDERS_ARCHIVE` stream is a mirror of `ORDERS`, valid options for specifying a mirror and sources are:

 * `name` - The name of the stream to mirror
 * `filter_subject` - (optional) For sources this filters the source
 * `start_seq` - (optional) Starts the mirror or source at this sequence in the source
 * `start_time` - (optional) Starts the mirror or source at this time in the source, in RFC3339 format
 * `external` - (optional) Reference to an external stream with keys `api` and `deliver`

## Attribute Reference

 * `description` - (optional) Contains additional information about this stream (string)
 * `discard` - (optional) When a Stream reach it's limits either old messages are deleted or new ones are denied (`new` or `old`)
 * `discard_new_per_subject` - (optional) When discard policy is new and the stream is one with max messages per subject set, this will apply the new behavior to every subject. Essentially turning discard new from maximum number of subjects into maximum number of messages in a subject (bool)
 * `ack` - (optional) If the Stream should support confirming receiving messages via acknowledgements (bool)
 * `max_age` - (optional) The maximum oldest message that can be kept in the stream, duration specified in seconds (number)
 * `max_bytes` - (optional) The maximum size of all messages that can be kept in the stream (number)
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
 * `republish_source` - (optional) Republish matching messages to `republish_destination`
 * `republish_destination` - (optional) The destination to publish messages to
 * `republish_headers_only` - (optional) Republish only message headers, no bodies
