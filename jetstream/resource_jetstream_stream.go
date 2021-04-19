package jetstream

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

func resourceStream() *schema.Resource {
	sourceInfo := map[string]*schema.Schema{
		"name": {
			Type:        schema.TypeString,
			Description: "The name of the source Stream",
			Required:    true,
		},
		"start_seq": {
			Type:        schema.TypeInt,
			Description: "The sequence to start mirroring from",
			Optional:    true,
		},
		"start_time": {
			Type:         schema.TypeString,
			ValidateFunc: validation.IsRFC3339Time,
			Description:  "The time stamp in the source stream to start from, in RFC3339 format",
			Optional:     true,
		},
		"filter_subject": {
			Type:        schema.TypeString,
			Description: "Only copy messages matching a specific subject, not usable for mirrors",
			Optional:    true,
		},
		"external": {
			Type:        schema.TypeList,
			MaxItems:    1,
			Description: "Streams replicated from other accounts",
			Optional:    true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"api": {
						Type:        schema.TypeString,
						Description: "The subject prefix for the remote API",
						Required:    false,
						Optional:    true,
					},
					"deliver": {
						Type:        schema.TypeString,
						Description: "The subject prefix where messages will be delivered to",
						Required:    false,
						Optional:    true,
					},
				},
			},
		},
	}

	return &schema.Resource{
		Create: resourceStreamCreate,
		Read:   resourceStreamRead,
		Update: resourceStreamUpdate,
		Delete: resourceStreamDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Description: "The name of the stream",
				Required:    true,
				ForceNew:    true,
			},
			"subjects": {
				Type:        schema.TypeList,
				MinItems:    1,
				Description: "The list of subjects that will be consumed by the Stream, may be empty when sources and mirrors are present",
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"max_msgs": {
				Type:        schema.TypeInt,
				Description: "The maximum amount of messages that can be kept in the stream",
				Optional:    true,
				Default:     -1,
			},
			"max_bytes": {
				Type:        schema.TypeInt,
				Description: "The maximum size of all messages that can be kept in the stream",
				Optional:    true,
				Default:     -1,
			},
			"max_age": {
				Type:        schema.TypeInt,
				Description: "The maximum oldest message that can be kept in the stream, duration specified in seconds",
				Optional:    true,
				Default:     0,
			},
			"duplicate_window": {
				Type:        schema.TypeInt,
				Description: "The size of the duplicate tracking windows, duration specified in seconds",
				Optional:    true,
				Default:     120,
			},
			"max_msg_size": {
				Type:        schema.TypeInt,
				Description: "The maximum individual message size that the stream will accept",
				Optional:    true,
				Default:     -1,
			},
			"storage": {
				Type:         schema.TypeString,
				Description:  "The storage engine to use to back the stream",
				Default:      "file",
				ForceNew:     true,
				Optional:     true,
				ValidateFunc: validateStorageTypeString(),
			},
			"ack": {
				Type:        schema.TypeBool,
				Description: "If the Stream should support confirming receiving messages via acknowledgements",
				Optional:    true,
				Default:     true,
			},
			"retention": {
				Type:         schema.TypeString,
				Description:  "The retention policy to apply over and above max_msgs, max_bytes and max_age",
				Default:      "limits",
				Optional:     true,
				ValidateFunc: validateRetentionTypeString(),
			},
			"max_consumers": {
				Type:        schema.TypeInt,
				Description: "Number of consumers this stream allows",
				Default:     -1,
				Optional:    true,
			},
			"replicas": {
				Type:        schema.TypeInt,
				Description: "How many replicas of the data to keep in a clustered environment",
				Default:     1,
				Optional:    true,
			},
			"placement_cluster": {
				Type:        schema.TypeString,
				Description: "Place the stream in a specific cluster, influenced by placement_tags",
				Default:     "",
				Optional:    true,
			},
			"placement_tags": {
				Type:        schema.TypeList,
				Description: "Place the stream only on servers with these tags",
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"mirror": {
				Type:        schema.TypeList,
				Description: "Specifies a remote stream to mirror into this one",
				MaxItems:    1,
				ForceNew:    false,
				Required:    false,
				Optional:    true,
				Elem:        &schema.Resource{Schema: sourceInfo},
			},
			"source": {
				Type:        schema.TypeList,
				Description: "Specifies a list of streams to source into this one",
				ForceNew:    false,
				Required:    false,
				Optional:    true,
				Elem:        &schema.Resource{Schema: sourceInfo},
			},
		},
	}
}

