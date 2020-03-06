package jetstream

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats-server/v2/server"
)

func resourceConsumer() *schema.Resource {
	return &schema.Resource{
		Create: resourceConsumerCreate,
		Read:   resourceConsumerRead,
		Delete: resourceConsumerDelete,

		Schema: map[string]*schema.Schema{
			"stream_id": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The name of the Stream that this consumer consumes",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"durable_name": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The durable name of the Consumer",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"delivery_subject": &schema.Schema{
				Type:        schema.TypeString,
				Description: "The subject where a Push-based consumer will deliver messages",
				Optional:    true,
				ForceNew:    true,
			},
			"stream_sequence": &schema.Schema{
				Type:         schema.TypeInt,
				Description:  "The Stream Sequence that will be the first message delivered by this Consumer",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last"},
				ForceNew:     true,
			},
			"start_time": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The timestamp of the first message that will be delivered by this Consumer",
				ValidateFunc: validation.IsRFC3339Time,
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last"},
				ForceNew:     true,
			},
			"deliver_all": &schema.Schema{
				Type:         schema.TypeBool,
				Description:  "Starts at the first available message in the Stream",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last"},
				ForceNew:     true,
			},
			"deliver_last": &schema.Schema{
				Type:         schema.TypeBool,
				Description:  "Starts at the latest available message in the Stream",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last"},
				ForceNew:     true,
			},
			"ack_policy": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The delivery acknowledgement policy to apply to the Consumer",
				Optional:     true,
				Default:      "explicit",
				ValidateFunc: validation.StringInSlice([]string{"explicit", "all", "none"}, false),
				ForceNew:     true,
			},
			"ack_wait": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "Number of seconds to wait for acknowledgement",
				Default:     30,
				Optional:    true,
				ForceNew:    true,
			},
			"max_delivery": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "Maximum deliveries to attempt for each message",
				Default:     -1,
				Optional:    true,
				ForceNew:    true,
			},
			"filter_subject": &schema.Schema{
				Type:        schema.TypeString,
				Description: "Only receive a subset of messages from the Stream based on the subject they entered the Stream on",
				Default:     "",
				Optional:    true,
				ForceNew:    true,
			},
			"replay_policy": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The rate at which messages will be replayed from the stream",
				Optional:     true,
				Default:      "instant",
				ValidateFunc: validation.StringInSlice([]string{"instant", "original"}, false),
				ForceNew:     true,
			},
			"sample_freq": &schema.Schema{
				Type:         schema.TypeInt,
				Description:  "The percentage of acknowledgements that will be sampled for observability purposes",
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntBetween(0, 100),
				ForceNew:     true,
			},
		},
	}
}

func consumerConfigFromResourceData(d *schema.ResourceData) (cfg server.ConsumerConfig, err error) {
	cfg = server.ConsumerConfig{
		Durable:         d.Get("durable_name").(string),
		AckWait:         time.Duration(d.Get("ack_wait").(int)) * time.Second,
		MaxDeliver:      d.Get("max_delivery").(int),
		FilterSubject:   d.Get("filter_subject").(string),
		SampleFrequency: fmt.Sprintf("%d%%", d.Get("sample_freq").(int)),
		Delivery:        d.Get("delivery_subject").(string),
		StreamSeq:       uint64(d.Get("stream_sequence").(int)),
		DeliverAll:      d.Get("deliver_all").(bool),
		DeliverLast:     d.Get("deliver_last").(bool),
	}

	t := d.Get("start_time").(string)
	if t != "" {
		cfg.StartTime, err = time.Parse(time.RFC3339, t)
		if err != nil {
			return server.ConsumerConfig{}, err
		}
	}

	switch d.Get("replay_policy").(string) {
	case "instant":
		cfg.ReplayPolicy = server.ReplayInstant
	case "original":
		cfg.ReplayPolicy = server.ReplayOriginal
	}

	switch d.Get("ack_policy").(string) {
	case "explicit":
		cfg.AckPolicy = server.AckExplicit
	case "all":
		cfg.AckPolicy = server.AckAll
	case "none":
		cfg.AckPolicy = server.AckNone
	}

	return cfg, nil
}

