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
	"github.com/nats-io/nats.go"
)

func resourceConsumer() *schema.Resource {
	return &schema.Resource{
		Create: resourceConsumerCreate,
		Read:   resourceConsumerRead,
		Delete: resourceConsumerDelete,
		Update: resourceConsumerUpdate,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"stream_id": {
				Type:         schema.TypeString,
				Description:  "The name of the Stream that this consumer consumes",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"description": {
				Type:        schema.TypeString,
				Description: "Contains additional information about this consumer",
				Optional:    true,
				ForceNew:    false,
			},
			"durable_name": {
				Type:         schema.TypeString,
				Description:  "The durable name of the Consumer",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"delivery_subject": {
				Type:        schema.TypeString,
				Description: "The subject where a Push-based consumer will deliver messages",
				Optional:    true,
				ForceNew:    false,
			},
			"stream_sequence": {
				Type:         schema.TypeInt,
				Description:  "The Stream Sequence that will be the first message delivered by this Consumer",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new", "deliver_last_per_subject"},
				ForceNew:     true,
			},
			"start_time": {
				Type:         schema.TypeString,
				Description:  "The timestamp of the first message that will be delivered by this Consumer",
				ValidateFunc: validation.IsRFC3339Time,
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new", "deliver_last_per_subject"},
				ForceNew:     true,
			},
			"deliver_all": {
				Type:         schema.TypeBool,
				Description:  "Starts at the first available message in the Stream",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new", "deliver_last_per_subject"},
				ForceNew:     true,
			},
			"deliver_last_per_subject": {
				Type:         schema.TypeBool,
				Description:  "Starts with the last message for each subject matched by filter",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new", "deliver_last_per_subject"},
				ForceNew:     true,
			},
			"deliver_last": {
				Type:         schema.TypeBool,
				Description:  "Starts at the latest available message in the Stream",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new", "deliver_last_per_subject"},
				ForceNew:     true,
			},
			"deliver_new": {
				Type:         schema.TypeBool,
				Description:  "Starts with the next available message in the Stream",
				Optional:     true,
				ExactlyOneOf: []string{"stream_sequence", "start_time", "deliver_all", "deliver_last", "deliver_new", "deliver_last_per_subject"},
				ForceNew:     true,
			},
			"delivery_group": {
				Type:        schema.TypeString,
				Description: "When set Push consumers will only deliver messages to subscriptions with this group set",
				Optional:    true,
				ForceNew:    true,
			},
			"ack_policy": {
				Type:         schema.TypeString,
				Description:  "The delivery acknowledgement policy to apply to the Consumer",
				Optional:     true,
				Default:      "explicit",
				ValidateFunc: validation.StringInSlice([]string{"explicit", "all", "none"}, false),
				ForceNew:     true,
			},
			"ack_wait": {
				Type:        schema.TypeInt,
				Description: "Number of seconds to wait for acknowledgement",
				Default:     30,
				Optional:    true,
				ForceNew:    false,
			},
			"max_delivery": {
				Type:        schema.TypeInt,
				Description: "Maximum deliveries to attempt for each message",
				Default:     -1,
				Optional:    true,
				ForceNew:    false,
			},
			"filter_subject": {
				Type:        schema.TypeString,
				Description: "Only receive a subset of messages from the Stream based on the subject they entered the Stream on",
				Default:     "",
				Optional:    true,
				ForceNew:    false,
			},
			"replay_policy": {
				Type:         schema.TypeString,
				Description:  "The rate at which messages will be replayed from the stream",
				Optional:     true,
				Default:      "instant",
				ValidateFunc: validation.StringInSlice([]string{"instant", "original"}, false),
				ForceNew:     true,
			},
			"sample_freq": {
				Type:         schema.TypeInt,
				Description:  "The percentage of acknowledgements that will be sampled for observability purposes",
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntBetween(0, 100),
				ForceNew:     false,
			},
			"ratelimit": {
				Type:         schema.TypeInt,
				Description:  "The rate limit for delivering messages to push consumers, expressed in bits per second",
				Optional:     true,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
				ForceNew:     true,
			},
			"max_ack_pending": {
				Type:         schema.TypeInt,
				Description:  "Maximum pending Acks before consumers are paused",
				Optional:     true,
				Default:      20000,
				ValidateFunc: validation.IntAtLeast(0),
				ForceNew:     false,
			},
			"heartbeat": {
				Type:        schema.TypeInt,
				Description: "Enable heartbeat messages for push consumers, duration specified in seconds",
				Optional:    true,
				Default:     "",
				ForceNew:    true,
			},
			"flow_control": {
				Type:        schema.TypeBool,
				Description: "Enable flow control for push consumers",
				Optional:    true,
				Default:     false,
				ForceNew:    true,
			},
			"max_waiting": {
				Type:         schema.TypeInt,
				Description:  "The number of pulls that can be outstanding on a pull consumer, pulls received after this is reached are ignored",
				Optional:     true,
				Default:      512,
				ForceNew:     false,
				ValidateFunc: validation.IntAtLeast(0),
			},
			"headers_only": {
				Type:        schema.TypeBool,
				Description: "When true no message bodies will be delivered only headers",
				Optional:    true,
				Default:     false,
				ForceNew:    false,
			},
			"max_batch": {
				Type:        schema.TypeInt,
				Description: "Limits Pull Batch sizes to this maximum",
				Optional:    true,
				Default:     0,
				ForceNew:    false,
			},
			"max_expires": {
				Type:        schema.TypeInt,
				Description: "Limits the Pull Expires duration to this maximum in seconds",
				Optional:    true,
				Default:     "",
				ForceNew:    false,
			},
			"max_bytes": {
				Type:        schema.TypeInt,
				Description: "The maximum bytes value that maybe set when dong a pull on a Pull Consumer",
				Optional:    true,
				Default:     "",
				ForceNew:    false,
			},
			"inactive_threshold": {
				Type:        schema.TypeInt,
				Description: "Removes the consumer after a idle period, specified as a duration in seconds",
				Default:     "0",
				Optional:    true,
				ForceNew:    false,
			},
			"replicas": {
				Type:        schema.TypeInt,
				Description: "How many replicas of the data to keep in a clustered environment",
				Default:     0,
				Optional:    true,
				ForceNew:    true,
			},
			"memory": {
				Type:        schema.TypeBool,
				Description: "Force the consumer state to be kept in memory rather than inherit the setting from the stream",
				Default:     false,
				Optional:    true,
				ForceNew:    true,
			},
			"backoff": {
				Type:        schema.TypeList,
				Description: "List of durations in Go format that represents a retry time scale for NaK'd messages. A list of durations in seconds.",
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
		},
	}
}

func consumerConfigFromResourceData(d *schema.ResourceData) (cfg api.ConsumerConfig, err error) {
	cfg = api.ConsumerConfig{
		Durable:            d.Get("durable_name").(string),
		AckWait:            time.Duration(d.Get("ack_wait").(int)) * time.Second,
		MaxDeliver:         d.Get("max_delivery").(int),
		FilterSubject:      d.Get("filter_subject").(string),
		SampleFrequency:    fmt.Sprintf("%d%%", d.Get("sample_freq").(int)),
		DeliverSubject:     d.Get("delivery_subject").(string),
		DeliverPolicy:      api.DeliverAll,
		RateLimit:          uint64(d.Get("ratelimit").(int)),
		MaxAckPending:      d.Get("max_ack_pending").(int),
		FlowControl:        d.Get("flow_control").(bool),
		Heartbeat:          time.Duration(d.Get("heartbeat").(int)) * time.Second,
		HeadersOnly:        d.Get("headers_only").(bool),
		MaxRequestBatch:    d.Get("max_batch").(int),
		MaxRequestExpires:  time.Duration(d.Get("max_expires").(int)) * time.Second,
		MaxRequestMaxBytes: d.Get("max_bytes").(int),
		Replicas:           d.Get("replicas").(int),
		MemoryStorage:      d.Get("memory").(bool),
		InactiveThreshold:  time.Duration(d.Get("inactive_threshold").(int)) * time.Second,
	}

	if description, ok := d.GetOk("description"); ok {
		cfg.Description = description.(string)
	}

	if cfg.DeliverSubject != "" {
		cfg.MaxWaiting = d.Get("max_waiting").(int)
		cfg.DeliverGroup = d.Get("delivery_group").(string)
	}

	for _, d := range d.Get("backoff").([]any) {
		cfg.BackOff = append(cfg.BackOff, time.Duration(d.(int))*time.Second)
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
	case d.Get("deliver_last_per_subject").(bool):
		cfg.DeliverPolicy = api.DeliverLastPerSubject
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

	ok, errs := cfg.Validate(new(SchemaValidator))
	if !ok {
		return api.ConsumerConfig{}, fmt.Errorf(strings.Join(errs, ", "))
	}

	return cfg, nil
}

func resourceConsumerUpdate(d *schema.ResourceData, m any) error {
	stream := d.Get("stream_id").(string)
	if stream == "" {
		return fmt.Errorf("cannot determine stream name for update")
	}

	durable := d.Get("durable_name").(string)
	if durable == "" {
		return fmt.Errorf("cannot determine durable name for update")
	}

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	known, err := mgr.IsKnownConsumer(stream, durable)
	if err != nil {
		return err
	}
	if !known {
		d.SetId("")
		return nil
	}

	cons, err := mgr.LoadConsumer(stream, durable)
	if err != nil {
		return err
	}

	cfg, err := consumerConfigFromResourceData(d)
	if err != nil {
		return err
	}

	s := strings.TrimSuffix(cons.SampleFrequency(), "%")
	freq, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("failed to parse consumer sampling configuration: %v", err)
	}

	opts := []jsm.ConsumerOption{
		jsm.ConsumerDescription(cfg.Description),
		jsm.AckWait(cfg.AckWait),
		jsm.MaxDeliveryAttempts(cfg.MaxDeliver),
		jsm.SamplePercent(freq),
		jsm.MaxAckPending(uint(cfg.MaxAckPending)),
		jsm.MaxWaiting(uint(cfg.MaxWaiting)),
		jsm.MaxRequestExpires(cfg.MaxRequestExpires),
		jsm.MaxRequestBatch(uint(cfg.MaxRequestBatch)),
	}

	if cfg.HeadersOnly {
		opts = append(opts, jsm.DeliverHeadersOnly())
	}

	err = cons.UpdateConfiguration(opts...)
	if err != nil {
		return err
	}

	return resourceConsumerRead(d, m)
}

func resourceConsumerCreate(d *schema.ResourceData, m any) error {
	cfg, err := consumerConfigFromResourceData(d)
	if err != nil {
		return err
	}

	stream, err := parseStreamID(d.Get("stream_id").(string))
	if err != nil {
		return err
	}

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	_, err = mgr.NewConsumerFromDefault(stream, cfg)
	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("JETSTREAM_STREAM_%s_CONSUMER_%s", stream, cfg.Durable))

	return resourceConsumerRead(d, m)
}

func resourceConsumerRead(d *schema.ResourceData, m any) error {
	stream, name, err := parseConsumerID(d.Id())
	if err != nil {
		return err
	}

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	known, err := mgr.IsKnownStream(stream)
	if err != nil {
		return fmt.Errorf("could not determine if stream %q is known: %s", name, err)
	}
	if !known {
		d.SetId("")
		return nil
	}

	known, err = mgr.IsKnownConsumer(stream, name)
	if err != nil {
		return fmt.Errorf("could not determine if %q > %q is a known consumer: %s", stream, name, err)
	}
	if !known {
		d.SetId("")
		return nil
	}

	cons, err := mgr.LoadConsumer(stream, name)
	if err != nil {
		return err
	}

	d.Set("stream", cons.StreamName())
	d.Set("description", cons.Description())
	d.Set("durable_name", cons.DurableName())
	d.Set("delivery_subject", cons.DeliverySubject())
	d.Set("ack_wait", cons.AckWait().Seconds())
	d.Set("max_delivery", cons.MaxDeliver())
	d.Set("filter_subject", cons.FilterSubject())
	d.Set("stream_sequence", 0)
	d.Set("start_time", "")
	d.Set("ratelimit", cons.RateLimit())
	d.Set("max_ack_pending", cons.MaxAckPending())
	d.Set("heartbeat", cons.Heartbeat().Seconds())
	d.Set("flow_control", cons.FlowControl())
	d.Set("max_waiting", cons.MaxWaiting())
	d.Set("delivery_group", cons.DeliverGroup())
	d.Set("headers_only", cons.IsHeadersOnly())
	d.Set("max_batch", cons.MaxRequestBatch())
	d.Set("max_expires", cons.MaxRequestExpires().Seconds())
	d.Set("max_bytes", cons.MaxRequestMaxBytes())
	d.Set("replicas", cons.Replicas())
	d.Set("memory", cons.MemoryStorage())

	if cons.InactiveThreshold() != 0 {
		d.Set("inactive_threshold", cons.InactiveThreshold().String())
	}

	switch cons.DeliverPolicy() {
	case api.DeliverAll:
		d.Set("delivery_all", true)
	case api.DeliverNew:
		d.Set("delivery_new", true)
	case api.DeliverLast:
		d.Set("delivery_last", true)
	case api.DeliverLastPerSubject:
		d.Set("deliver_last_per_subject", true)
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

	var bo []any
	for _, d := range cons.Backoff() {
		bo = append(bo, d.String())
	}
	d.Set("backoff", bo)

	return nil
}

func resourceConsumerDelete(d *schema.ResourceData, m any) error {
	streamName, durableName, err := parseConsumerID(d.Id())
	if err != nil {
		return err
	}

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	known, err := mgr.IsKnownConsumer(streamName, durableName)
	if err != nil {
		return err
	}
	if !known {
		d.SetId("")
		return nil
	}

	cons, err := mgr.LoadConsumer(streamName, durableName)
	if err != nil {
		return err
	}

	return cons.Delete()
}