func resourceStreamCreate(d *schema.ResourceData, m interface{}) error {
	cfg, err := streamConfigFromResourceData(d)
	if err != nil {
		return err
	}

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	_, err = mgr.NewStreamFromDefault(cfg.Name, cfg)
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

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	known, err := mgr.IsKnownStream(name)
	if err != nil {
		return fmt.Errorf("could not determine if stream %q is known: %s", name, err)
	}
	if !known {
		d.SetId("")
		return nil
	}

	str, err := mgr.LoadStream(name)
	if err != nil {
		return fmt.Errorf("could not load stream %q: %s", name, err)
	}

	maxAge := str.MaxAge().Seconds()
	if maxAge == 0 {
		maxAge = -1
	}

	d.Set("name", str.Name())
	d.Set("subjects", str.Subjects())
	d.Set("max_consumers", str.MaxConsumers())
	d.Set("max_msgs", int(str.MaxMsgs()))
	d.Set("max_age", str.MaxAge().Seconds())
	d.Set("duplicate_window", str.DuplicateWindow().Seconds())
	d.Set("max_bytes", str.MaxBytes())
	d.Set("max_msg_size", int(str.MaxMsgSize()))
	d.Set("replicas", str.Replicas())
	d.Set("ack", !str.NoAck())

	if str.MaxAge() == -1 || str.MaxAge() == 0 {
		d.Set("max_age", "-1")
	}

	switch str.Storage() {
	case api.FileStorage:
		d.Set("storage", "file")
	case api.MemoryStorage:
		d.Set("storage", "memory")
	}

	switch str.Retention() {
	case api.LimitsPolicy:
		d.Set("retention", "limits")
	case api.InterestPolicy:
		d.Set("retention", "interest")
	case api.WorkQueuePolicy:
		d.Set("retention", "workqueue")
	}

	if str.IsMirror() {
		mirror := str.Mirror()
		d.Set("mirror.0.name", mirror.Name)
		d.Set("mirror.0.filter_subject", mirror.FilterSubject)
		d.Set("mirror.0.start_seq", mirror.OptStartSeq)
		if mirror.OptStartTime != nil {
			d.Set("mirror.0.start_time", mirror.OptStartTime.Format(time.RFC3339))
		}
		if mirror.External != nil {
			d.Set("mirror.0.external.api", mirror.External.ApiPrefix)
			d.Set("mirror.0.external.deliver", mirror.External.DeliverPrefix)
		}
	}

	if str.IsSourced() {
		for i, source := range str.Sources() {
			d.Set(fmt.Sprintf("source.%d.name", i), source.Name)
			d.Set(fmt.Sprintf("source.%d.filter_subject", i), source.FilterSubject)
			d.Set(fmt.Sprintf("source.%d.start_seq", i), source.OptStartSeq)
			if source.OptStartTime != nil {
				d.Set(fmt.Sprintf("source.%d.start_time", i), source.OptStartTime.Format(time.RFC3339))
			}
			if source.External != nil {
				d.Set(fmt.Sprintf("mirror.%d.external.api", i), source.External.ApiPrefix)
				d.Set(fmt.Sprintf("mirror.%d.external.deliver", i), source.External.DeliverPrefix)
			}
		}
	}
	return nil
}

func resourceStreamUpdate(d *schema.ResourceData, m interface{}) error {
	name := d.Get("name").(string)

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	known, err := mgr.IsKnownStream(name)
	if err != nil {
		return err
	}
	if !known {
		d.SetId("")
		return nil
	}

	str, err := mgr.LoadStream(name)
	if err != nil {
		return err
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

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	known, err := mgr.IsKnownStream(name)
	if err != nil {
		return err
	}
	if !known {
		d.SetId("")
		return nil
	}

	str, err := mgr.LoadStream(name)
	if err != nil {
		return err
	}

	return str.Delete()
}
