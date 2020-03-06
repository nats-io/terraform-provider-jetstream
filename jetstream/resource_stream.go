package jetstream

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats-server/v2/server"
)

func resourceStream() *schema.Resource {
	return &schema.Resource{
		Create: resourceStreamCreate,
		Read:   resourceStreamRead,
		Update: resourceStreamUpdate,
		Delete: resourceStreamDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:        schema.TypeString,
				Description: "The name of the stream",
				Required:    true,
				ForceNew:    true,
			},
			"subjects": &schema.Schema{
				Type:        schema.TypeList,
				MinItems:    1,
				Description: "The list of subjects that will be consumed by the Stream",
				Required:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"max_msgs": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "The maximum amount of messages that can be kept in the stream",
				Optional:    true,
				Default:     -1,
			},
			"max_bytes": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "The maximum size of all messages that can be kept in the stream",
				Optional:    true,
				Default:     -1,
			},
			"max_age": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "The maximum oldest message that can be kept in the stream, duration specified in seconds",
				Optional:    true,
				Default:     -1,
			},
			"max_msg_size": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "The maximum individual message size that the stream will accept",
				Optional:    true,
				Default:     -1,
			},
			"storage": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The storage engine to use to back the stream",
				Default:      "file",
				ForceNew:     true,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"file", "memory"}, false),
			},
			"ack": &schema.Schema{
				Type:        schema.TypeBool,
				Description: "If the Stream should support confirming receiving messages via acknowledgements",
				Optional:    true,
				Default:     true,
			},
			"retention": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The retention policy to apply over and above max_msgs, max_bytes and max_age",
				Default:      "limits",
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"limits", "interest", "workqueue"}, false),
			},
			"max_consumers": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "Number of consumers this stream allows",
				Default:     -1,
				Optional:    true,
			},
			"replicas": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "How many replicas of the data to keep in a clustered environment",
				Default:     1,
				Optional:    true,
			},
		},
	}
}

func streamConfigFromResourceData(d *schema.ResourceData) (cfg server.StreamConfig, err error) {
	var retention server.RetentionPolicy
	var storage server.StorageType

	switch d.Get("retention").(string) {
	case "limits":
		retention = server.LimitsPolicy
	case "interest":
		retention = server.InterestPolicy
	case "workqueue":
		retention = server.WorkQueuePolicy
	}

	switch d.Get("storage").(string) {
	case "file":
		storage = server.FileStorage
	case "memory":
		storage = server.MemoryStorage
	}

	subs := d.Get("subjects").([]interface{})
	var subjects = make([]string, len(subs))
	for i, sub := range subs {
		subjects[i] = sub.(string)
	}

	return server.StreamConfig{
		Name:         d.Get("name").(string),
		Subjects:     subjects,
		Retention:    retention,
		MaxConsumers: d.Get("max_consumers").(int),
		MaxMsgs:      int64(d.Get("max_msgs").(int)),
		MaxBytes:     int64(d.Get("max_bytes").(int)),
		MaxAge:       time.Second * time.Duration(d.Get("max_age").(int)),
		MaxMsgSize:   int32(d.Get("max_msg_size").(int)),
		Storage:      storage,
		Replicas:     d.Get("replicas").(int),
		NoAck:        !d.Get("ack").(bool),
	}, nil
}

func resourceStreamCreate(d *schema.ResourceData, m interface{}) error {
	cfg, err := streamConfigFromResourceData(d)
	if err != nil {
		return err
	}

	_, err = jsm.NewStreamFromDefault(cfg.Name, cfg)
	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("JETSTREAM_STREAM_%s", cfg.Name))

	return resourceStreamRead(d, m)
}

func resourceStreamRead(d *schema.ResourceData, m interface{}) error {
	name, err := parseStreamID(d.Id())
	if err != nil {
		return err
	}

	known, err := jsm.IsKnownStream(name)
	if err != nil {
		return fmt.Errorf("could not determine if stream %q is known: %s", name, err)
	}
	if !known {
		d.SetId("")
		return nil
	}

	str, err := jsm.LoadStream(name)
	if err != nil {
		return fmt.Errorf("could not load stream %q: %s", name, err)
	}

	d.Set("name", str.Name())
	d.Set("subjects", str.Subjects())
	d.Set("max_consumers", str.MaxConsumers())
	d.Set("max_msgs", int(str.MaxMsgs()))
	d.Set("max_age", str.MaxAge().Seconds())
	d.Set("max_msg_size", int(str.MaxMsgSize()))
	d.Set("replicas", str.Replicas())
	d.Set("ack", !str.NoAck())

	if str.MaxAge() == -1 {
		d.Set("max_age", "-1")
	}

	switch str.Storage() {
	case server.FileStorage:
		d.Set("storage", "file")
	case server.MemoryStorage:
		d.Set("storage", "memory")
	}

	switch str.Retention() {
	case server.LimitsPolicy:
		d.Set("retention", "limits")
	case server.InterestPolicy:
		d.Set("retention", "interest")
	case server.WorkQueuePolicy:
		d.Set("retention", "workqueue")
	}

	return nil
}

func resourceStreamUpdate(d *schema.ResourceData, m interface{}) error {
	name := d.Get("name").(string)
	str, err := jsm.LoadStream(name)
	if err != nil {
		d.SetId("")
		return nil
	}

	cfg, err := streamConfigFromResourceData(d)
	if err != nil {
		return err
	}

	err = str.UpdateConfiguration(cfg)
	if err != nil {
		return err
	}

	return resourceStreamRead(d, m)
}

func resourceStreamDelete(d *schema.ResourceData, m interface{}) error {
	name := d.Get("name").(string)

	str, err := jsm.LoadStream(name)
	if err != nil {
		d.SetId("")
		return nil
	}

	return str.Delete()
}