func parseStreamID(id string) (string, error) {
	parts := strings.Split(id, "_")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid stream id %q", id)
	}

	if parts[0] != "JETSTREAM" || parts[1] != "STREAM" {
		return "", fmt.Errorf("invalid stream id %q", id)
	}

	return parts[2], nil
}

func parseConsumerID(id string) (stream string, consumer string, err error) {
	parts := strings.Split(id, "_")
	if len(parts) != 5 {
		return "", "", fmt.Errorf("invalid consumer id %q", id)
	}

	if parts[0] != "JETSTREAM" || parts[1] != "STREAM" || parts[3] != "CONSUMER" {
		return "", "", fmt.Errorf("invalid consumer id %q", id)
	}

	return parts[2], parts[4], nil
}

func resourceConsumerCreate(d *schema.ResourceData, m interface{}) error {
	cfg, err := consumerConfigFromResourceData(d)
	if err != nil {
		return err
	}

	stream, err := parseStreamID(d.Get("stream_id").(string))
	if err != nil {
		return err
	}

	_, err = jsm.NewConsumerFromDefault(stream, cfg)
	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("JETSTREAM_STREAM_%s_CONSUMER_%s", stream, cfg.Durable))

	return resourceConsumerRead(d, m)
}

func resourceConsumerRead(d *schema.ResourceData, m interface{}) error {
	stream, name, err := parseConsumerID(d.Id())
	if err != nil {
		return err
	}

	known, err := jsm.IsKnownStream(stream)
	if err != nil {
		return fmt.Errorf("could not determine if stream %q is known: %s", name, err)
	}
	if !known {
		d.SetId("")
		return nil
	}

	known, err = jsm.IsKnownConsumer(stream, name)
	if err != nil {
		return fmt.Errorf("could not determine if %q > %q is a known consumer: %s", stream, name, err)
	}
	if !known {
		d.SetId("")
		return nil
	}

	cons, err := jsm.LoadConsumer(stream, name)
	if err != nil {
		return err
	}

	d.Set("stream", cons.StreamName())
	d.Set("durable_name", cons.DurableName())
	d.Set("delivery_subject", cons.DeliverySubject())
	d.Set("stream_sequence", cons.StreamSequence())
	d.Set("delivery_all", cons.DeliverAll())
	d.Set("deliver_last", cons.DeliverLast())
	d.Set("ack_wait", cons.AckWait().Seconds())
	d.Set("max_delivery", cons.MaxDeliver())
	d.Set("filter_subject", cons.FilterSubject())

	s := strings.TrimSuffix(cons.SampleFrequency(), "%")
	freq, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("failed to parse consumer sampling configuration: %v", err)
	}
	d.Set("sample_freq", freq)
	d.Set("start_time", "")
	if !cons.StartTime().IsZero() {
		d.Set("start_time", cons.StartTime().Format(time.RFC3339))
	}

	switch cons.ReplayPolicy() {
	case server.ReplayInstant:
		d.Set("replay_policy", "instant")
	case server.ReplayOriginal:
		d.Set("replay_policy", "original")
	}

	switch cons.AckPolicy() {
	case server.AckExplicit:
		d.Set("ack_policy", "explicit")
	case server.AckAll:
		d.Set("ack_policy", "all")
	case server.AckNone:
		d.Set("ack_policy", "none")
	}

	return nil
}

func resourceConsumerDelete(d *schema.ResourceData, m interface{}) error {
	streamName, durableName, err := parseConsumerID(d.Id())
	if err != nil {
		return err
	}

	cons, err := jsm.LoadConsumer(streamName, durableName)
	if err != nil {
		d.SetId("")
		return nil
	}

	return cons.Delete()
}
