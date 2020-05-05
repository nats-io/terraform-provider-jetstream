package jetstream

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

func resourceConsumer() *schema.Resource {
	return &schema.Resource{
		Create: resourceConsumerCreate,
		Read:   resourceConsumerRead,
		Delete: resourceConsumerDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

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
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new"},
				ForceNew:     true,
			},
			"start_time": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The timestamp of the first message that will be delivered by this Consumer",
				ValidateFunc: validation.IsRFC3339Time,
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new"},
				ForceNew:     true,
			},
			"deliver_all": &schema.Schema{
				Type:         schema.TypeBool,
				Description:  "Starts at the first available message in the Stream",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new"},
				ForceNew:     true,
			},
			"deliver_last": &schema.Schema{
				Type:         schema.TypeBool,
				Description:  "Starts at the latest available message in the Stream",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new"},
				ForceNew:     true,
			},
			"deliver_new": &schema.Schema{
				Type:         schema.TypeBool,
				Description:  "Starts with the next available message in the Stream",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new"},
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

func consumerConfigFromResourceData(d *schema.ResourceData) (cfg api.ConsumerConfig, err error) {
	cfg = api.ConsumerConfig{
		Durable:         d.Get("durable_name").(string),
		AckWait:         time.Duration(d.Get("ack_wait").(int)) * time.Second,
		MaxDeliver:      d.Get("max_delivery").(int),
		FilterSubject:   d.Get("filter_subject").(string),
		SampleFrequency: fmt.Sprintf("%d%%", d.Get("sample_freq").(int)),
		DeliverSubject:  d.Get("delivery_subject").(string),
		DeliverPolicy:   api.DeliverAll,
	}

	seq := uint64(d.Get("stream_sequence").(int))
	st := d.Get("start_time").(string)
	switch {
	case d.Get("deliver_all").(bool):
		cfg.DeliverPolicy = api.DeliverAll
	case d.Get("deliver_last").(bool):
		cfg.DeliverPolicy = api.DeliverLast
	case d.Get("deliver_new").(bool):
		cfg.DeliverPolicy = api.DeliverNew
	case seq > 0:
		cfg.DeliverPolicy = api.DeliverByStartSequence
		cfg.OptStartSeq = seq
	case st != "":
		ts, err := time.Parse(time.RFC3339, st)
		if err != nil {
			return api.ConsumerConfig{}, err
		}
		cfg.DeliverPolicy = api.DeliverByStartTime
		cfg.OptStartTime = &ts
	}

	switch d.Get("replay_policy").(string) {
	case "instant":
		cfg.ReplayPolicy = api.ReplayInstant
	case "original":
		cfg.ReplayPolicy = api.ReplayOriginal
	}

	switch d.Get("ack_policy").(string) {
	case "explicit":
		cfg.AckPolicy = api.AckExplicit
	case "all":
		cfg.AckPolicy = api.AckAll
	case "none":
		cfg.AckPolicy = api.AckNone
	}

	return cfg, nil
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
	d.Set("ack_wait", cons.AckWait().Seconds())
	d.Set("max_delivery", cons.MaxDeliver())
	d.Set("filter_subject", cons.FilterSubject())
	d.Set("stream_sequence", 0)
	d.Set("start_time", "")

	switch cons.DeliverPolicy() {
	case api.DeliverAll:
		d.Set("delivery_all", true)
	case api.DeliverNew:
		d.Set("delivery_new", true)
	case api.DeliverLast:
		d.Set("delivery_last", true)
	case api.DeliverByStartSequence:
		d.Set("stream_sequence", cons.StartSequence())
	case api.DeliverByStartTime:
		d.Set("start_time", cons.StartTime())
	}

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
	case api.ReplayInstant:
		d.Set("replay_policy", "instant")
	case api.ReplayOriginal:
		d.Set("replay_policy", "original")
	}

	switch cons.AckPolicy() {
	case api.AckExplicit:
		d.Set("ack_policy", "explicit")
	case api.AckAll:
		d.Set("ack_policy", "all")
	case api.AckNone:
		d.Set("ack_policy", "none")
	}

	return nil
}

func resourceConsumerDelete(d *schema.ResourceData, m interface{}) error {
	streamName, durableName, err := parseConsumerID(d.Id())
	if err != nil {
		return err
	}

	known, err := jsm.IsKnownConsumer(streamName, durableName)
	if err != nil {
		return err
	}
	if !known {
		d.SetId("")
		return nil
	}

	cons, err := jsm.LoadConsumer(streamName, durableName)
	if err != nil {
		return err
	}

	return cons.Delete()
}
