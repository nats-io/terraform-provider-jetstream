# jetstream_stream Resource

The `jetstream_consumer` Resource creates, updates or deletes JetStream Consumers on any Terraform managed Stream.

## Example Usage

```hcl
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
 * `ack_policy` - (optional) The delivery acknowledgement policy to apply to the Consumer
 * `ack_wait` - (optional) Number of seconds to wait for acknowledgement
 * `deliver_all` - (optional) Starts at the first available message in the Stream
 * `deliver_last` - (optional) Starts at the latest available message in the Stream
 * `delivery_subject` - (optional) The subject where a Push-based consumer will deliver messages
 * `delivery_group` - (optional) When set Push consumers will only deliver messages to subscriptions with this group set
 * `durable_name` - The durable name of the Consumer
 * `filter_subject` - (optional) Only receive a subset of messages from the Stream based on the subject they entered the Stream on
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
 * `max_expires` - (optional) Limits the Pull Expires duration to this maximum in seconds
 * `inactive_threshold` - (optional) Removes the consumer after a idle period, specified as a duration in seconds
 * `replicas` - (optional) How many replicas of the data to keep in a clustered environment
 * `memory` - (optional) Force the consumer state to be kept in memory rather than inherit the setting from the stream
 * `backoff` - (optional) List of durations in Go format that represents a retry time scale for NaK'd messages. A list of durations in seconds
