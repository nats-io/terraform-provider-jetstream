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
	subjectTransform := map[string]*schema.Schema{
		"source": {
			Type:        schema.TypeString,
			Description: "The subject transform source",
			Required:    true,
		},
		"destination": {
			Type:        schema.TypeString,
			Description: "The subject transform destination",
			Required:    true,
		},
	}

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
		"subject_transform": {
			Type:        schema.TypeList,
			Description: "The subject filtering sources and associated destination transforms",
			Optional:    true,
			ForceNew:    false,
			Required:    false,
			Elem:        &schema.Resource{Schema: subjectTransform},
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
			"description": {
				Type:        schema.TypeString,
				Description: "Contains additional information about this stream",
				Optional:    true,
				ForceNew:    false,
			},
			"metadata": {
				Type:        schema.TypeMap,
				Description: "Free form metadata about the stream",
				Optional:    true,
				ForceNew:    false,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
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
			"discard": {
				Type:         schema.TypeString,
				Description:  "When a Stream reach it's limits either old messages are deleted or new ones are denied",
				Optional:     true,
				Default:      "old",
				ValidateFunc: validateDiscardPolicy(),
			},
			"discard_new_per_subject": {
				Type:        schema.TypeBool,
				Description: "When discard policy is new and the stream is one with max messages per subject set, this will apply the new behavior to every subject. Essentially turning discard new from maximum number of subjects into maximum number of messages in a subject",
				Optional:    true,
				Default:     false,
			},
			"max_msgs": {
				Type:        schema.TypeInt,
				Description: "The maximum amount of messages that can be kept in the stream",
				Optional:    true,
				Default:     -1,
			},
			"max_msgs_per_subject": {
				Type:        schema.TypeInt,
				Description: "The maximum amount of messages that can be kept in the stream on a per-subject basis",
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
			"compression": {
				Type:         schema.TypeString,
				Description:  "Optional compression algorithm used for the Stream",
				Default:      "none",
				Optional:     true,
				ValidateFunc: validateCompressionTypeString(),
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
			"deny_delete": {
				Type:        schema.TypeBool,
				Description: "Restricts the ability to delete messages from a stream via the API. Cannot be changed once set to true",
				Default:     false,
				Optional:    true,
			},
			"deny_purge": {
				Type:        schema.TypeBool,
				Description: "Restricts the ability to purge messages from a stream via the API. Cannot be change once set to true",
				Default:     false,
				Optional:    true,
			},
			"allow_rollup_hdrs": {
				Type:        schema.TypeBool,
				Description: "Allows the use of the Nats-Rollup header to replace all contents of a stream, or subject in a stream, with a single new message",
				Default:     false,
				Optional:    true,
			},
			"allow_direct": {
				Type:        schema.TypeBool,
				Description: "Allow higher performance, direct access to get individual messages via the $JS.DS.GET API",
				Default:     true,
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
			"subject_transform": {
				Type:        schema.TypeList,
				Description: "Subject transform to apply to matching messages",
				MaxItems:    1,
				ForceNew:    false,
				Required:    false,
				Optional:    true,
				Elem:        &schema.Resource{Schema: subjectTransform},
			},
			"mirror": {
				Type:        schema.TypeList,
				Description: "Specifies a remote stream to mirror into this one",
				MaxItems:    1,
				ForceNew:    true,
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
			"republish_source": {
				Type:        schema.TypeString,
				Description: "Republish messages to republish_destination",
				ForceNew:    false,
				Optional:    true,
			},
			"republish_destination": {
				Type:         schema.TypeString,
				Description:  "The destination to publish messages to",
				ForceNew:     false,
				Optional:     true,
				RequiredWith: []string{"republish_source"},
			},
			"republish_headers_only": {
				Type:        schema.TypeBool,
				Description: "Republish only message headers, no bodies",
				ForceNew:    false,
				Optional:    true,
			},
		},
	}
}

func resourceStreamCreate(d *schema.ResourceData, m any) error {
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

func resourceStreamRead(d *schema.ResourceData, m any) error {
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
	d.Set("description", str.Description())
	d.Set("metadata", str.Metadata())
	d.Set("subjects", str.Subjects())
	d.Set("max_consumers", str.MaxConsumers())
	d.Set("max_msgs", int(str.MaxMsgs()))
	d.Set("max_msgs_per_subject", int(str.MaxMsgsPerSubject()))
	d.Set("max_age", str.MaxAge().Seconds())
	d.Set("duplicate_window", str.DuplicateWindow().Seconds())
	d.Set("max_bytes", str.MaxBytes())
	d.Set("max_msg_size", int(str.MaxMsgSize()))
	d.Set("replicas", str.Replicas())
	d.Set("ack", !str.NoAck())
	d.Set("deny_delete", !str.DeleteAllowed())
	d.Set("deny_purge", !str.PurgeAllowed())
	d.Set("allow_rollup_hdrs", str.RollupAllowed())
	d.Set("allow_direct", str.DirectAllowed())
	d.Set("discard_new_per_subject", str.DiscardNewPerSubject())
	d.Set("compression", str.Compression())

	if str.MaxAge() == -1 || str.MaxAge() == 0 {
		d.Set("max_age", "-1")
	}

	switch str.DiscardPolicy() {
	case api.DiscardNew:
		d.Set("discard", "new")
	case api.DiscardOld:
		d.Set("discard", "old")
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

	if str.Configuration().Placement != nil {
		d.Set("placement_cluster", str.Configuration().Placement.Cluster)
		d.Set("placement_tags", str.Configuration().Placement.Tags)
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
		for c, v := range mirror.SubjectTransforms {
			d.Set(fmt.Sprintf("mirror.0.subject_transforms.%d.src", c), v.Source)
			d.Set(fmt.Sprintf("mirror.0.subject_transforms.%d.dest", c), v.Destination)
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
				d.Set(fmt.Sprintf("source.%d.external.api", i), source.External.ApiPrefix)
				d.Set(fmt.Sprintf("source.%d.external.deliver", i), source.External.DeliverPrefix)
			}
			for c, v := range source.SubjectTransforms {
				d.Set(fmt.Sprintf("source.%d.subject_transforms.%d.src", i, c), v.Source)
				d.Set(fmt.Sprintf("source.%d.subject_transforms.%d.dest", i, c), v.Destination)
			}
		}
	}

	if str.IsRepublishing() {
		d.Set("republish_source", str.Republish().Source)
		d.Set("republish_destination", str.Republish().Destination)
		d.Set("republish_headers_only", str.Republish().HeadersOnly)
	}

	return nil
}

func resourceStreamUpdate(d *schema.ResourceData, m any) error {
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

func resourceStreamDelete(d *schema.ResourceData, m any) error {
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
